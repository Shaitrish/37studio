package social

import (
	"cmp"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Shaitrish/37studio/internal/agent"
	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/costs"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/keys"
	"github.com/Shaitrish/37studio/internal/remote"
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/textutil"
)

var graphURL = config.Env("STUDIO37_GRAPH_URL", "https://graph.facebook.com") // or a stand-in, for demos and tests

const graphVer = "v26.0"

const tiktokAPI = "https://open.tiktokapis.com"

const metaScopes = "pages_show_list,pages_read_engagement,pages_manage_posts,business_management,instagram_basic,instagram_content_publish," +
	"pages_messaging,pages_manage_metadata,pages_read_user_content,pages_manage_engagement,instagram_manage_messages,instagram_manage_comments" // the last six: Inbox

// Copies share Shaitrish's 37Studio Meta app by default. Its secret never ships: they finish the login through the auth
// Worker on studio.shaitrish.com (worker/), which holds it. Anyone can instead enter their own app's ID and secret
// (Settings → Keys, or Social → Use your own Meta app). It is then the admin's own app in Development mode, so their
// Pages work without waiting for Meta to review ours, and logins never touch the Worker.
const sharedMetaApp, sharedMetaConfig = "2057436148296558", "1117077147715456"

func ownMetaApp() bool { return os.Getenv("META_APP_ID") != "" && os.Getenv("META_APP_SECRET") != "" }

// metaApp is the app ID and Login for Business configuration logins use; an own app without a configuration asks for scopes.
func metaApp() (id, config string) {
	if ownMetaApp() {
		return os.Getenv("META_APP_ID"), os.Getenv("META_CONFIG_ID")
	}
	return sharedMetaApp, sharedMetaConfig
}

func ProviderReady(p string) bool {
	if p == "meta" {
		return ownMetaApp() || config.AuthURL != ""
	}
	keys := map[string][2]string{"google": {"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET"}, "tiktok": {"TIKTOK_CLIENT_KEY", "TIKTOK_CLIENT_SECRET"},
		"linkedin": {"LINKEDIN_CLIENT_ID", "LINKEDIN_CLIENT_SECRET"}, "x": {"X_CLIENT_ID", "X_CLIENT_SECRET"}, "pinterest": {"PINTEREST_APP_ID", "PINTEREST_APP_SECRET"}}[p]
	return keys[0] != "" && os.Getenv(keys[0]) != "" && os.Getenv(keys[1]) != ""
}

// redirectURI is where a platform sends the browser back after sign-in. X refuses localhost callbacks, so it gets
// 127.0.0.1, the same server: the OAuth state lives here, not in a cookie, so the change of host doesn't matter.
func redirectURI(provider string) string {
	base := config.PublicURL
	if provider == "x" {
		base = strings.Replace(base, "://localhost", "://127.0.0.1", 1)
	}
	return base + "/api/oauth/" + provider + "/callback"
}

type oauthState struct {
	tenant             int64
	provider, ret, pkc string
	at                 time.Time
}

type socialAccount struct {
	Platform  string `json:"platform"`
	AccountID string `json:"account_id"`
	Name      string `json:"name"`
	Picture   string `json:"picture"`
	token     string // never sent to the browser
	refresh   string
}

type pendingConn struct {
	Tenant   int64           `json:"tenant"`
	Accounts []socialAccount `json:"accounts"`
	Error    string          `json:"error,omitempty"`
}

var oauthMu sync.Mutex

var states = map[string]*oauthState{}

var pending = map[string]*pendingConn{} // ponytail: in memory and never pruned; a restart or an abandoned picker just means connecting again

