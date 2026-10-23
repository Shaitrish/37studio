package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// `37studio transcribe`: free, local speech-to-text for the agent (asr/transcribe.py, faster-whisper). The Docker image
// ships the Python environment (STUDIO37_ASR_PYTHON); a copy run outside Docker sets one up in the user cache once.
//
// A 2-hour class takes about 2 hours on a CPU, longer than one Bash call may run and than one agent turn should wait.
// So the work runs detached, and the command waits up to 9 minutes for it. Run it again to keep waiting: a live run
// is joined, not restarted, and a finished transcript is reused.

//go:embed asr/transcribe.py
var asrScript []byte

const fasterWhisper = "faster-whisper==1.2.1"

// The editor can switch transcription off for a project, or cancel a run, from the project page: a "skip" file in
// the output folder, which the command honours until it is run with --force.
const skipNote = "The editor switched transcription off for this project, or cancelled it. Don't run it again: work from the footage and the brief, or ask the editor for a subtitle file (.srt). Use --force only if the editor asks you to transcribe."

func transcribeCLI(args []string) int {
	var media, out, subs, lang string
	var force bool
	var pass []string // what the script gets
	for i := 0; i < len(args); i++ {
		a, v := args[i], ""
		if strings.HasPrefix(a, "--") && a != "--force" && i+1 < len(args) {
			i++
			v = args[i]
		}
		switch a {
		case "--force":
			force = true
			continue
		case "--subs":
			subs = v
			continue
		case "--out":
			out = v
		case "--language":
			lang = v
		default:
			if !strings.HasPrefix(a, "--") && media == "" {
				media = a
			}
		}
		pass = append(pass, a)
		if v != "" {
			pass = append(pass, v)
		}
	}
	if out == "" || media == "" {
		fmt.Fprintln(os.Stderr, "usage: 37studio transcribe FILE --out DIR [--language bn] [--quality auto|fast|best] [--model NAME] [--subs FILE.srt] [--force]")
		return 2
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "transcribe:", err)
		return 1
	}
	skipFile := filepath.Join(out, "skip")
	if _, err := os.Stat(skipFile); err == nil && !force {
		fmt.Println(skipNote)
		return 0
	}
	os.Remove(skipFile)
	if subs == "" {
		subs = subtitlesFor(media, lang)
	}
	if subs != "" {
		return fromSubtitles(subs, lang, out)
	}
	args = pass
	logPath, pidPath := filepath.Join(out, "log.txt"), filepath.Join(out, "run.pid")

	done := make(chan error, 1)
	if pid, _ := strconv.Atoi(strings.TrimSpace(readFile(pidPath))); transcribing(pid) {
		fmt.Println("Already transcribing into", out+"; waiting for it.")
		go func() {
			for transcribing(pid) {
				time.Sleep(2 * time.Second)
			}
			done <- nil
		}()
	} else {
		py, err := asrPython()
		if err != nil {
			fmt.Fprintln(os.Stderr, "transcribe:", err)
			return 1
		}
		script := filepath.Join(os.TempDir(), "37studio-transcribe.py")
		logf, err := os.Create(logPath)
		if err == nil {
			err = os.WriteFile(script, asrScript, 0o644)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "transcribe:", err)
			return 1
		}
		cmd := exec.Command(py, append([]string{script}, args...)...)
		cmd.Stdout, cmd.Stderr = logf, logf
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // outlives this command, the agent's turn and its Stop button
		if err := cmd.Start(); err != nil {
			fmt.Fprintln(os.Stderr, "transcribe:", err)
			return 1
		}
		os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644)
		go func() { done <- cmd.Wait() }()
	}

	wait, _ := strconv.Atoi(env("STUDIO37_TRANSCRIBE_WAIT", "540"))
	select {
	case err := <-done:
		if _, serr := os.Stat(skipFile); serr == nil { // cancelled from the project page while it ran
			fmt.Println(skipNote)
			return 0
		}
		fmt.Print(lastLines(readFile(logPath), 12))
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		return 0
	case <-time.After(time.Duration(wait) * time.Second):
		fmt.Print(lastLines(readFile(logPath), 3))
		fmt.Println("Still transcribing in the background; it keeps going after this command and after your turn ends. Run the same command again to keep waiting.")
		return 0
	}
}

