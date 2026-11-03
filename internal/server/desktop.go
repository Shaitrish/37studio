package server

import (
	"cmp"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/Shaitrish/37studio/internal/config"
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
