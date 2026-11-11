package social

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/proc"
	"github.com/Shaitrish/37studio/internal/store"
)

const pic = "not really a png"

// Each platform that takes images gets one through its image API: endpoint, fields and auth on a stand-in server.
func TestImagePosts(t *testing.T) {
	img := filepath.Join(t.TempDir(), "pic.png")
	os.WriteFile(img, []byte(pic), 0o644)
	var srv *httptest.Server
	var got map[string]any // the last JSON post body
	var calls []string
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		auth := r.Header.Get("Authorization")
		switch r.URL.Path {
		// Facebook
		case "/" + graphVer + "/page1/photos":
			r.ParseMultipartForm(1 << 20)
			fh := r.MultipartForm.File["source"]
			if auth != "Bearer tok" || r.FormValue("message") != "Sale today" || r.FormValue("published") != "true" || len(fh) != 1 || fh[0].Filename != "pic.png" {
				t.Errorf("facebook photo: auth %q form %v", auth, r.MultipartForm)
			}
			w.Write([]byte(`{"id":"ph1","post_id":"page1_9"}`))
		// X
		case "/2/media/upload/initialize":
			json.NewDecoder(r.Body).Decode(&got)
			if got["media_type"] != "image/png" || got["media_category"] != "tweet_image" || auth != "Bearer tok" {
				t.Errorf("x initialize %v auth %q", got, auth)
			}
			w.Write([]byte(`{"data":{"id":"88"}}`))
		case "/2/media/upload/88/append":
		case "/2/media/upload/88/finalize":
			w.Write([]byte(`{"data":{"id":"88"}}`)) // images need no processing
		case "/2/tweets":
			json.NewDecoder(r.Body).Decode(&got)
			w.Write([]byte(`{"data":{"id":"2002"}}`))
		// LinkedIn
		case "/rest/images":
			json.NewDecoder(r.Body).Decode(&got)
			if r.URL.Query().Get("action") != "initializeUpload" || got["initializeUploadRequest"].(map[string]any)["owner"] != "urn:li:person:me" ||
				auth != "Bearer tok" || r.Header.Get("LinkedIn-Version") == "" {
				t.Errorf("linkedin initializeUpload %v %v", r.URL, got)
			}
			json.NewEncoder(w).Encode(map[string]any{"value": map[string]string{"uploadUrl": srv.URL + "/li-upload", "image": "urn:li:image:C1"}})
		case "/li-upload":
			if b, _ := io.ReadAll(r.Body); r.Method != "PUT" || string(b) != pic || auth != "Bearer tok" {
				t.Errorf("linkedin upload %s %q", r.Method, b)
			}
			w.WriteHeader(201)
		case "/rest/images/urn:li:image:C1":
			w.Write([]byte(`{"status":"AVAILABLE"}`))
		case "/rest/posts":
			json.NewDecoder(r.Body).Decode(&got)
			w.Header().Set("X-Restli-Id", "urn:li:share:5")
			w.WriteHeader(201)
		// Pinterest
		case "/v5/pins":
			json.NewDecoder(r.Body).Decode(&got)
			if auth != "Bearer tok" || got["board_id"] != "board1" {
				t.Errorf("pinterest pin %v auth %q", got, auth)
			}
			w.Write([]byte(`{"id":"pin3"}`))
		// Bluesky
		case "/xrpc/com.atproto.server.createSession":
			json.NewEncoder(w).Encode(map[string]any{"did": "did:plc:abc", "accessJwt": "jwt"})
		case "/xrpc/com.atproto.repo.uploadBlob":
			if b, _ := io.ReadAll(r.Body); auth != "Bearer jwt" || r.Header.Get("Content-Type") != "image/png" || string(b) != pic {
				t.Errorf("bluesky uploadBlob auth %q type %q", auth, r.Header.Get("Content-Type"))
			}
			w.Write([]byte(`{"blob":{"$type":"blob","ref":{"$link":"bafk"},"mimeType":"image/png","size":16}}`))
		case "/xrpc/com.atproto.repo.createRecord":
			json.NewDecoder(r.Body).Decode(&got)
			w.Write([]byte(`{"uri":"at://did:plc:abc/app.bsky.feed.post/3ky"}`))
		// Mastodon: images are processed at once, so the upload answers 200 with a url
		case "/api/v2/media":
			if b, _ := io.ReadAll(r.Body); auth != "Bearer tok" || !strings.Contains(string(b), pic) {
				t.Error("mastodon media upload is missing the image")
			}
			w.Write([]byte(`{"id":"m2","url":"https://files/m2.png"}`))
		case "/api/v1/statuses":
			json.NewDecoder(r.Body).Decode(&got)
			w.Write([]byte(`{"id":"s2","url":"https://masto/@shop/s2"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	oldGraph, oldX, oldLI, oldPin := graphURL, xAPI, linkedinAPI, pinterestAPI
	graphURL, xAPI, linkedinAPI, pinterestAPI = srv.URL, srv.URL, srv.URL, srv.URL
	defer func() { graphURL, xAPI, linkedinAPI, pinterestAPI = oldGraph, oldX, oldLI, oldPin }()

	check := func(name, id, link string, err error, wantID, wantLink string) {
		t.Helper()
		if err != nil || id != wantID || link != wantLink {
			t.Fatalf("%s = %q %q %v, want %q %q", name, id, link, err, wantID, wantLink)
		}
	}
	id, link, err := postFacebook("page1", "tok", img, "Sale today")
	check("facebook", id, link, err, "ph1", "https://www.facebook.com/page1_9")

	id, link, err = postX("tok", img, "Sale today")
	check("x", id, link, err, "2002", "https://x.com/i/web/status/2002")
	if got["media"].(map[string]any)["media_ids"].([]any)[0] != "88" {
		t.Fatalf("x post %v", got)
	}

	id, link, err = postLinkedIn("urn:li:person:me", "tok", img, "Sale today")
	check("linkedin", id, link, err, "urn:li:share:5", "https://www.linkedin.com/feed/update/urn:li:share:5/")
	if got["content"].(map[string]any)["media"].(map[string]any)["id"] != "urn:li:image:C1" {
		t.Fatalf("linkedin post %v", got)
	}

	id, link, err = postPinterest("board1", "tok", img, "Sale", "Sale today")
	check("pinterest", id, link, err, "pin3", "https://www.pinterest.com/pin/pin3/")
	src := got["media_source"].(map[string]any)
	if src["source_type"] != "image_base64" || src["content_type"] != "image/png" || src["data"] != base64.StdEncoding.EncodeToString([]byte(pic)) {
		t.Fatalf("pinterest media_source %v", src)
	}

	id, link, err = postBluesky("did:plc:abc", "app-pass", srv.URL, img, "Sale today\nEverything half price #sale")
	check("bluesky", id, link, err, "at://did:plc:abc/app.bsky.feed.post/3ky", "https://bsky.app/profile/did:plc:abc/post/3ky")
	embed := got["record"].(map[string]any)["embed"].(map[string]any)
	if images := embed["images"].([]any); embed["$type"] != "app.bsky.embed.images" || len(images) != 1 || images[0].(map[string]any)["alt"] != "Sale today" {
		t.Fatalf("bluesky embed %v", embed)
	}

	id, link, err = postMastodon(srv.URL, "tok", img, "Sale today")
	check("mastodon", id, link, err, "s2", "https://masto/@shop/s2")
	if got["media_ids"].([]any)[0] != "m2" {
		t.Fatalf("mastodon status %v", got)
	}

	// Bluesky's 1 MB limit: an image that can't be shrunk under it fails before signing in.
	big := filepath.Join(t.TempDir(), "big.png")
	os.WriteFile(big, []byte(strings.Repeat("x", bskyImageMax+1)), 0o644)
	calls = nil
	if _, _, err := postBluesky("did:plc:abc", "app-pass", srv.URL, big, "Big"); err == nil || !strings.Contains(err.Error(), "smaller image") || len(calls) > 0 {
		t.Fatalf("big bluesky image: %v, calls %v", err, calls)
	}
}

// Instagram, TikTok and YouTube can't take an image from here: publish says why before any upload, and so does an
// image type no platform takes.
func TestPublishImageRefused(t *testing.T) {
	config.DataDir = t.TempDir()
	store.Open()
	t.Cleanup(store.Close)
	store.DB.Exec("insert into tenants(id,name) values(1,'Shop')")
	out := filepath.Join(files.ProjectDir(1), "out")
	os.MkdirAll(out, 0o755)
	os.WriteFile(filepath.Join(out, "pic.png"), []byte(pic), 0o644)
	os.WriteFile(filepath.Join(out, "logo.svg"), []byte("<svg/>"), 0o644)
	for i, pl := range []string{"instagram", "tiktok", "youtube", "x"} {
		store.DB.Exec("insert into social_accounts(id,tenant_id,platform,account_id,name,token,refresh) values(?,1,?,?,?,'','')", i+1, pl, pl, pl)
	}
	post := func(account int64, file string) error {
		_, _, err := publish(map[string]any{"account_id": account, "project_id": int64(1), "file": file})
		return err
	}
	for i, pl := range []string{"instagram", "tiktok", "youtube"} {
		ok, why := CanPostImage(pl)
		if err := post(int64(i+1), "out/pic.png"); ok || err == nil || err.Error() != why {
			t.Errorf("%s image: %v", pl, err)
		}
	}
	if ok, _ := CanPostImage("facebook"); !ok {
		t.Error("facebook should take images")
	}
	if err := post(4, "out/logo.svg"); err == nil || !strings.Contains(err.Error(), "JPG, PNG, WebP and GIF") {
		t.Errorf("svg: %v", err)
	}
}

// jpegCopy shrinks to fit maxSide and keeps the shape; Bluesky relies on it for its 1 MB limit.
func TestJpegCopy(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("no ffmpeg")
	}
	png := filepath.Join(t.TempDir(), "wide.png")
	if err := proc.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=red:s=3000x1000", "-frames:v", "1", png).Run(); err != nil {
		t.Fatal(err)
	}
	jpg, done, err := jpegCopy(png, 2000)
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	if w, h := files.VideoSize(jpg); filepath.Ext(jpg) != ".jpg" || w != 2000 || h < 666 || h > 667 {
		t.Fatalf("jpegCopy = %s %dx%d", jpg, w, h)
	}
}
