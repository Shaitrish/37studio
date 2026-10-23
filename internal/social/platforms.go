package social

import (
	"cmp"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/remote"
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/textutil"
)

var linkedinAPI = "https://api.linkedin.com"

var xAPI = "https://api.x.com"

var pinterestAPI = strings.TrimRight(config.Env("PINTEREST_API", "https://api.pinterest.com"), "/") // Trial-access apps: https://api-sandbox.pinterest.com

// linkedinVersion pins LinkedIn's versioned REST API; each monthly version is supported for a year.
const linkedinVersion = "202605"

func linkedinPages() bool { v := config.Env("LINKEDIN_PAGES", ""); return v == "1" || v == "true" }

func linkedinScopes() string {
	if linkedinPages() { // the Community Management API, which LinkedIn grants per app after review
		return "openid profile w_member_social r_organization_admin w_organization_social"
	}
	return "openid profile w_member_social"
}

func linkedin(req *http.Request) *http.Request {
	req.Header.Set("LinkedIn-Version", linkedinVersion)
	req.Header.Set("X-Restli-Protocol-Version", "2.0.0")
	return req
}

type oauthTok struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// basicTokenCall trades a code or refresh token at a token endpoint that takes the app's ID and secret as Basic auth (X, Pinterest).
func basicTokenCall(u, id, secret string, v url.Values) (oauthTok, error) {
	var t oauthTok
	err := remote.Call(remote.Form("POST", u, v, "Basic "+base64.StdEncoding.EncodeToString([]byte(id+":"+secret))), &t)
	if err == nil && t.AccessToken == "" {
		err = errors.New("no access token came back")
	}
	return t, err
}

func linkedinAccounts(code string) ([]socialAccount, error) {
	var tok oauthTok
	err := remote.Call(remote.Form("POST", "https://www.linkedin.com/oauth/v2/accessToken", url.Values{"grant_type": {"authorization_code"}, "code": {code},
		"redirect_uri": {redirectURI("linkedin")}, "client_id": {os.Getenv("LINKEDIN_CLIENT_ID")}, "client_secret": {os.Getenv("LINKEDIN_CLIENT_SECRET")}}, ""), &tok)
	if err != nil {
		return nil, err
	}
	var me struct{ Sub, Name, Picture string }
	if err := remote.Call(remote.Get(linkedinAPI+"/v2/userinfo", "Bearer "+tok.AccessToken), &me); err != nil {
		return nil, err
	}
	out := []socialAccount{{Platform: "linkedin", AccountID: "urn:li:person:" + me.Sub, Name: me.Name, Picture: me.Picture, token: tok.AccessToken, refresh: tok.RefreshToken}}
	if !linkedinPages() {
		return out, nil
	}
	var acls struct {
		Elements []struct {
			Org string `json:"organization"`
		} `json:"elements"`
	}
	if err := remote.Call(linkedin(remote.Get(linkedinAPI+"/rest/organizationAcls?q=roleAssignee&role=ADMINISTRATOR&state=APPROVED", "Bearer "+tok.AccessToken)), &acls); err != nil {
		return nil, err
	}
	for _, e := range acls.Elements {
		var org struct {
			Name string `json:"localizedName"`
		}
		remote.Call(linkedin(remote.Get(linkedinAPI+"/rest/organizations/"+strings.TrimPrefix(e.Org, "urn:li:organization:"), "Bearer "+tok.AccessToken)), &org)
		out = append(out, socialAccount{Platform: "linkedin", AccountID: e.Org, Name: cmp.Or(org.Name, e.Org), token: tok.AccessToken, refresh: tok.RefreshToken})
	}
	return out, nil
}

func xAccounts(code, verifier string) ([]socialAccount, error) {
	t, err := basicTokenCall(xAPI+"/2/oauth2/token", os.Getenv("X_CLIENT_ID"), os.Getenv("X_CLIENT_SECRET"), url.Values{"grant_type": {"authorization_code"},
		"code": {code}, "redirect_uri": {redirectURI("x")}, "code_verifier": {verifier}, "client_id": {os.Getenv("X_CLIENT_ID")}})
	if err != nil {
		return nil, err
	}
	var me struct {
		Data struct {
			ID       string `json:"id"`
			Username string `json:"username"`
			Picture  string `json:"profile_image_url"`
		} `json:"data"`
	}
	if err := remote.Call(remote.Get(xAPI+"/2/users/me?user.fields=profile_image_url", "Bearer "+t.AccessToken), &me); err != nil {
		return nil, err
	}
	return []socialAccount{{Platform: "x", AccountID: me.Data.ID, Name: "@" + me.Data.Username, Picture: me.Data.Picture, token: t.AccessToken, refresh: t.RefreshToken}}, nil
}

