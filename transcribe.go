package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func transcribeCLI(args []string) int {
	out := ""
	for i, a := range args {
		if a == "--out" && i+1 < len(args) {
			out = args[i+1]
		}
	}
	if out == "" {
		fmt.Fprintln(os.Stderr, "usage: 37studio transcribe FILE --out DIR [--language bn] [--quality auto|fast|best] [--model NAME]")
		return 2
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "transcribe:", err)
		return 1
	}
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

// asrPython returns a Python that can import faster_whisper.
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
	if exec.Command(py, "-c", "import faster_whisper").Run() == nil {
		return py, nil
	}
	fmt.Fprintln(os.Stderr, "Setting up local transcription, once (about 150 MB)…")
	for _, c := range [][]string{{"python3", "-m", "venv", venv}, {filepath.Join(venv, "bin", "pip"), "install", "--quiet", fasterWhisper}} {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("%v failed: %w (needs python3 with venv)", c, err)
		}
	}
	return py, nil
}
