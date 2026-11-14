// Package integrations keeps the services connected without built-in support: any HTTP API that takes a key, like
// Cartesia, Deepgram or Runway. The assistant (or any MCP client) reads the service's docs and saves how to call it
// here: its base URL, how the key is sent, fixed headers, and a guide for the editor agent. The agent then calls it with
// `37studio api`, which adds the key, so a new service needs no code of its own.
package integrations

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/keys"
	"github.com/Shaitrish/37studio/internal/remote"
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/textutil"
)

const defaultAuth = "Authorization: Bearer {key}"

var validID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

// Kinds are what a service can be used for; the harness names them so the agent knows when to reach for one.
var Kinds = []string{"voice", "image", "video", "music", "sfx", "transcribe", "other"}

// All lists the connected services without their keys, for the API, the UI and the harness.
func All() []map[string]any {
	rs, _ := store.Rows("select * from integrations order by name")
	for _, r := range rs {
		k := keys.OpenText(store.Str(r["key"]))
		delete(r, "key")
		r["set"], r["last4"] = k != "", k[max(0, len(k)-4):]
		var h map[string]string
		json.Unmarshal([]byte(store.Str(r["headers"])), &h)
		r["headers"], r["prefer"] = h, store.Num(r["prefer"]) != 0
	}
	return rs
}

func List(r *http.Request) (any, error) { return All(), nil }

// Put saves a service by its id, creating it if new; only the fields passed change. Moving it to another host drops
// its key unless a key comes with the change, so a key typed for one service is never sent to a different one.
func Put(r *http.Request) (any, error) {
	b, err := httpapi.Body(r)
	if err != nil {
		return nil, err
	}
	id := r.PathValue("service")
	if !validID.MatchString(id) {
		return nil, httpapi.Bad("the service id must be short, lowercase letters, digits and dashes, like cartesia")
	}
	old, _ := store.Row("select * from integrations where id=?", id)
	val := func(k string) string {
		if v, ok := b[k]; ok {
			return strings.TrimSpace(store.Str(v))
		}
		return store.Str(old[k])
	}
	name, base, auth, note := val("name"), strings.TrimRight(val("base_url"), "/"), val("auth"), ""
	if name == "" {
		name = id
	}
	if auth == "" {
		auth = defaultAuth
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, httpapi.Bad("base_url must be the service's https API address, like https://api.cartesia.ai")
	}
	if !strings.Contains(auth, "{key}") || !(strings.HasPrefix(auth, "?") || strings.Contains(auth, ":")) {
		return nil, httpapi.Bad(`auth says how the key is sent: a header line like "X-API-Key: {key}" or "Authorization: Bearer {key}", or a query parameter like "?key={key}"`)
	}
	headers := store.Str(old["headers"])
	if v, ok := b["headers"]; ok {
		var h map[string]string
		if s, isStr := v.(string); isStr {
			err = json.Unmarshal([]byte(cmp.Or(s, "{}")), &h)
		} else {
			raw, _ := json.Marshal(v)
			err = json.Unmarshal(raw, &h)
		}
		if err != nil {
			return nil, httpapi.Bad(`headers must be an object of header name to value, like {"Cartesia-Version": "2025-04-16"}`)
		}
		raw, _ := json.Marshal(h)
		headers = string(raw)
	}
	kinds := val("kinds")
	if v, ok := b["kinds"].([]any); ok {
		var ks []string
		for _, k := range v {
			ks = append(ks, store.Str(k))
		}
		kinds = strings.Join(ks, ",")
	}
	for _, k := range strings.Split(kinds, ",") {
		if k = strings.TrimSpace(k); k != "" && !slices.Contains(Kinds, k) {
			return nil, httpapi.Bad("unknown kind " + k + "; use " + strings.Join(Kinds, ", "))
		}
	}
	prefer := store.Num(old["prefer"]) != 0
	if v, ok := b["prefer"].(bool); ok {
		prefer = v
	}
	price := store.Num(old["unit_price"])
	if v, ok := b["unit_price"]; ok {
		price = store.Num(v)
	}
	key := store.Str(old["key"])
	if v, ok := b["key"]; ok {
		key = keys.SealText(strings.TrimSpace(store.Str(v)))
	} else if old != nil && hostOf(store.Str(old["base_url"])) != u.Host && key != "" {
		key, note = "", "The address moved to another host, so its key was removed: save the key again."
	}
	_, err = store.DB.Exec(`insert into integrations(id,name,base_url,auth,headers,kinds,prefer,guide,docs_url,unit,unit_price,key) values(?,?,?,?,?,?,?,?,?,?,?,?)
	  on conflict(id) do update set name=excluded.name, base_url=excluded.base_url, auth=excluded.auth, headers=excluded.headers, kinds=excluded.kinds,
	  prefer=excluded.prefer, guide=excluded.guide, docs_url=excluded.docs_url, unit=excluded.unit, unit_price=excluded.unit_price, key=excluded.key`,
		id, name, base, auth, headers, kinds, prefer, val("guide"), val("docs_url"), val("unit"), price, key)
	if err != nil {
		return nil, err
	}
	for _, s := range All() {
		if s["id"] == id {
			if note != "" {
				s["note"] = note
			}
			return s, nil
		}
	}
	return nil, errors.New("saved, but can't read it back")
}