func pinterestAccounts(code string) ([]socialAccount, error) {
	t, err := basicTokenCall(pinterestAPI+"/v5/oauth/token", os.Getenv("PINTEREST_APP_ID"), os.Getenv("PINTEREST_APP_SECRET"),
		url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI("pinterest")}})
	if err != nil {
		return nil, err
	}
	var u struct {
		Username string `json:"username"`
		Image    string `json:"profile_image"`
	}
	remote.Call(remote.Get(pinterestAPI+"/v5/user_account", "Bearer "+t.AccessToken), &u)
	var boards struct {
		Items []struct{ ID, Name string } `json:"items"`
	}
	if err := remote.Call(remote.Get(pinterestAPI+"/v5/boards?page_size=100", "Bearer "+t.AccessToken), &boards); err != nil {
		return nil, err
	}
	var out []socialAccount
	for _, b := range boards.Items { // a Pin goes to one board, so each board is an account the editor can pick
		out = append(out, socialAccount{Platform: "pinterest", AccountID: b.ID, Name: b.Name + " · @" + u.Username, Picture: u.Image, token: t.AccessToken, refresh: t.RefreshToken})
	}
	return out, nil
}

// ConnectBluesky adds an account with an app password (bsky.app → Settings → Privacy and security → App passwords).
// The password is stored as the token and the sign-in host as `refresh`: Bluesky sessions are short, so each post signs in.
func ConnectBluesky(r *http.Request) (any, error) {
	b, err := httpapi.Body(r)
	if err != nil {
		return nil, err
	}
	handle, pass := strings.TrimPrefix(strings.TrimSpace(store.Str(b["handle"])), "@"), strings.TrimSpace(store.Str(b["password"]))
	service := strings.TrimRight(cmp.Or(strings.TrimSpace(store.Str(b["service"])), "https://bsky.social"), "/")
	if handle == "" || pass == "" {
		return nil, httpapi.Bad("enter the handle and an app password")
	}
	if u, err := url.Parse(service); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, httpapi.Bad("the server should look like https://bsky.social")
	}
	s, err := blueskySession(service, handle, pass)
	if err != nil {
		return nil, httpapi.Bad("Bluesky didn't accept that handle and app password: " + err.Error())
	}
	var prof struct {
		Avatar string `json:"avatar"`
	}
	remote.Call(remote.Get(s.PDS+"/xrpc/app.bsky.actor.getProfile?actor="+url.QueryEscape(s.DID), "Bearer "+s.Access), &prof)
	_, err = store.DB.Exec(`insert into social_accounts(tenant_id,platform,account_id,name,picture,token,refresh) values(?,'bluesky',?,?,?,?,?)
	  on conflict(tenant_id,platform,account_id) do update set name=excluded.name, picture=excluded.picture, token=excluded.token, refresh=excluded.refresh`,
		httpapi.PathID(r), s.DID, "@"+s.Handle, prof.Avatar, pass, service)
	return nil, err
}

// ConnectMastodon adds an account with an access token from the server's Preferences → Development → New application
// (scopes read:accounts write:media write:statuses). The server URL is stored as `refresh`.
func ConnectMastodon(r *http.Request) (any, error) {
	b, err := httpapi.Body(r)
	if err != nil {
		return nil, err
	}
	server, tok := strings.TrimRight(strings.TrimSpace(store.Str(b["server"])), "/"), strings.TrimSpace(store.Str(b["token"]))
	if server != "" && !strings.Contains(server, "://") {
		server = "https://" + server
	}
	u, err := url.Parse(server)
	if err != nil || u.Host == "" || tok == "" {
		return nil, httpapi.Bad("enter your server (like mastodon.social) and an access token")
	}
	var me struct {
		ID     string `json:"id"`
		Acct   string `json:"acct"`
		Avatar string `json:"avatar"`
	}
	if err := remote.Call(remote.Get(server+"/api/v1/accounts/verify_credentials", "Bearer "+tok), &me); err != nil {
		return nil, httpapi.Bad("that server didn't accept the token: " + err.Error())
	}
	name := "@" + me.Acct
	if !strings.Contains(me.Acct, "@") {
		name += "@" + u.Host
	}
	_, err = store.DB.Exec(`insert into social_accounts(tenant_id,platform,account_id,name,picture,token,refresh) values(?,'mastodon',?,?,?,?,?)
	  on conflict(tenant_id,platform,account_id) do update set name=excluded.name, picture=excluded.picture, token=excluded.token, refresh=excluded.refresh`,
		httpapi.PathID(r), me.ID, name, me.Avatar, tok, server)
	return nil, err
}

