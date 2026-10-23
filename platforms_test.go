package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LinkedIn's post markup: reserved characters escaped, #tags (Bangla ones too) turned into hashtags.
func TestLinkedinText(t *testing.T) {
	got := linkedinText("New (free) reel @ 8pm! #BCS #বিসিএস_প্রস্তুতি")
	want := `New \(free\) reel \@ 8pm! {hashtag|\#|BCS} {hashtag|\#|বিসিএস\_প্রস্তুতি}`
	if got != want {
		t.Fatalf("linkedinText = %q\nwant %q", got, want)
	}
}

// Bluesky facets are UTF-8 byte ranges.
func TestBskyFacets(t *testing.T) {
	text := "আজকের টিপ #BCS https://bcsmate.com."
	fs := bskyFacets(text)
	if len(fs) != 2 {
		t.Fatalf("facets = %v", fs)
	}
	idx := fs[1]["index"].(map[string]int)
	if text[idx["byteStart"]:idx["byteEnd"]] != "https://bcsmate.com" {
		t.Fatalf("link facet covers %q", text[idx["byteStart"]:idx["byteEnd"]])
	}
	idx = fs[0]["index"].(map[string]int)
	if text[idx["byteStart"]:idx["byteEnd"]] != "#BCS" {
		t.Fatalf("tag facet covers %q", text[idx["byteStart"]:idx["byteEnd"]])
	}
}

// Platform app secrets never reach the agents.
func TestAgentEnvSecrets(t *testing.T) {
	t.Setenv("X_CLIENT_SECRET", "s")
	t.Setenv("GEMINI_API_KEY", "g")
	env := strings.Join(agentEnv(), "\n")
	if strings.Contains(env, "X_CLIENT_SECRET=") || !strings.Contains(env, "GEMINI_API_KEY=g") {
		t.Fatal("agentEnv should drop *_SECRET and keep AI keys")
	}
}