func randToken(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func OAuthStart(w http.ResponseWriter, r *http.Request) {
	prov := r.PathValue("provider")
	if !ProviderReady(prov) {
		http.Error(w, prov+" is not configured. See README → Social publishing.", http.StatusBadRequest)
		return
	}
	tenant, _ := strconv.ParseInt(r.URL.Query().Get("tenant"), 10, 64)
	ret := r.URL.Query().Get("return")
	if u, err := url.Parse(ret); err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		ret = "/"
	}
	key, st := randToken(24), &oauthState{tenant: tenant, provider: prov, ret: strings.Split(ret, "#")[0], pkc: randToken(48), at: time.Now()}
	oauthMu.Lock()
	states[key] = st
	oauthMu.Unlock()
	var u string
	switch prov {
	case "meta":
		app, config := metaApp()
		q := url.Values{"client_id": {app}, "redirect_uri": {redirectURI(prov)}, "state": {key}, "response_type": {"code"}}
		if c := config; c != "" { // Facebook Login for Business: the configuration holds the permissions
			q.Set("config_id", c)
		} else {
			q.Set("scope", metaScopes)
		}
		u = "https://www.facebook.com/" + graphVer + "/dialog/oauth?" + q.Encode()
	case "google":
		u = "https://accounts.google.com/o/oauth2/v2/auth?" + url.Values{"client_id": {os.Getenv("GOOGLE_CLIENT_ID")}, "redirect_uri": {redirectURI(prov)},
			"response_type": {"code"}, "state": {key}, "access_type": {"offline"}, "prompt": {"consent select_account"},
			"scope": {"https://www.googleapis.com/auth/youtube.upload https://www.googleapis.com/auth/youtube.readonly"}}.Encode()
	case "tiktok":
		// TikTok only allows http://localhost redirects for apps registered as Desktop, which must use PKCE (hex-encoded, per their docs).
		sum := sha256.Sum256([]byte(st.pkc))
		u = "https://www.tiktok.com/v2/auth/authorize/?" + url.Values{"client_key": {os.Getenv("TIKTOK_CLIENT_KEY")}, "redirect_uri": {redirectURI(prov)},
			"response_type": {"code"}, "state": {key}, "scope": {"user.info.basic,video.publish,video.upload"},
			"code_challenge": {hex.EncodeToString(sum[:])}, "code_challenge_method": {"S256"}}.Encode()
	case "linkedin":
		u = "https://www.linkedin.com/oauth/v2/authorization?" + url.Values{"client_id": {os.Getenv("LINKEDIN_CLIENT_ID")}, "redirect_uri": {redirectURI(prov)},
			"response_type": {"code"}, "state": {key}, "scope": {linkedinScopes()}}.Encode()
	case "x":
		sum := sha256.Sum256([]byte(st.pkc)) // standard PKCE: base64url, unlike TikTok's hex
		u = "https://x.com/i/oauth2/authorize?" + url.Values{"client_id": {os.Getenv("X_CLIENT_ID")}, "redirect_uri": {redirectURI(prov)},
			"response_type": {"code"}, "state": {key}, "scope": {"tweet.read tweet.write users.read media.write offline.access"},
			"code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"S256"}}.Encode()
	case "pinterest":
		u = "https://www.pinterest.com/oauth/?" + url.Values{"client_id": {os.Getenv("PINTEREST_APP_ID")}, "redirect_uri": {redirectURI(prov)},
			"response_type": {"code"}, "state": {key}, "scope": {"boards:read,pins:read,pins:write,user_accounts:read"}}.Encode()
	}
	http.Redirect(w, r, u, http.StatusFound)
}

func OAuthCallback(w http.ResponseWriter, r *http.Request) {
	prov, q := r.PathValue("provider"), r.URL.Query()
	oauthMu.Lock()
	st := states[q.Get("state")]
	delete(states, q.Get("state"))
	oauthMu.Unlock()
	if st == nil || st.provider != prov || time.Since(st.at) > 30*time.Minute {
		http.Error(w, "This sign-in link expired. Go back to 37Studio and connect again.", http.StatusBadRequest)
		return
	}
	pc := &pendingConn{Tenant: st.tenant}
	if e := q.Get("error"); e != "" {
		pc.Error = "Not connected: " + cmp.Or(q.Get("error_description"), e)
	} else {
		var err error
		switch prov {
		case "meta":
			pc.Accounts, err = metaAccounts(q.Get("code"))
		case "google":
			pc.Accounts, err = youtubeAccounts(q.Get("code"))
		case "tiktok":
			pc.Accounts, err = tiktokAccounts(q.Get("code"), st.pkc)
		case "linkedin":
			pc.Accounts, err = linkedinAccounts(q.Get("code"))
		case "x":
			pc.Accounts, err = xAccounts(q.Get("code"), st.pkc)
		case "pinterest":
			pc.Accounts, err = pinterestAccounts(q.Get("code"))
		}
		if err != nil {
			pc.Error = err.Error()
		} else if len(pc.Accounts) == 0 {
			pc.Error = map[string]string{
				"meta":      "No Facebook Pages came back. You need a role on the Page (or the client must add your business as a partner), and must tick it on Facebook's screen.",
				"google":    "That Google account has no YouTube channel.",
				"pinterest": "That Pinterest account has no boards yet. Create one on Pinterest, then connect again.",
			}[prov]
		}
	}
	key := randToken(24)
	oauthMu.Lock()
	pending[key] = pc
	oauthMu.Unlock()
	http.Redirect(w, r, fmt.Sprintf("%s#/clients/%d/social/%s", st.ret, st.tenant, key), http.StatusFound)
}

// metaUserToken trades the login code for a long-lived user token: directly when this copy has the app secret,
// else through the auth Worker. Page tokens fetched with a long-lived user token never expire, so nobody reconnects.
func metaUserToken(code string) (string, error) {
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if !ownMetaApp() {
		err := remote.Call(remote.JSON("POST", config.AuthURL+"/oauth/meta/token", map[string]string{"code": code, "redirect_uri": redirectURI("meta")}, ""), &tok)
		return tok.AccessToken, err
	}
	id, _ := metaApp()
	app := url.Values{"client_id": {id}, "client_secret": {os.Getenv("META_APP_SECRET")}}
	q := url.Values{"redirect_uri": {redirectURI("meta")}, "code": {code}}
	for k, v := range app {
		q[k] = v
	}
	if err := remote.Call(remote.Get(graph("/oauth/access_token", q), ""), &tok); err != nil {
		return "", err
	}
	q = url.Values{"grant_type": {"fb_exchange_token"}, "fb_exchange_token": {tok.AccessToken}}
	for k, v := range app {
		q[k] = v
	}
	err := remote.Call(remote.Get(graph("/oauth/access_token", q), ""), &tok)
	return tok.AccessToken, err
}

func metaAccounts(code string) ([]socialAccount, error) {
	var tok struct{ AccessToken string }
	var err error
	if tok.AccessToken, err = metaUserToken(code); err != nil {
		return nil, err
	}
	var pages struct {
		Data []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			AccessToken string `json:"access_token"`
			Picture     struct {
				Data struct {
					URL string `json:"url"`
				} `json:"data"`
			} `json:"picture"`
			IG *struct {
				ID       string `json:"id"`
				Username string `json:"username"`
				Picture  string `json:"profile_picture_url"`
			} `json:"instagram_business_account"`
		} `json:"data"`
	}
	fields := "id,name,access_token,picture{url},instagram_business_account{id,username,profile_picture_url}"
	if err := remote.Call(remote.Get(graph("/me/accounts", url.Values{"fields": {fields}, "limit": {"100"}}), "Bearer "+tok.AccessToken), &pages); err != nil {
		return nil, err
	}
	var out []socialAccount
	for _, p := range pages.Data {
		out = append(out, socialAccount{Platform: "facebook", AccountID: p.ID, Name: p.Name, Picture: p.Picture.Data.URL, token: p.AccessToken})
		if p.IG == nil { // the user-token listing can omit a linked Instagram that the Page's own token sees
			var pg struct {
				IG *struct {
					ID       string `json:"id"`
					Username string `json:"username"`
					Picture  string `json:"profile_picture_url"`
				} `json:"instagram_business_account"`
			}
			remote.Call(remote.Get(graph("/"+p.ID, url.Values{"fields": {"instagram_business_account{id,username,profile_picture_url}"}}), "Bearer "+p.AccessToken), &pg)
			p.IG = pg.IG
		}
		if p.IG != nil {
			// ponytail: Instagram calls use the linked Page's token; it works for Business accounts and never expires.
			out = append(out, socialAccount{Platform: "instagram", AccountID: p.IG.ID, Name: "@" + p.IG.Username, Picture: p.IG.Picture, token: p.AccessToken})
		}
	}
	return out, nil
}