// transcribing reports whether pid is a live run of the script (not a zombie, not a reused pid).
func transcribing(pid int) bool {
	if pid <= 0 || syscall.Kill(pid, 0) != nil {
		return false
	}
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	return err != nil || bytes.Contains(cmdline, []byte("37studio-transcribe")) // no /proc outside Linux: trust kill
}

func readFile(path string) string {
	b, _ := os.ReadFile(path)
	return string(b)
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return strings.Join(lines[max(0, len(lines)-n):], "\n") + "\n"
}

// asrPython returns a Python that can import faster_whisper and yt_dlp (link imports, import.go).
func asrPython() (string, error) {
	if p := env("STUDIO37_ASR_PYTHON", ""); p != "" {
		return p, nil
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	venv := filepath.Join(cache, "37studio", "asr")
	py := filepath.Join(venv, "bin", "python")
	if exec.Command(py, "-c", "import faster_whisper, yt_dlp").Run() == nil {
		return py, nil
	}
	fmt.Fprintln(os.Stderr, "Setting up local transcription and downloads, once (about 200 MB)…")
	for _, c := range [][]string{{"python3", "-m", "venv", venv}, {filepath.Join(venv, "bin", "pip"), "install", "--quiet", fasterWhisper, "yt-dlp[default,curl-cffi]"}} {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("%v failed: %w (needs python3 with venv)", c, err)
		}
	}
	return py, nil
}

// ---- subtitles instead of transcribing: YouTube's from an import ("Title.bn.srt"), or a file the editor added

// subtitlesFor picks the subtitles next to media: same name first (the video's language if there are several),
// otherwise the only subtitle file, when it sits beside the only video.
func subtitlesFor(media, lang string) string {
	dir, stem := filepath.Dir(media), strings.TrimSuffix(filepath.Base(media), filepath.Ext(media))
	var all, same []string
	videos := 0
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if k := kindOf(e.Name()); k == "video" || k == "audio" {
			videos++
		}
		if e.IsDir() || (ext != ".srt" && ext != ".vtt") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		all = append(all, p)
		if strings.HasPrefix(e.Name(), stem+".") {
			if lang != "" && strings.Contains(e.Name(), "."+lang) {
				return p
			}
			same = append(same, p)
		}
	}
	if len(same) > 0 {
		return same[0]
	}
	if len(all) == 1 && videos == 1 {
		return all[0]
	}
	return ""
}

type cue struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
	Words []any   `json:"words"` // subtitles carry no word timings
}

var (
	cueTime   = regexp.MustCompile(`(?:(\d+):)?(\d{1,2}):(\d{2})[.,](\d{3})\s*-->\s*(?:(\d+):)?(\d{1,2}):(\d{2})[.,](\d{3})`)
	cueTag    = regexp.MustCompile(`<[^>]*>`)
	cueBreaks = regexp.MustCompile(`\r?\n[ \t]*\r?\n`)
)

// parseSubs reads SRT or WebVTT. YouTube's automatic captions repeat each line in the next cue as they roll, so a line
// equal to the previous one is dropped.
func parseSubs(s string) []cue {
	var out []cue
	prev := ""
	secs := func(m []string) float64 {
		h, _ := strconv.Atoi(m[0])
		mi, _ := strconv.Atoi(m[1])
		se, _ := strconv.Atoi(m[2])
		ms, _ := strconv.Atoi(m[3])
		return float64(h*3600+mi*60+se) + float64(ms)/1000
	}
	for _, block := range cueBreaks.Split(s, -1) {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		for i, l := range lines {
			m := cueTime.FindStringSubmatch(l)
			if m == nil {
				continue
			}
			var text []string
			for _, t := range lines[i+1:] {
				t = strings.TrimSpace(html.UnescapeString(cueTag.ReplaceAllString(t, "")))
				if t != "" && t != prev {
					text = append(text, t)
					prev = t
				}
			}
			if len(text) > 0 {
				out = append(out, cue{Start: secs(m[1:5]), End: secs(m[5:9]), Text: strings.Join(text, " "), Words: []any{}})
			}
			break
		}
	}
	return out
}

