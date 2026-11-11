package server

import (
	"cmp"
	"encoding/json"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/httpapi"
)

// Desktop is filled in by the desktop app. Its window opens no tabs and saves no downloads by itself, so the UI asks
// the app instead: links go to the system browser, downloads to a Save dialog. Both are nil in Docker.
var Desktop struct {
	OpenURL func(url string) error
	// SaveAs asks where to save a file called name; "" means the person cancelled.
	SaveAs func(name string) (string, error)
}

// desktopOpen is POST /api/desktop/open {url}. No MCP tool: it acts on the person's own screen, which an agent has no use for.
func desktopOpen(r *http.Request) (any, error) {
	if Desktop.OpenURL == nil {
		return nil, httpapi.Bad("opening links this way only works in the desktop app")
	}
	var b struct{ URL string }
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		return nil, err
	}
	// Only web pages: the OS would hand any other scheme (file:, smb:, custom app links) to whatever is registered for it.
	if u, err := url.Parse(b.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, httpapi.Bad("only http and https links can be opened")
	}
	return nil, Desktop.OpenURL(b.URL)
}

// desktopSave is POST /api/desktop/save {path, name}: it fetches path from this server, the same request the
// browser's download would make, asks where to keep it and streams it there, so a backup with every video never sits
// in memory. ponytail: the request lasts as long as the copy; a big backup could report progress as background work.
func desktopSave(r *http.Request) (any, error) {
	if Desktop.SaveAs == nil {
		return nil, httpapi.Bad("saving files this way only works in the desktop app")
	}
	var b struct{ Path, Name string }
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		return nil, err
	}
	// ".." would let /files/../ reach any GET route once the router cleans the path.
	if (!strings.HasPrefix(b.Path, "/files/") && !strings.HasPrefix(b.Path, "/api/backup")) || strings.Contains(b.Path, "..") {
		return nil, httpapi.Bad("only this app's files and backups can be saved")
	}
	req, err := http.NewRequestWithContext(r.Context(), "GET", "http://"+config.Addr+b.Path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", r.Header.Get("Authorization")) // STUDIO37_PASSWORD, if set
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 500))
		return nil, httpapi.Bad("couldn't get the file: " + strings.TrimSpace(string(msg)))
	}
	_, params, _ := mime.ParseMediaType(res.Header.Get("Content-Disposition"))
	name := path.Base(cmp.Or(b.Name, params["filename"], strings.SplitN(b.Path, "?", 2)[0]))
	dest, err := Desktop.SaveAs(name)
	if err != nil || dest == "" {
		return map[string]string{"saved": ""}, err
	}
	f, err := os.Create(dest)
	if err != nil {
		return nil, err
	}
	if _, err = io.Copy(f, res.Body); err != nil {
		f.Close()
		os.Remove(dest) // a half-written video or backup would look fine until someone needs it
		return nil, err
	}
	return map[string]string{"saved": dest}, f.Close()
}

// canReveal says whether a file can be shown in this computer's file manager for the person making r. Only when
// 37Studio runs on the computer they're looking at: the desktop app, or the server started on it. Not in Docker or
// on a headless server, which have no screen, and not for a browser on another machine, which would see nothing
// while a window opened on the server's screen.
func canReveal(r *http.Request) bool {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return false
	}
	return runtime.GOOS != "linux" || os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

// reveal is showInFolder, swapped out by the test so it doesn't open windows.
var reveal = showInFolder

// desktopReveal is POST /api/desktop/reveal {path}: it opens the folder holding one of this app's files (path is its
// /files/ URL) in the file manager, with the file selected. No MCP tool, like open and save.
func desktopReveal(r *http.Request) (any, error) {
	if !canReveal(r) {
		return nil, httpapi.Bad("showing a file in its folder only works when 37Studio runs on this computer, not in Docker or on a server: download it instead")
	}
	var b struct{ Path string }
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		return nil, err
	}
	rel, ok := strings.CutPrefix(b.Path, "/files/")
	rel, err := url.PathUnescape(rel)
	rel = path.Clean("/" + rel) // checked after cleaning, so projects/../keys can't step out
	if !ok || err != nil || !(strings.HasPrefix(rel, "/projects/") || strings.HasPrefix(rel, "/tenants/") || strings.HasPrefix(rel, "/templates/")) {
		return nil, httpapi.Bad("only this app's files can be shown")
	}
	p, err := filepath.Abs(files.Within(config.DataDir, rel))
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(p); err != nil {
		return nil, httpapi.Bad("that file isn't there any more")
	}
	return nil, reveal(p)
}