func postLinkedIn(author, tok, file, caption string) (string, string, error) {
	st, err := os.Stat(file)
	if err != nil {
		return "", "", err
	}
	var init struct {
		Value struct {
			Video       string `json:"video"`
			UploadToken string `json:"uploadToken"`
			Parts       []struct {
				URL   string `json:"uploadUrl"`
				First int64  `json:"firstByte"`
				Last  int64  `json:"lastByte"`
			} `json:"uploadInstructions"`
		} `json:"value"`
	}
	req := remote.JSON("POST", linkedinAPI+"/rest/videos?action=initializeUpload", map[string]any{"initializeUploadRequest": map[string]any{
		"owner": author, "fileSizeBytes": st.Size(), "uploadCaptions": false, "uploadThumbnail": false}}, "Bearer "+tok)
	if err := remote.Call(linkedin(req), &init); err != nil {
		return "", "", err
	}
	f, _ := os.Open(file)
	defer f.Close()
	var etags []string
	for _, p := range init.Value.Parts { // LinkedIn splits uploads into 4 MB parts and wants each part's ETag back
		up, _ := http.NewRequest("PUT", p.URL, io.NewSectionReader(f, p.First, p.Last-p.First+1))
		up.ContentLength = p.Last - p.First + 1
		up.Header.Set("Content-Type", "application/octet-stream")
		up.Header.Set("Authorization", "Bearer "+tok)
		h, err := remote.CallHeader(remote.Uploads, up)
		if err != nil {
			return "", "", err
		}
		etags = append(etags, h.Get("ETag"))
	}
	req = remote.JSON("POST", linkedinAPI+"/rest/videos?action=finalizeUpload", map[string]any{"finalizeUploadRequest": map[string]any{
		"video": init.Value.Video, "uploadToken": init.Value.UploadToken, "uploadedPartIds": etags}}, "Bearer "+tok)
	if err := remote.Call(linkedin(req), nil); err != nil {
		return "", "", err
	}
	for i := 0; ; i++ { // a post can only use the video once LinkedIn has processed it
		var v struct {
			Status string `json:"status"`
		}
		if err := remote.Call(linkedin(remote.Get(linkedinAPI+"/rest/videos/"+url.QueryEscape(init.Value.Video), "Bearer "+tok)), &v); err != nil {
			return "", "", err
		}
		if v.Status == "AVAILABLE" {
			break
		}
		if v.Status == "PROCESSING_FAILED" || i > 120 {
			return "", "", fmt.Errorf("linkedin could not process the video (%s)", cmp.Or(v.Status, "timed out"))
		}
		time.Sleep(5 * time.Second)
	}
	h, err := remote.CallHeader(remote.Client, linkedin(remote.JSON("POST", linkedinAPI+"/rest/posts", map[string]any{
		"author": author, "commentary": linkedinText(caption), "visibility": "PUBLIC", "lifecycleState": "PUBLISHED", "isReshareDisabledByAuthor": false,
		"distribution": map[string]any{"feedDistribution": "MAIN_FEED", "targetEntities": []any{}, "thirdPartyDistributionChannels": []any{}},
		"content":      map[string]any{"media": map[string]any{"id": init.Value.Video}},
	}, "Bearer "+tok)))
	if err != nil {
		return "", "", err
	}
	id := h.Get("X-Restli-Id")
	return id, "https://www.linkedin.com/feed/update/" + id + "/", nil
}

var littleRe = regexp.MustCompile(`[\\|{}@\[\]()<>#*_~]`)

var hashRe = regexp.MustCompile(`#([\p{L}\p{M}\p{N}_]+)`)

// linkedinText escapes the characters LinkedIn's post markup reserves (unescaped, they can cut the post short) and
// turns #tags into real hashtags.
func linkedinText(s string) string {
	esc := func(s string) string { return littleRe.ReplaceAllString(s, `\$0`) }
	var b strings.Builder
	last := 0
	for _, m := range hashRe.FindAllStringSubmatchIndex(s, -1) {
		b.WriteString(esc(s[last:m[0]]) + `{hashtag|\#|` + esc(s[m[2]:m[3]]) + "}")
		last = m[1]
	}
	b.WriteString(esc(s[last:]))
	return b.String()
}

