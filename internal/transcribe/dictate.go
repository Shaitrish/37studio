package transcribe

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/proc"
)

//go:embed asr/dictate.py
var dictateScript []byte

// maxClip caps one phrase: 16 kHz 16-bit mono is 32 KB a second, so this is about four minutes.
const maxClip = 8 << 20

// idleFor is how long the worker keeps the models in memory (about 600 MB) after the last phrase.
const idleFor = 10 * time.Minute

// Dictate is POST /api/dictate, behind the chat's microphone wherever the window has no speech recognition of its
// own (the desktop app's WebKitGTK and WebView2, Firefox). The body is one phrase as 16 kHz mono 16-bit
// little-endian PCM and comes back as {"text": …}, always English, transcribed on this computer. ?final=1 is a
// finished phrase and waits its turn; anything else is a draft shown while the person is still talking, skipped with
// {"busy": true} when the worker is on something else so drafts never queue up behind each other. An empty body only
// starts the worker, so the models load while the person speaks.
//
// The page sends raw PCM because WebKitGTK has no MediaRecorder ("unsupported on this platform"), which is what broke
// the old recorded-clip dictation in the Linux app; the Web Audio API works in every window.
func Dictate(r *http.Request) (any, error) {
	final := r.URL.Query().Get("final") == "1"
	pcm, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, maxClip))
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return nil, httpapi.Bad("That phrase is too long for dictation. Pause now and then, or keep it to a few minutes.")
		}
		return nil, err
	}
	if len(pcm)%2 != 0 {
		return nil, httpapi.Bad("Dictation expects 16-bit audio samples; this upload has an odd number of bytes.")
	}
	w, err := dictation.get()
	if err != nil {
		return nil, err
	}
	if len(pcm) == 0 {
		return map[string]bool{"ok": true}, nil
	}
	if final {
		w.mu.Lock()
	} else if !w.mu.TryLock() {
		return map[string]bool{"busy": true}, nil
	}
	defer w.mu.Unlock()
	text, err := w.ask(pcm, final)
	if err != nil {
		dictation.drop(w)
		return nil, err
	}
	return map[string]string{"text": text}, nil
}

// dictation is the one speech worker, started on the first phrase and stopped after idleFor without one.
var dictation pool

type pool struct {
	sync.Mutex
	w *worker
}

type worker struct {
	mu    sync.Mutex // one phrase at a time
	cmd   *exec.Cmd
	dir   string
	in    io.WriteCloser
	out   *bufio.Reader
	log   *tail
	ready chan struct{} // closed once the models are loaded, or loading failed (err)
	err   error
	idle  *time.Timer
}

func (p *pool) get() (*worker, error) {
	p.Lock()
	defer p.Unlock()
	if p.w != nil {
		p.w.idle.Reset(idleFor)
		return p.w, nil
	}
	py, err := Python() // the first time, this sets up the venv (about 200 MB)
	if err != nil {
		return nil, httpapi.Bad("Dictation turns speech into text on this computer with Python 3 and faster-whisper, and that isn't set up: " + err.Error() + ". Install Python 3 (on Linux, python3 and python3-venv), then try again.")
	}
	w, err := startWorker(py)
	if err != nil {
		return nil, err
	}
	w.idle = time.AfterFunc(idleFor, func() { p.drop(w) })
	p.w = w
	return w, nil
}

// drop stops w once its phrase is done, and forgets it so the next phrase starts a fresh one.
func (p *pool) drop(w *worker) {
	p.Lock()
	if p.w == w {
		p.w = nil
	}
	p.Unlock()
	go func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		w.idle.Stop()
		w.in.Close() // the script exits at the end of its input
		done := make(chan struct{})
		go func() { w.cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			proc.Kill(w.cmd)
			<-done
		}
		proc.Done(w.cmd)
		os.RemoveAll(w.dir)
	}()
}

func startWorker(py string) (*worker, error) {
	dir, err := os.MkdirTemp("", "37studio-dictate-")
	if err != nil {
		return nil, err
	}
	script := filepath.Join(dir, "dictate.py")
	if err := os.WriteFile(script, dictateScript, 0o644); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	w := &worker{dir: dir, log: &tail{}, ready: make(chan struct{})}
	w.cmd = proc.Command(py, "-I", script)
	w.cmd.Stderr = w.log
	if w.in, err = w.cmd.StdinPipe(); err == nil {
		var out io.ReadCloser
		if out, err = w.cmd.StdoutPipe(); err == nil {
			w.out = bufio.NewReader(out)
			err = proc.Start(w.cmd)
		}
	}
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	go func() {
		var msg struct {
			Ready bool   `json:"ready"`
			Error string `json:"error"`
		}
		line, err := w.out.ReadBytes('\n')
		switch {
		case err != nil:
			w.err = w.failed(err)
		case json.Unmarshal(line, &msg) != nil || !msg.Ready:
			w.err = errors.New("Dictation couldn't start: " + strings.TrimSpace(msg.Error+" "+lastLines(w.log.String(), 1)))
		}
		close(w.ready)
	}()
	return w, nil
}

// ask sends one phrase and waits for its text. The caller holds w.mu.
func (w *worker) ask(pcm []byte, final bool) (string, error) {
	<-w.ready // the first phrase waits for the models; the page started the worker when the mic opened
	if w.err != nil {
		return "", w.err
	}
	head, _ := json.Marshal(map[string]any{"n": len(pcm), "final": final})
	if _, err := w.in.Write(append(head, '\n')); err != nil {
		return "", w.failed(err)
	}
	if _, err := w.in.Write(pcm); err != nil {
		return "", w.failed(err)
	}
	line, err := w.out.ReadBytes('\n')
	if err != nil {
		return "", w.failed(err)
	}
	var res struct {
		Text  string `json:"text"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(line, &res); err != nil {
		return "", fmt.Errorf("Dictation sent back something unexpected: %.200s", line)
	}
	if res.Error != "" {
		return "", errors.New("Couldn't turn what you said into text: " + res.Error)
	}
	return res.Text, nil
}

func (w *worker) failed(err error) error {
	why := strings.TrimSpace(lastLines(w.log.String(), 1))
	if why == "" {
		why = err.Error()
	}
	return errors.New("Dictation stopped working: " + why + ". Try the microphone again.")
}

// tail keeps the end of the worker's stderr for error messages; exec writes it from its own goroutine.
type tail struct {
	mu sync.Mutex
	b  []byte
}

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.b = append(t.b, p...)
	if len(t.b) > 4096 {
		t.b = t.b[len(t.b)-4096:]
	}
	return len(p), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.b)
}
