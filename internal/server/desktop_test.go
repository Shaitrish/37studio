package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Shaitrish/37studio/internal/config"
)

func post(body string) *http.Request {
	return httptest.NewRequest("POST", "/", strings.NewReader(body))
}

// The window's links reach the system browser only as web pages, never as file: or app links.
func TestDesktopOpen(t *testing.T) {
	var opened []string
	Desktop.OpenURL = func(u string) error { opened = append(opened, u); return nil }
	t.Cleanup(func() { Desktop.OpenURL = nil })
	for _, u := range []string{"file:///etc/passwd", "javascript:alert(1)", "smb://host/share", "/relative"} {
		if _, err := desktopOpen(post(`{"url":"` + u + `"}`)); err == nil {
			t.Errorf("%s opened", u)
		}
	}
	if _, err := desktopOpen(post(`{"url":"https://example.com/a"}`)); err != nil || len(opened) != 1 {
		t.Fatalf("https link: err %v, opened %v", err, opened)
	}
}

// Save streams the file under the name the server gives it, and leaves nothing behind when the person cancels.
func TestDesktopSave(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/backup" {
			w.Header().Set("Content-Disposition", `attachment; filename="37studio-backup.zip"`)
		}
		w.Write([]byte("video bytes"))
	}))
	defer srv.Close()
	addr := config.Addr
	config.Addr = strings.TrimPrefix(srv.URL, "http://")
	t.Cleanup(func() { config.Addr, Desktop.SaveAs = addr, nil })

	dir := t.TempDir()
	var asked string
	Desktop.SaveAs = func(name string) (string, error) { asked = name; return filepath.Join(dir, name), nil }

	if _, err := desktopSave(post(`{"path":"/files/../api/keys"}`)); err == nil {
		t.Error(`".." path was fetched`)
	}
	if _, err := desktopSave(post(`{"path":"/api/keys"}`)); err == nil {
		t.Error("a route that isn't a file or backup was fetched")
	}

	if _, err := desktopSave(post(`{"path":"/files/projects/1/out/reel.mp4"}`)); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "reel.mp4")); string(b) != "video bytes" || asked != "reel.mp4" {
		t.Fatalf("saved %q as %q", b, asked)
	}
	if _, err := desktopSave(post(`{"path":"/api/backup?media=1"}`)); err != nil || asked != "37studio-backup.zip" {
		t.Fatalf("backup: err %v, name %q", err, asked)
	}

	Desktop.SaveAs = func(string) (string, error) { return "", nil } // Cancel
	if v, err := desktopSave(post(`{"path":"/files/projects/1/out/other.mp4"}`)); err != nil || v.(map[string]string)["saved"] != "" {
		t.Fatalf("cancel: %v %v", v, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "other.mp4")); err == nil {
		t.Fatal("cancelled save wrote a file")
	}
}