func postX(tok, file, text string) (string, string, error) {
	st, err := os.Stat(file)
	if err != nil {
		return "", "", err
	}
	var m struct {
		Data struct {
			ID         string `json:"id"`
			Processing *struct {
				State string `json:"state"`
				After int    `json:"check_after_secs"`
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			} `json:"processing_info"`
		} `json:"data"`
	}
	body := map[string]any{"media_type": "video/mp4", "total_bytes": st.Size(), "media_category": "tweet_video"}
	if err := remote.Call(remote.JSON("POST", xAPI+"/2/media/upload/initialize", body, "Bearer "+tok), &m); err != nil {
		return "", "", err
	}
	id := m.Data.ID
	f, _ := os.Open(file)
	defer f.Close()
	const chunk = 4 << 20
	for i, off := 0, int64(0); off < st.Size(); i, off = i+1, off+chunk {
		part := io.NewSectionReader(f, off, min(chunk, st.Size()-off))
		if err := remote.CallWith(remote.Uploads, remote.Multipart(xAPI+"/2/media/upload/"+id+"/append", map[string]string{"segment_index": fmt.Sprint(i)}, "media", "chunk", part, "Bearer "+tok), nil); err != nil {
			return "", "", err
		}
	}
	if err := remote.Call(remote.Form("POST", xAPI+"/2/media/upload/"+id+"/finalize", nil, "Bearer "+tok), &m); err != nil {
		return "", "", err
	}
	for i := 0; m.Data.Processing != nil && m.Data.Processing.State != "succeeded"; i++ { // X transcodes before a post can use it
		if m.Data.Processing.State == "failed" || i > 120 {
			return "", "", fmt.Errorf("x could not process the video: %s", cmp.Or(m.Data.Processing.Error.Message, m.Data.Processing.State))
		}
		time.Sleep(time.Duration(min(max(m.Data.Processing.After, 2), 10)) * time.Second)
		m.Data.Processing = nil
		if err := remote.Call(remote.Get(xAPI+"/2/media/upload?"+url.Values{"media_id": {id}, "command": {"STATUS"}}.Encode(), "Bearer "+tok), &m); err != nil {
			return "", "", err
		}
	}
	var t struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	err = remote.Call(remote.JSON("POST", xAPI+"/2/tweets", map[string]any{"text": textutil.Cut(text, 280), "media": map[string]any{"media_ids": []string{id}}}, "Bearer "+tok), &t)
	return t.Data.ID, "https://x.com/i/web/status/" + t.Data.ID, err
}

func postPinterest(board, tok, file, title, caption string) (string, string, error) {
	var m struct {
		ID     string            `json:"media_id"`
		URL    string            `json:"upload_url"`
		Params map[string]string `json:"upload_parameters"`
	}
	if err := remote.Call(remote.JSON("POST", pinterestAPI+"/v5/media", map[string]string{"media_type": "video"}, "Bearer "+tok), &m); err != nil {
		return "", "", err
	}
	f, err := os.Open(file)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	if err := remote.CallWith(remote.Uploads, remote.Multipart(m.URL, m.Params, "file", filepath.Base(file), f, ""), nil); err != nil {
		return "", "", err
	}
	for i := 0; ; i++ {
		var s struct {
			Status string `json:"status"`
		}
		if err := remote.Call(remote.Get(pinterestAPI+"/v5/media/"+m.ID, "Bearer "+tok), &s); err != nil {
			return "", "", err
		}
		if s.Status == "succeeded" {
			break
		}
		if s.Status == "failed" || i > 120 {
			return "", "", fmt.Errorf("pinterest could not process the video (%s)", cmp.Or(s.Status, "timed out"))
		}
		time.Sleep(5 * time.Second)
	}
	var p struct {
		ID string `json:"id"`
	}
	err = remote.Call(remote.JSON("POST", pinterestAPI+"/v5/pins", map[string]any{"board_id": board, "title": textutil.Cut(title, 100), "description": textutil.Cut(caption, 800),
		"media_source": map[string]any{"source_type": "video_id", "media_id": m.ID, "cover_image_key_frame_time": 1}}, "Bearer "+tok), &p)
	return p.ID, "https://www.pinterest.com/pin/" + p.ID + "/", err
}

type bskySession struct{ DID, Handle, Access, PDS string }

