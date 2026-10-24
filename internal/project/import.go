package project

import (
	"bufio"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/Shaitrish/37studio/internal/agent"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/remote"
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/transcribe"
)

type importJob struct {
	URL   string  `json:"url"`
	Pct   float64 `json:"pct"`
	Line  string  `json:"line"`
	Error string  `json:"error,omitempty"`
	Done  bool    `json:"done"`
	cmd   *exec.Cmd
}

var importMu sync.Mutex

var importJobs = map[int64]*importJob{}

var dlProgress = regexp.MustCompile(`^\[download\]\s+([\d.]+)%`)

var ytTag = regexp.MustCompile(`^(\[[^\]]+\]\s*)+`) // "[youtube] [jsc:node] Solving…" reads better without the tags

// importState is the project's latest import, for the project page; nil when there is none.
func importState(id int64) map[string]any {
	importMu.Lock()
	defer importMu.Unlock()
	j := importJobs[id]
	if j == nil {
		return nil
	}
	return map[string]any{"url": j.URL, "pct": j.Pct, "line": j.Line, "error": j.Error, "done": j.Done}
}

// ImportVideo is POST /api/projects/{id}/import {"url": "...", "start": true}. With start, the agent starts on the
// project once the video is in (a new project's first run).
func ImportVideo(r *http.Request) (any, error) {
	id := httpapi.PathID(r)
	b, err := httpapi.Body(r)
	if err != nil {
		return nil, err
	}
	p, err := store.Row("select * from projects where id=?", id)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(strings.TrimSpace(store.Str(b["url"])))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, httpapi.Bad("paste a YouTube or Google Drive link")
	}
	if err := remote.Public(u.Hostname()); err != nil {
		return nil, httpapi.Bad(err.Error())
	}
	importMu.Lock()
	busy := importJobs[id] != nil && !importJobs[id].Done
	importMu.Unlock()
	if busy {
		return nil, httpapi.Bad("a video is already downloading into this project")
	}
	py, err := transcribe.Python()
	if err != nil {
		return nil, err
	}
	dir := files.ProjectDir(id)
	tmp := filepath.Join(dir, "work", "download")
	lang := agent.ProjectLanguage(p)
	cmd := exec.Command(py, "-m", "yt_dlp", "--no-playlist", "--newline", "--no-colors", "--windows-filenames",
		"--js-runtimes", "node", // YouTube's player challenges need a JavaScript runtime; the image has Node
		"-f", "bv*[height<=1080]+ba/b[height<=1080]/b", "--merge-output-format", "mp4",
		"--write-subs", "--write-auto-subs", "--sub-langs", lang+","+lang+"-orig", "--convert-subs", "srt",
		"-P", filepath.Join(dir, "input"), "-P", "temp:"+tmp, "-o", "%(title).80B.%(ext)s", u.String())
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	j := &importJob{URL: u.String(), Line: "Starting the download…", cmd: cmd}
	importMu.Lock()
	importJobs[id] = j
	importMu.Unlock()

	var lastErr string
	read := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "ERROR:") {
				lastErr = strings.TrimSpace(strings.TrimPrefix(line, "ERROR:"))
			}
			importMu.Lock()
			if m := dlProgress.FindStringSubmatch(line); m != nil {
				j.Pct, _ = strconv.ParseFloat(m[1], 64)
			}
			j.Line = ytTag.ReplaceAllString(line, "")
			importMu.Unlock()
		}
		io.Copy(io.Discard, pr)
		close(read)
	}()
	start := b["start"] == true
	go func() {
		err := cmd.Wait()
		pw.Close()
		<-read
		os.RemoveAll(tmp)
		importMu.Lock()
		j.Done = true
		if err != nil && j.Error == "" {
			j.Error = importHint(lastErr)
		}
		failed := j.Error
		importMu.Unlock()
		if failed != "" {
			agent.AddEvent(id, "error", map[string]string{"text": "Import failed: " + failed})
			return
		}
		agent.AddEvent(id, "info", map[string]string{"text": "Imported the video from " + u.Host + " into input/."})
		if start && !agent.IsRunning(id) {
			agent.StartRun(id, "", "", true)
		}
	}()
	return importState(id), nil
}

// importHint turns yt-dlp's last error into what the editor can do about it.
func importHint(e string) string {
	switch {
	case strings.Contains(e, "Sign in to confirm") || strings.Contains(e, "not a bot"):
		return "YouTube asked to confirm this isn't a bot. Try again in a while, or download the video yourself and upload the file."
	case strings.Contains(e, "Private video") || strings.Contains(e, "members-only") || strings.Contains(e, "This video is private"):
		return "That video is private. Make it unlisted, or upload the file."
	case strings.Contains(strings.ToLower(e), "drive") || strings.Contains(e, "403"):
		return "Couldn't open that file. On Google Drive, share it as \"Anyone with the link\"; or upload the file. (" + e + ")"
	case e == "":
		return "The download stopped."
	}
	return e
}

// CancelImport is DELETE /api/projects/{id}/import.
func CancelImport(r *http.Request) (any, error) {
	id := httpapi.PathID(r)
	importMu.Lock()
	defer importMu.Unlock()
	if j := importJobs[id]; j != nil && !j.Done && j.cmd.Process != nil {
		j.Error = "Cancelled."
		syscall.Kill(-j.cmd.Process.Pid, syscall.SIGTERM)
	}
	return nil, nil
}