// fromSubtitles writes the same files a transcription would, from a subtitle file, in a moment.
func fromSubtitles(path, lang, out string) int {
	raw, err := os.ReadFile(path)
	segs := parseSubs(string(raw))
	if err != nil || len(segs) == 0 {
		fmt.Fprintln(os.Stderr, "transcribe: no subtitles found in", path)
		return 1
	}
	model := "subtitles: " + filepath.Base(path)
	stamp := func(s float64) string {
		ms := int(s*1000 + 0.5)
		return fmt.Sprintf("%02d:%02d:%02d,%03d", ms/3600000, ms%3600000/60000, ms%60000/1000, ms%1000)
	}
	var srt, txt strings.Builder
	for i, c := range segs {
		fmt.Fprintf(&srt, "%d\n%s --> %s\n%s\n\n", i+1, stamp(c.Start), stamp(c.End), c.Text)
		sec := int(c.Start)
		fmt.Fprintf(&txt, "[%d:%02d:%02d] %s\n", sec/3600, sec%3600/60, sec%60, c.Text) // as transcribe.py writes it
	}
	js, _ := json.MarshalIndent(map[string]any{"language": lang, "duration": segs[len(segs)-1].End, "model": model, "segments": segs}, "", " ")
	summary := map[string]any{"ok": true, "language": lang, "segments": len(segs), "model": model, "word_timings": false,
		"json": filepath.Join(out, "transcript.json"), "srt": filepath.Join(out, "transcript.srt"), "txt": filepath.Join(out, "transcript.txt")}
	meta, _ := json.Marshal(map[string]any{"key": "subtitles", "summary": summary})
	for name, b := range map[string][]byte{"transcript.json": js, "transcript.srt": []byte(srt.String()), "transcript.txt": []byte(txt.String()), "meta.json": meta} {
		if err := os.WriteFile(filepath.Join(out, name), b, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "transcribe:", err)
			return 1
		}
	}
	fmt.Printf("Used the subtitles in %s instead of transcribing: segment timings only, no word timings.\n", path)
	line, _ := json.Marshal(summary)
	fmt.Println(string(line))
	return 0
}

// ---- the project page: progress, and switching it off

var progressLine = regexp.MustCompile(`(?m)^(\d+)% · (.*)$`)

// transcriptState is work/transcript as the project page shows it; nil when nothing has happened yet.
func transcriptState(id int64) map[string]any {
	dir := filepath.Join(projectDir(id), "work", "transcript")
	log := readFile(filepath.Join(dir, "log.txt"))
	pid, _ := strconv.Atoi(strings.TrimSpace(readFile(filepath.Join(dir, "run.pid"))))
	has := func(name string) bool { _, err := os.Stat(filepath.Join(dir, name)); return err == nil }
	switch {
	case transcribing(pid):
		st := map[string]any{"state": "running", "pct": 0, "line": "Getting ready. The first time, it downloads the speech model (about 1.6 GB)."}
		if m := progressLine.FindAllStringSubmatch(log, -1); len(m) > 0 {
			st["pct"], _ = strconv.Atoi(m[len(m)-1][1])
			st["line"] = m[len(m)-1][2]
		}
		return st
	case has("transcript.json"):
		var meta struct {
			Summary struct {
				Model string `json:"model"`
			} `json:"summary"`
		}
		json.Unmarshal([]byte(readFile(filepath.Join(dir, "meta.json"))), &meta)
		return map[string]any{"state": "done", "from": meta.Summary.Model}
	case has("skip"):
		return map[string]any{"state": "skipped"}
	case strings.Contains(log, "Traceback"):
		return map[string]any{"state": "failed", "line": strings.TrimSpace(lastLines(log, 1))}
	}
	return nil
}

// transcriptAction is POST /api/projects/{id}/transcript {"action": "skip"|"resume"}: skip switches transcription off
// for the project and stops a run in progress; resume switches it back on.
func transcriptAction(r *http.Request) (any, error) {
	b, err := body(r)
	if err != nil {
		return nil, err
	}
	id := pathID(r)
	dir := filepath.Join(projectDir(id), "work", "transcript")
	os.MkdirAll(dir, 0o755)
	switch str(b["action"]) {
	case "skip":
		if err := os.WriteFile(filepath.Join(dir, "skip"), nil, 0o644); err != nil {
			return nil, err
		}
		if pid, _ := strconv.Atoi(strings.TrimSpace(readFile(filepath.Join(dir, "run.pid")))); transcribing(pid) {
			syscall.Kill(-pid, syscall.SIGTERM) // the script leads its own session (Setsid), so this stops all of it
		}
	case "resume":
		os.Remove(filepath.Join(dir, "skip"))
	default:
		return nil, bad("unknown action")
	}
	return transcriptState(id), nil
}