// blueskySession signs in on the entryway (bsky.social, or a self-hosted server) and finds the account's own PDS.
func blueskySession(service, id, pass string) (bskySession, error) {
	var res struct {
		DID    string `json:"did"`
		Handle string `json:"handle"`
		Access string `json:"accessJwt"`
		Doc    struct {
			Service []struct {
				ID       string `json:"id"`
				Endpoint string `json:"serviceEndpoint"`
			} `json:"service"`
		} `json:"didDoc"`
	}
	err := remote.Call(remote.JSON("POST", service+"/xrpc/com.atproto.server.createSession", map[string]string{"identifier": id, "password": pass}, ""), &res)
	s := bskySession{DID: res.DID, Handle: res.Handle, Access: res.Access, PDS: service}
	for _, sv := range res.Doc.Service {
		if sv.ID == "#atproto_pds" && sv.Endpoint != "" {
			s.PDS = strings.TrimRight(sv.Endpoint, "/")
		}
	}
	return s, err
}

// postBluesky uploads the video as a blob to the account's PDS and posts it. ponytail: plain uploadBlob; Bluesky's
// video service (app.bsky.video.uploadVideo) transcodes before posting and reports errors earlier, if this proves flaky.
func postBluesky(did, pass, service, file, text string) (string, string, error) {
	s, err := blueskySession(service, did, pass)
	if err != nil {
		return "", "", err
	}
	f, err := os.Open(file)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	st, _ := f.Stat()
	req, _ := http.NewRequest("POST", s.PDS+"/xrpc/com.atproto.repo.uploadBlob", f)
	req.ContentLength = st.Size()
	req.Header.Set("Content-Type", "video/mp4")
	req.Header.Set("Authorization", "Bearer "+s.Access)
	var up struct {
		Blob json.RawMessage `json:"blob"`
	}
	if err := remote.CallWith(remote.Uploads, req, &up); err != nil {
		return "", "", err
	}
	text = textutil.Cut(text, 300)
	embed := map[string]any{"$type": "app.bsky.embed.video", "video": up.Blob}
	if w, h := files.VideoSize(file); w > 0 && h > 0 {
		embed["aspectRatio"] = map[string]int{"width": w, "height": h}
	}
	rec := map[string]any{"$type": "app.bsky.feed.post", "text": text, "createdAt": time.Now().UTC().Format(time.RFC3339), "embed": embed}
	if fs := bskyFacets(text); len(fs) > 0 {
		rec["facets"] = fs
	}
	var out struct {
		URI string `json:"uri"`
	}
	err = remote.Call(remote.JSON("POST", s.PDS+"/xrpc/com.atproto.repo.createRecord", map[string]any{"repo": s.DID, "collection": "app.bsky.feed.post", "record": rec}, "Bearer "+s.Access), &out)
	return out.URI, "https://bsky.app/profile/" + s.DID + "/post/" + out.URI[strings.LastIndex(out.URI, "/")+1:], err
}

var linkRe = regexp.MustCompile(`https?://\S+`)

// bskyFacets makes #tags and links clickable. Bluesky marks them by UTF-8 byte range, which is what Go's regexp returns.
func bskyFacets(text string) []map[string]any {
	var fs []map[string]any
	add := func(start, end int, feature map[string]any) {
		fs = append(fs, map[string]any{"index": map[string]int{"byteStart": start, "byteEnd": end}, "features": []any{feature}})
	}
	for _, m := range hashRe.FindAllStringSubmatchIndex(text, -1) {
		add(m[0], m[1], map[string]any{"$type": "app.bsky.richtext.facet#tag", "tag": text[m[2]:m[3]]})
	}
	for _, m := range linkRe.FindAllStringIndex(text, -1) {
		link := strings.TrimRight(text[m[0]:m[1]], ".,;:!?)")
		add(m[0], m[0]+len(link), map[string]any{"$type": "app.bsky.richtext.facet#link", "uri": link})
	}
	return fs
}

func postMastodon(server, tok, file, text string) (string, string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	var m struct {
		ID  string  `json:"id"`
		URL *string `json:"url"` // null until the server has processed the video
	}
	if err := remote.CallWith(remote.Uploads, remote.Multipart(server+"/api/v2/media", nil, "file", filepath.Base(file), f, "Bearer "+tok), &m); err != nil {
		return "", "", err
	}
	for i := 0; m.URL == nil; i++ {
		if i > 120 {
			return "", "", errors.New("mastodon is still processing the video; try again in a few minutes")
		}
		time.Sleep(3 * time.Second)
		if err := remote.Call(remote.Get(server+"/api/v1/media/"+m.ID, "Bearer "+tok), &m); err != nil {
			return "", "", err
		}
	}
	var s struct{ ID, URL string }
	err = remote.Call(remote.JSON("POST", server+"/api/v1/statuses", map[string]any{"status": textutil.Cut(text, 500), "media_ids": []string{m.ID}}, "Bearer "+tok), &s)
	return s.ID, s.URL, err
}