func youtubeAccounts(code string) ([]socialAccount, error) {
	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	err := remote.Call(remote.Form("POST", "https://oauth2.googleapis.com/token", url.Values{"code": {code}, "client_id": {os.Getenv("GOOGLE_CLIENT_ID")},
		"client_secret": {os.Getenv("GOOGLE_CLIENT_SECRET")}, "redirect_uri": {redirectURI("google")}, "grant_type": {"authorization_code"}}, ""), &tok)
	if err != nil {
		return nil, err
	}
	var ch struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title      string `json:"title"`
				Thumbnails struct {
					Default struct {
						URL string `json:"url"`
					} `json:"default"`
				} `json:"thumbnails"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if err := remote.Call(remote.Get("https://www.googleapis.com/youtube/v3/channels?part=snippet&mine=true", "Bearer "+tok.AccessToken), &ch); err != nil {
		return nil, err
	}
	var out []socialAccount
	for _, c := range ch.Items {
		out = append(out, socialAccount{Platform: "youtube", AccountID: c.ID, Name: c.Snippet.Title, Picture: c.Snippet.Thumbnails.Default.URL, refresh: tok.RefreshToken})
	}
	return out, nil
}

type tiktokToken struct {
	OpenID       string `json:"open_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func tiktokTokenCall(v url.Values) (tiktokToken, error) {
	v.Set("client_key", os.Getenv("TIKTOK_CLIENT_KEY"))
	v.Set("client_secret", os.Getenv("TIKTOK_CLIENT_SECRET"))
	var t tiktokToken
	err := remote.Call(remote.Form("POST", tiktokAPI+"/v2/oauth/token/", v, ""), &t)
	if err == nil && t.AccessToken == "" {
		err = errors.New("TikTok returned no access token")
	}
	return t, err
}

