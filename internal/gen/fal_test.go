package gen

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/store"
)

// A fal video: submit to the queue, poll until done, download the file and bill it at the video price; a first frame
// switches to the image-to-video endpoint and goes inline.
func TestFalVideo(t *testing.T) {
	config.DataDir = t.TempDir()
	store.Open()
	t.Setenv("FAL_KEY", "k")
	var srv *httptest.Server
	var got map[string]any
	var path string
	polls := 0
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v.mp4" && r.Header.Get("Authorization") != "Key k" { // the file itself is a public link
			t.Errorf("auth %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == "POST":
			path = r.URL.Path
			json.NewDecoder(r.Body).Decode(&got)
			json.NewEncoder(w).Encode(map[string]string{"status_url": srv.URL + "/status", "response_url": srv.URL + "/result"})
		case r.URL.Path == "/status":
			if polls++; polls < 2 {
				w.Write([]byte(`{"status":"IN_PROGRESS"}`))
			} else {
				w.Write([]byte(`{"status":"COMPLETED"}`))
			}
		case r.URL.Path == "/result":
			w.Write([]byte(`{"video":{"url":"` + srv.URL + `/v.mp4"}}`))
		case r.URL.Path == "/v.mp4":
			w.Write([]byte("fake mp4"))
		}
	}))
	defer srv.Close()
	falQueue, falPoll = srv.URL, 0
	frame := filepath.Join(t.TempDir(), "f.png")
	os.WriteFile(frame, []byte("\\x89PNG\\r\\n\\x1a\\n"), 0o644)
	out := filepath.Join(t.TempDir(), "clip.mp4")
	if err := falVideo(1, 1, "bytedance/seedance-2.0/fast/text-to-video", "a cat", "9:16", frame, out); err != nil {
		t.Fatal(err)
	}
	if path != "/bytedance/seedance-2.0/fast/image-to-video" || got["aspect_ratio"] != "9:16" || !strings.HasPrefix(got["image_url"].(string), "data:") {
		t.Errorf("submitted %s %v", path, got)
	}
	if b, _ := os.ReadFile(out); string(b) != "fake mp4" {
		t.Errorf("downloaded %q", b)
	}
	var provider string
	store.DB.QueryRow("select provider from usage where project_id=1").Scan(&provider)
	if provider != "fal" {
		t.Errorf("cost recorded under %q", provider)
	}
}
