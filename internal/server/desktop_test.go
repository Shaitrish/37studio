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

// Reveal shows only this app's own files, and only to a browser on this computer.
func TestDesktopReveal(t *testing.T) {
	dir := t.TempDir()
	data := config.DataDir
	config.DataDir = dir
	t.Setenv("DISPLAY", ":0")
	var shown []string
	reveal = func(p string) error { shown = append(shown, p); return nil }
	t.Cleanup(func() { config.DataDir, reveal = data, showInFolder })
	os.MkdirAll(filepath.Join(dir, "projects", "1", "out"), 0o755)
	os.WriteFile(filepath.Join(dir, "projects", "1", "out", "my reel.mp4"), nil, 0o644)
	os.WriteFile(filepath.Join(dir, "keys.db"), nil, 0o644)

	req := func(path string, local bool) *http.Request {
		r := post(`{"path":"` + path + `"}`)
		if local {
			r.RemoteAddr = "127.0.0.1:5000"
		}
		return r
	}
	if _, err := desktopReveal(req("/files/projects/1/out/my%20reel.mp4", false)); err == nil {
		t.Error("revealed for a browser on another computer")
	}
	for _, p := range []string{"/files/projects/../keys.db", "/files/projects/%2e%2e/keys.db", "/api/keys", "/files/projects/1/out/gone.mp4"} {
		if _, err := desktopReveal(req(p, true)); err == nil {
			t.Errorf("%s revealed", p)
		}
	}
	if _, err := desktopReveal(req("/files/projects/1/out/my%20reel.mp4", true)); err != nil {
		t.Fatal(err)
	}
	if len(shown) != 1 || shown[0] != filepath.Join(dir, "projects", "1", "out", "my reel.mp4") {
		t.Fatalf("shown %v", shown)
	}
}