func tiktokAccounts(code, verifier string) ([]socialAccount, error) {
	t, err := tiktokTokenCall(url.Values{"code": {code}, "grant_type": {"authorization_code"}, "redirect_uri": {redirectURI("tiktok")}, "code_verifier": {verifier}})
	if err != nil {
		return nil, err
	}
	var u struct {
		Data struct {
			User struct {
				Name   string `json:"display_name"`
				Avatar string `json:"avatar_url"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := remote.Call(remote.Get(tiktokAPI+"/v2/user/info/?fields=open_id,display_name,avatar_url", "Bearer "+t.AccessToken), &u); err != nil {
		return nil, err
	}
	return []socialAccount{{Platform: "tiktok", AccountID: t.OpenID, Name: u.Data.User.Name, Picture: u.Data.User.Avatar, token: t.AccessToken, refresh: t.RefreshToken}}, nil
}

func ListAccounts(r *http.Request) (any, error) {
	accts, err := store.Rows("select id,tenant_id,platform,account_id,name,picture,created_at from social_accounts where tenant_id=? order by platform,name", httpapi.PathID(r))
	ready := map[string]any{}
	for _, p := range []string{"meta", "google", "tiktok", "linkedin", "x", "pinterest"} {
		ready[p] = ProviderReady(p)
	}
	var mode string
	store.DB.QueryRow("select inbox_mode from tenants where id=?", httpapi.PathID(r)).Scan(&mode)
	metaID, _ := metaApp()
	if metaID == sharedMetaApp { // Shaitrish's own install holds the shared app's secret too
		metaID = ""
	}
	return map[string]any{"accounts": accts, "ready": ready, "inbox_mode": mode, "replies": files.TenantDoc(httpapi.PathID(r), "replies.md"), "meta_app": metaID,
		"redirect": config.PublicURL + "/api/oauth/{provider}/callback"}, err
}

func GetPending(r *http.Request) (any, error) {
	oauthMu.Lock()
	defer oauthMu.Unlock()
	if pc := pending[r.PathValue("key")]; pc != nil {
		return pc, nil
	}
	return nil, httpapi.Error{Code: http.StatusNotFound, Msg: "this connection expired; connect again"}
}

func SavePending(r *http.Request) (any, error) {
	b, err := httpapi.Body(r)
	if err != nil {
		return nil, err
	}
	oauthMu.Lock()
	pc := pending[r.PathValue("key")]
	delete(pending, r.PathValue("key"))
	oauthMu.Unlock()
	if pc == nil {
		return nil, httpapi.Error{Code: http.StatusNotFound, Msg: "this connection expired; connect again"}
	}
	pick := map[string]bool{}
	list, _ := b["accounts"].([]any)
	for _, v := range list {
		pick[store.Str(v)] = true
	}
	n := 0
	for _, a := range pc.Accounts {
		if !pick[a.Platform+":"+a.AccountID] {
			continue
		}
		_, err := store.DB.Exec(`insert into social_accounts(tenant_id,platform,account_id,name,picture,token,refresh) values(?,?,?,?,?,?,?)
		  on conflict(tenant_id,platform,account_id) do update set name=excluded.name, picture=excluded.picture, token=excluded.token, refresh=excluded.refresh`,
			pc.Tenant, a.Platform, a.AccountID, a.Name, a.Picture, keys.SealText(a.token), keys.SealText(a.refresh))
		if err != nil {
			return nil, err
		}
		n++
	}
	return map[string]int{"connected": n}, nil
}

// SealTokens encrypts platform tokens saved before they were sealed at rest. main runs it once at startup.
func SealTokens() {
	rs, _ := store.Rows("select id, token, refresh from social_accounts where (token<>'' and token not like 'sealed:%') or (refresh<>'' and refresh not like 'sealed:%')")
	for _, r := range rs {
		store.DB.Exec("update social_accounts set token=?, refresh=? where id=?", keys.SealText(store.Str(r["token"])), keys.SealText(store.Str(r["refresh"])), r["id"])
	}
}

func Disconnect(r *http.Request) (any, error) {
	_, err := store.DB.Exec("delete from social_accounts where id=?", httpapi.PathID(r))
	return nil, err
}

func Captions(id int64) map[string]any {
	var m map[string]any
	if b, err := os.ReadFile(filepath.Join(files.ProjectDir(id), "work", "social.json")); err == nil {
		json.Unmarshal(b, &m)
	}
	return m
}

func saveSocialCaptions(id int64, m map[string]any) error {
	b, _ := json.MarshalIndent(m, "", "  ")
	os.MkdirAll(filepath.Join(files.ProjectDir(id), "work"), 0o755)
	return os.WriteFile(filepath.Join(files.ProjectDir(id), "work", "social.json"), b, 0o644)
}

var tagRe = regexp.MustCompile(`(?s)<(script|style)[^>]*>.*?</(script|style)>|<[^>]+>`)

var spaceRe = regexp.MustCompile(`\s+`)

// onScreenText is every visible string in the composition: captions, titles, CTAs. It tells Claude what the video says.
func onScreenText(id int64) string {
	var b strings.Builder
	filepath.WalkDir(filepath.Join(files.ProjectDir(id), "comp"), func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" || d.Name() == "assets") {
			return filepath.SkipDir
		}
		if err == nil && strings.HasSuffix(p, ".html") {
			raw, _ := os.ReadFile(p)
			b.WriteString(spaceRe.ReplaceAllString(tagRe.ReplaceAllString(string(raw), " "), " ") + "\n")
		}
		return nil
	})
	return textutil.Head(b.String(), 6000)
}

func WriteCaptions(r *http.Request) (any, error) { return MakeCaptions(httpapi.PathID(r)) }

// MakeCaptions asks Claude for every platform's caption from what the video shows and saves them to work/social.json.
func MakeCaptions(id int64) (map[string]any, error) {
	p, err := store.Row(store.ProjectSelect+" where p.id=?", id)
	if err != nil {
		return nil, err
	}
	tenant := p["tenant_id"].(int64)
	notes, _ := store.Rows("select body from events where project_id=? and kind='text' order by id desc limit 3", id)
	var said []string
	for _, n := range notes {
		said = append(said, store.Str(n["body"]))
	}
	prompt := fmt.Sprintf(`Write social media post captions for a finished video by %s.
Language: %s (write every caption in this language; hashtags may mix in English).
Video: %q, %v, about %v seconds. Brief: %s
Focus: %s

What is on screen:
%s

The editor agent's latest notes on the video:
%s

About the client (use only these facts; never invent claims, prices or links):
%s

Hook the reader in the first line, keep it true to the video, end with the client's call to action if the knowledge base has one.
Reply with only this JSON object, no code fence:
%s`, p["tenant_name"], languageName(agent.ProjectLanguage(p)), p["title"], p["aspect"], p["seconds"], cmp.Or(store.Str(p["brief"]), "-"), cmp.Or(store.Str(p["focus"]), "-"),
		cmp.Or(onScreenText(id), "-"), cmp.Or(textutil.Head(strings.Join(said, "\n---\n"), 3000), "-"), cmp.Or(textutil.Head(files.TenantDoc(tenant, "knowledge.md"), 5000), "-"), agent.CaptionSpec)
	text, cost, tokens, err := claudeOnce(files.ProjectDir(id), prompt)
	if err != nil {
		return nil, err
	}
	costs.Record("anthropic", id, tenant, agent.QualityByID("draft").Model, tokens, cost, "")
	var caps map[string]any
	s := text
	if i, j := strings.Index(s, "{"), strings.LastIndex(s, "}"); i >= 0 && j > i {
		s = s[i : j+1]
	}
	if err := json.Unmarshal([]byte(s), &caps); err != nil {
		return nil, fmt.Errorf("claude did not return captions: %s", textutil.Tail(text, 300))
	}
	return caps, saveSocialCaptions(id, caps)
}

// claudeOnce runs one tool-less Claude turn (Sonnet, low effort) in dir and returns the text, its cost and token count.
func claudeOnce(dir, prompt string) (text string, cost, tokens float64, err error) {
	if err := keys.ClaudeReady(); err != nil {
		return "", 0, 0, err
	}
	cmd := exec.Command(config.ClaudeBin, "-p", "--output-format", "json", "--model", agent.QualityByID("draft").Model, "--effort", "low", "--tools", "",
		"--strict-mcp-config", "--settings", `{"disableAllHooks":true}`)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(prompt)
	out, runErr := cmd.Output()
	var res struct {
		Result  string  `json:"result"`
		IsError bool    `json:"is_error"`
		Cost    float64 `json:"total_cost_usd"`
		Usage   struct {
			In     float64 `json:"input_tokens"`
			Out    float64 `json:"output_tokens"`
			CacheW float64 `json:"cache_creation_input_tokens"`
			CacheR float64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(out, &res) != nil || res.IsError {
		if h := keys.ClaudeHint(res.Result + string(out)); h != "" {
			return "", res.Cost, 0, errors.New(h)
		}
		return "", res.Cost, 0, fmt.Errorf("claude failed: %v %s", runErr, textutil.Tail(cmp.Or(res.Result, string(out)), 300))
	}
	return strings.TrimSpace(res.Result), res.Cost, res.Usage.In + res.Usage.Out + res.Usage.CacheW + res.Usage.CacheR, nil
}

func languageName(code string) string {
	if n := map[string]string{"bn": "Bangla", "en": "English", "hi": "Hindi", "ur": "Urdu", "ar": "Arabic", "es": "Spanish"}[code]; n != "" {
		return n
	}
	return code
}

func ListPosts(id int64) []map[string]any {
	ps, _ := store.Rows("select * from posts where project_id=? order by id desc", id)
	return ps
}

// CreatePosts queues one post per picked account: {file, at?, accounts: [id], captions: {platform: text, youtube_title}, privacy: {youtube, tiktok}}.
func CreatePosts(r *http.Request) (any, error) {
	var b struct {
		File     string            `json:"file"`
		At       string            `json:"at"`
		Accounts []int64           `json:"accounts"`
		Captions map[string]any    `json:"captions"`
		Privacy  map[string]string `json:"privacy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		return nil, httpapi.Bad("invalid JSON body")
	}
	id := httpapi.PathID(r)
	var tenant int64
	if err := store.DB.QueryRow("select tenant_id from projects where id=?", id).Scan(&tenant); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(b.File, "out/") || files.Kind(b.File) != "video" {
		return nil, httpapi.Bad("pick a final video from out/")
	}
	if _, err := os.Stat(files.Within(files.ProjectDir(id), b.File)); err != nil {
		return nil, httpapi.Bad("that video no longer exists")
	}
	if len(b.Accounts) == 0 {
		return nil, httpapi.Bad("pick at least one account")
	}
	at := time.Now().UTC()
	if b.At != "" {
		t, err := time.Parse(time.RFC3339, b.At)
		if err != nil {
			return nil, httpapi.Bad("invalid schedule time")
		}
		at = t.UTC()
	}
	caption := func(k string) string { return strings.TrimSpace(store.Str(b.Captions[k])) }
	for _, aid := range b.Accounts {
		var platform string
		if store.DB.QueryRow("select platform from social_accounts where id=? and tenant_id=? and platform<>'whatsapp'", aid, tenant).Scan(&platform) != nil {
			return nil, httpapi.Bad("an account is not connected to this client")
		}
		if platform == "youtube" && caption("youtube_title") == "" {
			return nil, httpapi.Bad("YouTube needs a title")
		}
		if platform == "tiktok" && b.Privacy["tiktok"] == "" {
			return nil, httpapi.Bad("choose who can see the TikTok post")
		}
	}
	tx, err := store.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for _, aid := range b.Accounts {
		a, _ := store.Row("select platform,name from social_accounts where id=?", aid)
		pl := store.Str(a["platform"])
		title := ""
		if pl == "youtube" || pl == "pinterest" {
			title = caption(pl + "_title")
		}
		if _, err := tx.Exec(`insert into posts(project_id,account_id,platform,account_name,file,title,caption,privacy,publish_at) values(?,?,?,?,?,?,?,?,?)`,
			id, aid, pl, a["name"], b.File, title, caption(pl), b.Privacy[pl], at.Format(time.DateTime)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	caps := Captions(id) // keep the final wording for the next post of this project
	if caps == nil {
		caps = map[string]any{}
	}
	for k, v := range b.Captions {
		caps[k] = v
	}
	saveSocialCaptions(id, caps)
	go RunDuePosts()
	return map[string]int{"queued": len(b.Accounts)}, nil
}

func RetryPost(r *http.Request) (any, error) {
	_, err := store.DB.Exec("update posts set status='scheduled', error='', publish_at=datetime('now') where id=? and status='failed'", httpapi.PathID(r))
	go RunDuePosts()
	return nil, err
}

// DeletePost cancels a scheduled post or removes a finished one from the list; it never deletes anything on the platform.
func DeletePost(r *http.Request) (any, error) {
	res, err := store.DB.Exec("delete from posts where id=? and status!='posting'", httpapi.PathID(r))
	if n, _ := res.RowsAffected(); err == nil && n == 0 {
		return nil, httpapi.Bad("it is uploading right now; wait for it to finish")
	}
	return nil, err
}

var postMu sync.Mutex

// PostLoop publishes due posts. ponytail: one upload at a time and a 30 s tick; uploads are uplink-bound anyway,
// and scheduled posts only go out while 37Studio is running.
func PostLoop() {
	store.DB.Exec(`update posts set status='failed', error='Interrupted by a server restart. Check the account before retrying: it may have gone out.' where status='posting'`)
	for {
		RunDuePosts()
		time.Sleep(30 * time.Second)
	}
}

func RunDuePosts() {
	if !postMu.TryLock() {
		return
	}
	defer postMu.Unlock()
	store.DB.Exec(`update posts set status='failed', error='Missed: 37Studio was not running at the scheduled time. Retry to post now.'
	  where status='scheduled' and publish_at < datetime('now','-12 hours')`)
	for {
		p, err := store.Row("select * from posts where status='scheduled' and publish_at <= datetime('now') order by publish_at, id limit 1")
		if err != nil {
			return
		}
		store.DB.Exec("update posts set status='posting', error='' where id=?", p["id"])
		remote, link, err := publish(p)
		if err != nil {
			store.DB.Exec("update posts set status='failed', error=? where id=?", err.Error(), p["id"])
			continue
		}
		store.DB.Exec("update posts set status='posted', remote_id=?, url=? where id=?", remote, link, p["id"])
		// An autopilot video counts as posted once every account has it.
		store.DB.Exec(`update projects set autopilot_stage='posted' where id=? and autopilot_stage='scheduled'
		  and not exists(select 1 from posts where project_id=? and status<>'posted')`, p["project_id"], p["project_id"])
	}
}

func publish(p map[string]any) (remote, link string, err error) {
	a, err := store.Row("select * from social_accounts where id=?", p["account_id"])
	if err != nil {
		return "", "", errors.New("the account was disconnected; connect it again on the client's Social tab")
	}
	a["token"], a["refresh"] = keys.OpenText(store.Str(a["token"])), keys.OpenText(store.Str(a["refresh"]))
	file := files.Within(files.ProjectDir(p["project_id"].(int64)), store.Str(p["file"]))
	if _, err := os.Stat(file); err != nil {
		return "", "", errors.New("the video file was deleted")
	}
	tok, err := accessToken(a)
	if err != nil {
		return "", "", fmt.Errorf("could not refresh the login (%v); connect the account again", err)
	}
	switch store.Str(a["platform"]) {
	case "facebook":
		return postFacebook(store.Str(a["account_id"]), tok, file, store.Str(p["caption"]))
	case "instagram":
		return postInstagram(store.Str(a["account_id"]), tok, file, store.Str(p["caption"]))
	case "youtube":
		return postYouTube(tok, file, store.Str(p["title"]), store.Str(p["caption"]), cmp.Or(store.Str(p["privacy"]), "public"))
	case "tiktok":
		return postTikTok(tok, file, store.Str(p["caption"]), store.Str(p["privacy"]))
	case "linkedin":
		return postLinkedIn(store.Str(a["account_id"]), tok, file, store.Str(p["caption"]))
	case "x":
		return postX(tok, file, store.Str(p["caption"]))
	case "pinterest":
		return postPinterest(store.Str(a["account_id"]), tok, file, store.Str(p["title"]), store.Str(p["caption"]))
	case "bluesky":
		return postBluesky(store.Str(a["account_id"]), tok, store.Str(a["refresh"]), file, store.Str(p["caption"]))
	case "mastodon":
		return postMastodon(store.Str(a["refresh"]), tok, file, store.Str(p["caption"]))
	}
	return "", "", fmt.Errorf("unknown platform %v", a["platform"])
}

// accessToken returns a usable token: Meta Page tokens never expire; the others trade the stored refresh token.
// Bluesky's "token" is its app password and Mastodon's never expires, so both are used as they are.
func accessToken(a map[string]any) (string, error) {
	old := store.Str(a["refresh"])
	rotate := func(t oauthTok, err error) (string, error) { // platforms may rotate the refresh token; Pinterest boards share one
		if err == nil {
			rs, _ := store.Rows("select id, refresh from social_accounts where platform=?", a["platform"])
			for _, r := range rs {
				if keys.OpenText(store.Str(r["refresh"])) == old {
					store.DB.Exec("update social_accounts set token=?, refresh=? where id=?", keys.SealText(t.AccessToken), keys.SealText(cmp.Or(t.RefreshToken, old)), r["id"])
				}
			}
		}
		return t.AccessToken, err
	}
	switch store.Str(a["platform"]) {
	case "youtube":
		var t struct {
			AccessToken string `json:"access_token"`
		}
		err := remote.Call(remote.Form("POST", "https://oauth2.googleapis.com/token", url.Values{"client_id": {os.Getenv("GOOGLE_CLIENT_ID")}, "client_secret": {os.Getenv("GOOGLE_CLIENT_SECRET")},
			"refresh_token": {store.Str(a["refresh"])}, "grant_type": {"refresh_token"}}, ""), &t)
		return t.AccessToken, err
	case "tiktok":
		t, err := tiktokTokenCall(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {store.Str(a["refresh"])}})
		if err == nil {
			store.DB.Exec("update social_accounts set token=?, refresh=? where id=?", keys.SealText(t.AccessToken), keys.SealText(cmp.Or(t.RefreshToken, store.Str(a["refresh"]))), a["id"]) // TikTok may rotate it
		}
		return t.AccessToken, err
	case "x": // tokens last 2 hours
		return rotate(basicTokenCall(xAPI+"/2/oauth2/token", os.Getenv("X_CLIENT_ID"), os.Getenv("X_CLIENT_SECRET"),
			url.Values{"grant_type": {"refresh_token"}, "refresh_token": {old}, "client_id": {os.Getenv("X_CLIENT_ID")}}))
	case "pinterest": // tokens last 30 days
		return rotate(basicTokenCall(pinterestAPI+"/v5/oauth/token", os.Getenv("PINTEREST_APP_ID"), os.Getenv("PINTEREST_APP_SECRET"),
			url.Values{"grant_type": {"refresh_token"}, "refresh_token": {old}}))
	case "linkedin": // 60-day tokens; LinkedIn hands out refresh tokens only to some apps, and the rest reconnect
		if old != "" {
			var t oauthTok
			err := remote.Call(remote.Form("POST", "https://www.linkedin.com/oauth/v2/accessToken", url.Values{"grant_type": {"refresh_token"}, "refresh_token": {old},
				"client_id": {os.Getenv("LINKEDIN_CLIENT_ID")}, "client_secret": {os.Getenv("LINKEDIN_CLIENT_SECRET")}}, ""), &t)
			return rotate(t, err)
		}
	}
	return store.Str(a["token"]), nil
}

func postFacebook(page, tok, file, caption string) (string, string, error) {
	w, h := files.VideoSize(file)
	if sec := files.MediaSeconds(file); h <= w || sec < 3 || sec > 90 {
		return facebookVideo(page, tok, file, caption) // Reels must be vertical and 3-90 s; anything else posts as a regular Page video
	}
	var start struct {
		VideoID   string `json:"video_id"`
		UploadURL string `json:"upload_url"`
	}
	if err := remote.Call(remote.Form("POST", graph("/"+page+"/video_reels", nil), url.Values{"upload_phase": {"start"}}, "Bearer "+tok), &start); err != nil {
		return "", "", err
	}
	if err := rupload(start.UploadURL, tok, file); err != nil {
		return "", "", err
	}
	err := remote.Call(remote.Form("POST", graph("/"+page+"/video_reels", nil), url.Values{"upload_phase": {"finish"}, "video_id": {start.VideoID},
		"video_state": {"PUBLISHED"}, "description": {caption}}, "Bearer "+tok), nil)
	return start.VideoID, "https://www.facebook.com/reel/" + start.VideoID, err
}

func facebookVideo(page, tok, file, caption string) (string, string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	req := remote.Multipart("https://graph-video.facebook.com/"+graphVer+"/"+page+"/videos", map[string]string{"description": caption}, "source", filepath.Base(file), f, "Bearer "+tok)
	var res struct {
		ID string `json:"id"`
	}
	err = remote.CallWith(remote.Uploads, req, &res)
	return res.ID, "https://www.facebook.com/" + page + "/videos/" + res.ID, err
}

func postInstagram(ig, tok, file, caption string) (string, string, error) {
	var c struct {
		ID  string `json:"id"`
		URI string `json:"uri"`
	}
	if err := remote.Call(remote.Form("POST", graph("/"+ig+"/media", nil), url.Values{"media_type": {"REELS"}, "upload_type": {"resumable"}, "caption": {caption}}, "Bearer "+tok), &c); err != nil {
		return "", "", err
	}
	if err := rupload(c.URI, tok, file); err != nil {
		return "", "", err
	}
	for i := 0; ; i++ { // Instagram transcodes before it lets you publish
		var s struct {
			Code   string `json:"status_code"`
			Status string `json:"status"`
		}
		if err := remote.Call(remote.Get(graph("/"+c.ID, url.Values{"fields": {"status_code,status"}}), "Bearer "+tok), &s); err != nil {
			return "", "", err
		}
		if s.Code == "FINISHED" {
			break
		}
		if s.Code == "ERROR" || s.Code == "EXPIRED" || i > 120 {
			return "", "", fmt.Errorf("instagram could not process the video: %s", cmp.Or(s.Status, s.Code))
		}
		time.Sleep(5 * time.Second)
	}
	var m struct {
		ID        string `json:"id"`
		Permalink string `json:"permalink"`
	}
	if err := remote.Call(remote.Form("POST", graph("/"+ig+"/media_publish", nil), url.Values{"creation_id": {c.ID}}, "Bearer "+tok), &m); err != nil {
		return "", "", err
	}
	remote.Call(remote.Get(graph("/"+m.ID, url.Values{"fields": {"permalink"}}), "Bearer "+tok), &m)
	return m.ID, m.Permalink, nil
}

func postYouTube(tok, file, title, desc, privacy string) (string, string, error) {
	st, err := os.Stat(file)
	if err != nil {
		return "", "", err
	}
	meta := map[string]any{
		"snippet": map[string]any{"title": textutil.Cut(strings.NewReplacer("<", "", ">", "").Replace(title), 100), "description": textutil.Cut(desc, 4900), "categoryId": "22"},
		"status":  map[string]any{"privacyStatus": privacy, "selfDeclaredMadeForKids": false},
	}
	req := remote.JSON("POST", "https://www.googleapis.com/upload/youtube/v3/videos?uploadType=resumable&part=snippet,status", meta, "Bearer "+tok)
	req.Header.Set("X-Upload-Content-Type", "video/mp4")
	req.Header.Set("X-Upload-Content-Length", fmt.Sprint(st.Size()))
	res, err := remote.Client.Do(req)
	if err != nil {
		return "", "", err
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	loc := res.Header.Get("Location")
	if res.StatusCode >= 300 || loc == "" {
		return "", "", fmt.Errorf("youtube %d: %s", res.StatusCode, remote.Message(raw))
	}
	f, _ := os.Open(file)
	defer f.Close()
	up, _ := http.NewRequest("PUT", loc, f)
	up.ContentLength = st.Size()
	up.Header.Set("Content-Type", "video/mp4")
	var v struct {
		ID string `json:"id"`
	}
	err = remote.CallWith(remote.Uploads, up, &v)
	return v.ID, "https://youtu.be/" + v.ID, err
}

// postTikTok direct-posts (or, with privacy INBOX, sends to the creator's TikTok drafts). Until TikTok audits the app,
// only SELF_ONLY (private) direct posts are allowed.
func postTikTok(tok, file, caption, privacy string) (string, string, error) {
	st, err := os.Stat(file)
	if err != nil {
		return "", "", err
	}
	size := st.Size()
	chunk, count := size, int64(1)
	if size > 64_000_000 { // TikTok: whole file up to 64 MB, else 5-64 MB chunks with the remainder folded into the last
		chunk = 10_000_000
		count = size / chunk
	}
	src := map[string]any{"source": "FILE_UPLOAD", "video_size": size, "chunk_size": chunk, "total_chunk_count": count}
	endpoint, body := "/v2/post/publish/inbox/video/init/", map[string]any{"source_info": src}
	user := ""
	if privacy != "INBOX" {
		var ci struct {
			Data struct {
				Options  []string `json:"privacy_level_options"`
				Username string   `json:"creator_username"`
			} `json:"data"`
		}
		if err := remote.Call(remote.JSON("POST", tiktokAPI+"/v2/post/publish/creator_info/query/", map[string]any{}, "Bearer "+tok), &ci); err != nil {
			return "", "", err
		}
		if !slices.Contains(ci.Data.Options, privacy) {
			return "", "", fmt.Errorf("TikTok allows only %s for this account right now (unaudited apps can post private only; or send it to drafts)", strings.Join(ci.Data.Options, ", "))
		}
		user = ci.Data.Username
		endpoint, body = "/v2/post/publish/video/init/", map[string]any{"source_info": src, "post_info": map[string]any{"title": caption, "privacy_level": privacy}}
	}
	var init struct {
		Data struct {
			PublishID string `json:"publish_id"`
			UploadURL string `json:"upload_url"`
		} `json:"data"`
	}
	if err := remote.Call(remote.JSON("POST", tiktokAPI+endpoint, body, "Bearer "+tok), &init); err != nil {
		return "", "", err
	}
	f, _ := os.Open(file)
	defer f.Close()
	ctype := map[string]string{".mov": "video/quicktime", ".webm": "video/webm"}[strings.ToLower(filepath.Ext(file))]
	for i := int64(0); i < count; i++ {
		start, end := i*chunk, (i+1)*chunk-1
		if i == count-1 {
			end = size - 1
		}
		req, _ := http.NewRequest("PUT", init.Data.UploadURL, io.NewSectionReader(f, start, end-start+1))
		req.ContentLength = end - start + 1
		req.Header.Set("Content-Type", cmp.Or(ctype, "video/mp4"))
		req.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
		if err := remote.CallWith(remote.Uploads, req, nil); err != nil {
			return "", "", err
		}
	}
	for i := 0; i < 120; i++ {
		var s struct {
			Data struct {
				Status string  `json:"status"`
				Reason string  `json:"fail_reason"`
				IDs    []int64 `json:"publicaly_available_post_id"` // sic
			} `json:"data"`
		}
		if err := remote.Call(remote.JSON("POST", tiktokAPI+"/v2/post/publish/status/fetch/", map[string]any{"publish_id": init.Data.PublishID}, "Bearer "+tok), &s); err != nil {
			return "", "", err
		}
		switch s.Data.Status {
		case "FAILED":
			return "", "", errors.New("tiktok: " + cmp.Or(s.Data.Reason, "processing failed"))
		case "SEND_TO_USER_INBOX":
			return init.Data.PublishID, "", nil
		case "PUBLISH_COMPLETE":
			if len(s.Data.IDs) > 0 && user != "" {
				return fmt.Sprint(s.Data.IDs[0]), fmt.Sprintf("https://www.tiktok.com/@%s/video/%d", user, s.Data.IDs[0]), nil
			}
			return init.Data.PublishID, "", nil
		}
		time.Sleep(5 * time.Second)
	}
	return init.Data.PublishID, "", errors.New("tiktok is still processing; check the account in a few minutes")
}

func graph(path string, q url.Values) string {
	u := graphURL + "/" + graphVer + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return u
}

// rupload sends a whole file to Meta's resumable upload host (Facebook Reels and Instagram).
func rupload(u, tok, file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	st, _ := f.Stat()
	req, _ := http.NewRequest("POST", u, f)
	req.ContentLength = st.Size()
	req.Header.Set("Authorization", "OAuth "+tok)
	req.Header.Set("offset", "0")
	req.Header.Set("file_size", fmt.Sprint(st.Size()))
	req.Header.Set("Content-Type", "application/octet-stream")
	return remote.CallWith(remote.Uploads, req, nil)
}