func Delete(r *http.Request) (any, error) {
	_, err := store.DB.Exec("delete from integrations where id=?", r.PathValue("service"))
	return map[string]bool{"ok": true}, err
}

// Request builds one call to a service, signed with its key. path is under the service's base URL and can't leave it.
func Request(id, method, path string, body io.Reader, contentType string) (*http.Request, error) {
	s, err := store.Row("select * from integrations where id=?", id)
	if err != nil {
		return nil, fmt.Errorf("no service called %q is connected; Settings → AI services → Other services lists them", id)
	}
	key := keys.OpenText(store.Str(s["key"]))
	if key == "" {
		return nil, fmt.Errorf("%s has no key yet: add it in Settings → AI services → Other services", s["name"])
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return nil, errors.New("the path must start with /, like /voices, and is added to " + store.Str(s["base_url"]))
	}
	u, err := url.Parse(store.Str(s["base_url"]) + path)
	if err != nil || u.Host != hostOf(store.Str(s["base_url"])) {
		return nil, errors.New("that path leaves " + store.Str(s["base_url"]))
	}
	auth := strings.ReplaceAll(store.Str(s["auth"]), "{key}", key)
	if q, ok := strings.CutPrefix(auth, "?"); ok {
		extra, _ := url.ParseQuery(q)
		v := u.Query()
		for k, vs := range extra {
			v[k] = vs
		}
		u.RawQuery = v.Encode()
	}
	req, err := http.NewRequest(strings.ToUpper(method), u.String(), body)
	if err != nil {
		return nil, err
	}
	var h map[string]string
	json.Unmarshal([]byte(store.Str(s["headers"])), &h)
	for k, v := range h {
		req.Header.Set(k, v)
	}
	if name, val, ok := strings.Cut(auth, ":"); ok && !strings.HasPrefix(auth, "?") {
		req.Header.Set(strings.TrimSpace(name), strings.TrimSpace(val))
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req, nil
}

// Error is a service's own message for a failed call, which is what tells the agent or the user what to fix.
func Error(res *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	return fmt.Errorf("%s answered %d: %s", res.Request.URL.Host, res.StatusCode, remote.Message(raw))
}

// Call is a quick call from the assistant or the UI, to check a key or list a service's voices and models. The reply
// comes back as text; a file (audio, an image) is only described, since `37studio api --out` is how files are made.
func Call(r *http.Request) (any, error) {
	b, err := httpapi.Body(r)
	if err != nil {
		return nil, err
	}
	var body io.Reader
	ct := ""
	if v, ok := b["json"]; ok && v != nil {
		raw, _ := json.Marshal(v)
		body, ct = strings.NewReader(string(raw)), "application/json"
	}
	req, err := Request(r.PathValue("service"), cmp.Or(store.Str(b["method"]), "GET"), cmp.Or(store.Str(b["path"]), "/"), body, ct)
	if err != nil {
		return nil, httpapi.Bad(err.Error())
	}
	res, err := remote.Client.Do(req)
	if err != nil {
		return nil, httpapi.Bad(err.Error())
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return nil, httpapi.Bad(Error(res).Error())
	}
	t := res.Header.Get("Content-Type")
	if !strings.Contains(t, "json") && !strings.HasPrefix(t, "text/") {
		n, _ := io.Copy(io.Discard, res.Body)
		return map[string]any{"status": res.StatusCode, "content_type": t, "body": fmt.Sprintf("%d bytes of %s: the call works; the editor agent saves files like this with 37studio api --out", n, t)}, nil
	}
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	return map[string]any{"status": res.StatusCode, "content_type": t, "body": textutil.Head(string(raw), 20000)}, nil
}

// Prompt is the editor agent's section on the connected services; "" when there are none.
func Prompt() string {
	var lines []string
	for _, s := range All() {
		if s["set"] != true {
			continue
		}
		head := fmt.Sprintf("- %s (37studio api %s", s["name"], s["id"])
		if k := store.Str(s["kinds"]); k != "" {
			head += "; for " + strings.ReplaceAll(k, ",", ", ")
		}
		if s["prefer"] == true && store.Str(s["kinds"]) != "" {
			head += "; the editor's choice for these: use it instead of 37studio gen for them"
		}
		if p := store.Num(s["unit_price"]); p > 0 && store.Str(s["unit"]) != "" {
			head += fmt.Sprintf("; billed $%g per %s, pass --units in %s", p, s["unit"], s["unit"])
		}
		lines = append(lines, head+")")
		if g := strings.TrimSpace(store.Str(s["guide"])); g != "" {
			lines = append(lines, "  "+strings.ReplaceAll(textutil.Head(g, 4000), "\n", "\n  "))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "\n## Other services the editor connected (billed per use, like the media above)\n" +
		`Call one with: 37studio api <service> <METHOD> </path> [--json '<body>' | --json-file body.json] [--out file] [--units N --item "what it made"]
The app adds the key and the service's fixed headers; you never see or need the key. --out saves the reply (audio, images, video); without it the reply is printed. Pass --units with what the call bills (in the service's unit), so the cost lands on the project and the spending check applies; it exits with "Ask the editor first" like gen does. Use a service when the editor asks for it by name, or when it is their choice for a kind.
` + strings.Join(lines, "\n") + "\n"
}

func hostOf(u string) string {
	p, _ := url.Parse(u)
	if p == nil {
		return ""
	}
	return p.Host
}