// Mastodon and Bluesky run against the server the user names, so their whole connect → post flow runs on a stand-in.
func TestMastodonAndBluesky(t *testing.T) {
	dataDir = t.TempDir()
	openDB()
	db.Exec("insert into tenants(id,name) values(1,'Shop')")
	video := filepath.Join(t.TempDir(), "reel.mp4")
	os.WriteFile(video, []byte("not really a video"), 0o644)

	var srv *httptest.Server
	var posted map[string]any
	polls := 0
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		switch r.URL.Path {
		// Mastodon
		case "/api/v1/accounts/verify_credentials":
			if auth != "Bearer tok" {
				w.WriteHeader(401)
				json.NewEncoder(w).Encode(map[string]string{"error": "The access token is invalid"})
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"id": "42", "acct": "shop", "avatar": "a.png"})
		case "/api/v2/media":
			if b, _ := io.ReadAll(r.Body); !strings.Contains(string(b), "not really a video") {
				t.Error("mastodon media upload is missing the file")
			}
			w.WriteHeader(202)
			w.Write([]byte(`{"id":"m1","url":null}`))
		case "/api/v1/media/m1":
			polls++
			w.Write([]byte(`{"id":"m1","url":"https://files/m1.mp4"}`))
		case "/api/v1/statuses":
			json.NewDecoder(r.Body).Decode(&posted)
			w.Write([]byte(`{"id":"s1","url":"https://masto/@shop/s1"}`))
		// Bluesky
		case "/xrpc/com.atproto.server.createSession":
			var b map[string]string
			json.NewDecoder(r.Body).Decode(&b)
			if b["password"] != "app-pass" {
				w.WriteHeader(401)
				json.NewEncoder(w).Encode(map[string]string{"error": "AuthenticationRequired", "message": "Invalid identifier or password"})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"did": "did:plc:abc", "handle": "shop.bsky.social", "accessJwt": "jwt",
				"didDoc": map[string]any{"service": []map[string]string{{"id": "#atproto_pds", "serviceEndpoint": srv.URL}}}})
		case "/xrpc/app.bsky.actor.getProfile":
			w.Write([]byte(`{"avatar":"av.jpg"}`))
		case "/xrpc/com.atproto.repo.uploadBlob":
			if auth != "Bearer jwt" || r.Header.Get("Content-Type") != "video/mp4" {
				t.Errorf("uploadBlob auth %q type %q", auth, r.Header.Get("Content-Type"))
			}
			w.Write([]byte(`{"blob":{"$type":"blob","ref":{"$link":"bafk"},"mimeType":"video/mp4","size":18}}`))
		case "/xrpc/com.atproto.repo.createRecord":
			json.NewDecoder(r.Body).Decode(&posted)
			w.Write([]byte(`{"uri":"at://did:plc:abc/app.bsky.feed.post/3kx","cid":"c"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	connect := func(fn func(*http.Request) (any, error), body string) error {
		req := httptest.NewRequest("POST", "/api/tenants/1/x", strings.NewReader(body))
		req.SetPathValue("id", "1")
		_, err := fn(req)
		return err
	}
	if err := connect(connectMastodon, `{"server":"`+srv.URL+`","token":"wrong"}`); err == nil || !strings.Contains(err.Error(), "access token is invalid") {
		t.Fatalf("bad mastodon token: %v", err)
	}
	if err := connect(connectMastodon, `{"server":"`+srv.URL+`/","token":"tok"}`); err != nil {
		t.Fatal(err)
	}
	a, _ := row("select * from social_accounts where platform='mastodon'")
	if a["name"] != "@shop@"+strings.TrimPrefix(srv.URL, "http://") || a["refresh"] != srv.URL {
		t.Fatalf("mastodon account = %v", a)
	}
	id, link, err := postMastodon(str(a["refresh"]), str(a["token"]), video, "Hello #Reels")
	if err != nil || id != "s1" || link != "https://masto/@shop/s1" || polls != 1 || posted["media_ids"].([]any)[0] != "m1" {
		t.Fatalf("mastodon post: %s %s %v polls=%d %v", id, link, err, polls, posted)
	}

	if err := connect(connectBluesky, `{"handle":"@shop.bsky.social","password":"nope","service":"`+srv.URL+`"}`); err == nil || !strings.Contains(err.Error(), "Invalid identifier or password") {
		t.Fatalf("bad bluesky password: %v", err)
	}
	if err := connect(connectBluesky, `{"handle":"@shop.bsky.social","password":"app-pass","service":"`+srv.URL+`"}`); err != nil {
		t.Fatal(err)
	}
	a, _ = row("select * from social_accounts where platform='bluesky'")
	if a["account_id"] != "did:plc:abc" || a["name"] != "@shop.bsky.social" || a["picture"] != "av.jpg" {
		t.Fatalf("bluesky account = %v", a)
	}
	id, link, err = postBluesky(str(a["account_id"]), str(a["token"]), str(a["refresh"]), video, "New reel #BCS")
	if err != nil || link != "https://bsky.app/profile/did:plc:abc/post/3kx" {
		t.Fatalf("bluesky post: %s %s %v", id, link, err)
	}
	rec := posted["record"].(map[string]any)
	if rec["embed"].(map[string]any)["$type"] != "app.bsky.embed.video" || len(rec["facets"].([]any)) != 1 {
		t.Fatalf("bluesky record = %v", rec)
	}
}

// X: chunked upload, finalize, wait for processing, then the post with the media attached.
func TestPostX(t *testing.T) {
	video := filepath.Join(t.TempDir(), "reel.mp4")
	os.WriteFile(video, []byte(strings.Repeat("v", 5<<20)), 0o644) // two 4 MB chunks
	var calls []string
	var tweet map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path+" "+r.URL.Query().Get("command"))
		switch r.URL.Path {
		case "/2/media/upload/initialize":
			w.Write([]byte(`{"data":{"id":"77"}}`))
		case "/2/media/upload/77/append":
			r.ParseMultipartForm(8 << 20)
			if r.FormValue("segment_index") == "" {
				t.Error("append without segment_index")
			}
		case "/2/media/upload/77/finalize":
			w.Write([]byte(`{"data":{"id":"77","processing_info":{"state":"pending","check_after_secs":1}}}`))
		case "/2/media/upload":
			w.Write([]byte(`{"data":{"id":"77","processing_info":{"state":"succeeded"}}}`))
		case "/2/tweets":
			json.NewDecoder(r.Body).Decode(&tweet)
			w.Write([]byte(`{"data":{"id":"1001","text":"hi"}}`))
		}
	}))
	defer srv.Close()
	old := xAPI
	xAPI = srv.URL
	defer func() { xAPI = old }()
	id, link, err := postX("tok", video, "New reel #BCS")
	if err != nil || id != "1001" || link != "https://x.com/i/web/status/1001" {
		t.Fatalf("postX = %s %s %v (calls %v)", id, link, err, calls)
	}
	want := "POST /2/media/upload/initialize |POST /2/media/upload/77/append |POST /2/media/upload/77/append |POST /2/media/upload/77/finalize |GET /2/media/upload STATUS|POST /2/tweets "
	if strings.Join(calls, "|") != want || tweet["media"].(map[string]any)["media_ids"].([]any)[0] != "77" {
		t.Fatalf("calls %v, tweet %v", calls, tweet)
	}
}
