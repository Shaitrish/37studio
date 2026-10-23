package main

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
)

// Import a video from a link (YouTube, Google Drive, or anything else yt-dlp reads) into a project's input/. YouTube's
// subtitles in the video's language come along, and 37studio transcribe uses them instead of the slow transcription.
// The download runs here, on the user's computer, from their own connection.

type importJob struct {
	URL   string  `json:"url"`
	Pct   float64 `json:"pct"`
	Line  string  `json:"line"`
	Error string  `json:"error,omitempty"`
	Done  bool    `json:"done"`
	cmd   *exec.Cmd
}

var (
	importMu   sync.Mutex
	importJobs = map[int64]*importJob{}
	dlProgress = regexp.MustCompile(`^\[download\]\s+([\d.]+)%`)
	ytTag      = regexp.MustCompile(`^(\[[^\]]+\]\s*)+`) // "[youtube] [jsc:node] Solving…" reads better without the tags
)

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

// importVideo is POST /api/projects/{id}/import {"url": "...", "start": true}. With start, the agent starts on the
// project once the video is in (a new project's first run).
func importVideo(r *http.Request) (any, error) {
	id := pathID(r)
	b, err := body(r)
	if err != nil {
		return nil, err
	}
	p, err := row("select * from projects where id=?", id)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(strings.TrimSpace(str(b["url"])))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, bad("paste a YouTube or Google Drive link")
	}
	importMu.Lock()
	busy := importJobs[id] != nil && !importJobs[id].Done
	importMu.Unlock()
	if busy {
		return nil, bad("a video is already downloading into this project")
	}
	py, err := asrPython()
	if err != nil {
		return nil, err
	}
	dir := projectDir(id)
	tmp := filepath.Join(dir, "work", "download")
	lang := projectLanguage(p)
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
			addEvent(id, "error", map[string]string{"text": "Import failed: " + failed})
			return
		}
		addEvent(id, "info", map[string]string{"text": "Imported the video from " + u.Host + " into input/."})
		if start && !isRunning(id) {
			startRun(id, "", "", true)
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

// cancelImport is DELETE /api/projects/{id}/import.
func cancelImport(r *http.Request) (any, error) {
	id := pathID(r)
	importMu.Lock()
	defer importMu.Unlock()
	if j := importJobs[id]; j != nil && !j.Done && j.cmd.Process != nil {
		j.Error = "Cancelled."
		syscall.Kill(-j.cmd.Process.Pid, syscall.SIGTERM)
	}
	return nil, nil
}
