package agent

import (
	"bufio"
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/textutil"
)

// Codex is OpenAI's coding agent. Signed in with a ChatGPT plan (Plus, Pro, Business…), its runs come out of that
// plan, the way a Claude subscription token works for Claude Code: no API key. Its login lives in the data folder
// (data/codex), so it survives restarts and updates and is backed up with everything else.
var codexQualities = []Quality{
	{"codex", "Codex", "codex", "medium", "Codex with your ChatGPT plan: OpenAI's coding agent and its default model.", "codex", "Codex (ChatGPT plan)"},
	{"codex-high", "Codex, high effort", "codex", "high", "Codex thinking longer, for harder edits. Uses more of your plan's limits.", "codex", "Codex (ChatGPT plan)"},
}

func codexHome() string {
	h := cmp.Or(os.Getenv("CODEX_HOME"), filepath.Join(config.DataDir, "codex"))
	os.MkdirAll(h, 0o700) // Codex refuses a CODEX_HOME that doesn't exist
	return h
}

// CodexReady is whether Codex is installed and signed in.
func CodexReady() bool {
	if _, err := exec.LookPath("codex"); err != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(codexHome(), "auth.json"))
	return err == nil
}

func codexCmd(args ...string) *exec.Cmd {
	cmd := exec.Command("codex", args...)
	cmd.Env = Env("CODEX_HOME=" + codexHome())
	return cmd
}

func startCodex(id int64, p map[string]any, q Quality, text string, announce bool) error {
	if !CodexReady() {
		return httpapi.Bad("Codex isn't signed in yet. Settings → AI services → Codex → Sign in with ChatGPT.")
	}
	sid := store.Str(p["cx_session"])
	prompt := recap(id, lastTurn(id, "codex")) + text
	if sid == "" {
		prompt = strings.TrimSpace(FirstPrompt(p) + joinRecap(recap(id, -1)) + text)
	} else if text == "" {
		return httpapi.Bad("message is empty")
	}
	dir := files.ProjectDir(id)
	agents := filepath.Join(dir, "AGENTS.md") // Codex reads the harness from AGENTS.md, like OpenCode
	if err := os.WriteFile(agents, []byte(harnessPrompt(p)), 0o644); err != nil {
		return err
	}
	args := []string{"exec", "--json", "--skip-git-repo-check", "-c", "model_reasoning_effort=" + q.Effort, "--add-dir", files.TenantRoot(p["tenant_id"].(int64))}
	if config.Env("STUDIO37_AGENT_PERMISSIONS", "") == "bypass" {
		args = append(args, "--dangerously-bypass-approvals-and-sandbox") // the container is the sandbox, as for Claude Code
	} else {
		args = append(args, "-s", "workspace-write")
	}
	if sid != "" {
		args = append(args, "resume", sid)
	}
	exe, _ := os.Executable()
	cmd := codexCmd(append(args, "-")...) // the prompt goes over stdin: no argv length limit
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Env = append(cmd.Env, fmt.Sprintf("STUDIO37_PROJECT=%d", id), "STUDIO37_DATA="+config.DataDir,
		"PATH="+filepath.Dir(exe)+string(os.PathListSeparator)+os.Getenv("PATH"))
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if IsRunning(id) {
		return httpapi.Bad("something is already running on this project")
	}
	if announce {
		AddEvent(id, "user", map[string]string{"text": cmp.Or(text, "Start the job.")})
	}
	setStatus(id, "running")
	parsed := make(chan struct{})
	err := Launch(id, cmd, func(err error) {
		pw.Close()
		<-parsed
		os.Remove(agents) // Claude Code reads AGENTS.md too; don't hand it a second copy of the harness
		if err != nil && cmd.ProcessState != nil && !cmd.ProcessState.Success() && stderr.Len() > 0 {
			AddEvent(id, "error", map[string]string{"text": textutil.Tail(textutil.ANSI.ReplaceAllString(stderr.String(), ""), 2000)})
		}
		setStatus(id, "review")
		drainQueue(id)
	})
	if err != nil {
		os.Remove(agents)
		setStatus(id, store.Str(p["status"]))
		return err
	}
	go func() {
		if s := parseCodex(id, p["tenant_id"].(int64), q, pr); s != "" && sid == "" {
			store.DB.Exec("update projects set cx_session=? where id=?", s, id)
		}
		io.Copy(io.Discard, pr)
		close(parsed)
	}()
	return nil
}

// parseCodex maps `codex exec --json` events (codex-rs/exec/src/exec_events.rs) onto the same chat events as the
// other agents. A plan login has no per-run price, so the usage row records tokens at no cost.
func parseCodex(pid, tenant int64, q Quality, r io.Reader) (thread string) {
	start := time.Now()
	var tokens float64
	turns := 0
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var ev struct {
			Type     string `json:"type"`
			ThreadID string `json:"thread_id"`
			Item     struct {
				Type, Text, Command, Server, Tool, Query, Status, Message string
				Output                                                    string `json:"aggregated_output"`
				Changes                                                   []struct{ Path string }
			} `json:"item"`
			Usage struct {
				Input  float64 `json:"input_tokens"`
				Output float64 `json:"output_tokens"`
			} `json:"usage"`
			Error struct{ Message string } `json:"error"`
		}
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		it := ev.Item
		switch ev.Type {
		case "thread.started":
			thread = ev.ThreadID
		case "turn.completed":
			tokens += ev.Usage.Input + ev.Usage.Output
			turns++
		case "turn.failed": // "Reconnecting…" errors before it are retries, not news
			AddEvent(pid, "error", map[string]string{"text": codexHint(ev.Error.Message)})
		case "item.completed":
			switch it.Type {
			case "agent_message":
				if strings.TrimSpace(it.Text) != "" {
					AddEvent(pid, "text", map[string]string{"text": it.Text})
				}
			case "command_execution":
				AddEvent(pid, "tool", map[string]string{"name": "Bash", "summary": it.Command})
				if it.Status == "failed" {
					AddEvent(pid, "tool_error", map[string]string{"text": textutil.Tail(it.Output, 600)})
				}
			case "file_change":
				var paths []string
				for _, c := range it.Changes {
					paths = append(paths, c.Path)
				}
				AddEvent(pid, "tool", map[string]string{"name": "Edit", "summary": strings.Join(paths, ", ")})
			case "mcp_tool_call":
				AddEvent(pid, "tool", map[string]string{"name": "mcp__" + it.Server + "__" + it.Tool, "summary": it.Tool})
			case "web_search":
				AddEvent(pid, "tool", map[string]string{"name": "WebSearch", "summary": it.Query})
			}
		}
	}
	store.DB.Exec("insert into usage(project_id,tenant_id,provider,item,quantity,cost) values(?,?,?,?,?,0)", pid, tenant, "codex", q.Label, tokens)
	AddEvent(pid, "result", map[string]any{"cost": 0, "turns": turns, "ms": time.Since(start).Milliseconds(), "tokens": tokens,
		"model": q.Label, "harness": "codex"})
	return thread
}

// codexHint turns Codex's sign-in and plan-limit errors into what to do about them.
func codexHint(msg string) string {
	m := strings.ToLower(msg)
	switch {
	case strings.Contains(m, "401") || strings.Contains(m, "unauthorized") || strings.Contains(m, "refresh token"):
		return "Codex's sign-in has expired. Sign in again: Settings → AI services → Codex."
	case strings.Contains(m, "usage limit") || strings.Contains(m, "rate limit") || strings.Contains(m, "429"):
		return "Your ChatGPT plan's Codex limit is used up for now. Try again later, or pick another model for this message."
	}
	return textutil.Tail(msg, 1000)
}

// Device-code sign-in: Codex prints a link and a one-time code, then waits until the person approves it in a browser.
// The app shows both, so nobody needs a terminal. ponytail: one sign-in at a time, held in memory.
var (
	loginMu   sync.Mutex
	loginCmd  *exec.Cmd
	loginInfo map[string]string
	loginErr  string
)

var (
	loginURL  = regexp.MustCompile(`https://\S+`)
	loginCode = regexp.MustCompile(`\b[A-Z0-9]{4}-[A-Z0-9]{4,6}\b`)
)

// CodexLogin starts a sign-in (or returns the one in progress): {url, code} to show the person.
func CodexLogin(r *http.Request) (any, error) {
	if _, err := exec.LookPath("codex"); err != nil {
		return nil, httpapi.Bad("Codex isn't installed in this copy. Update 37Studio, or from source run: npm i -g @openai/codex")
	}
	loginMu.Lock()
	defer loginMu.Unlock()
	if loginCmd != nil && loginInfo != nil {
		return loginInfo, nil
	}
	cmd := codexCmd("login", "--device-auth")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // `codex` is a Node wrapper around the real binary: stop both
	out, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	loginCmd, loginInfo, loginErr = cmd, nil, ""
	found := make(chan map[string]string, 1)
	go func() {
		var text strings.Builder
		sc := bufio.NewScanner(out)
		sent := false
		for sc.Scan() {
			text.WriteString(textutil.ANSI.ReplaceAllString(sc.Text(), "") + "\n")
			u, c := loginURL.FindString(text.String()), loginCode.FindString(text.String())
			if !sent && u != "" && c != "" {
				found <- map[string]string{"url": u, "code": c}
				sent = true
			}
		}
		err := cmd.Wait()
		loginMu.Lock()
		if loginCmd == cmd { // not already cancelled by a sign-out
			if err != nil {
				loginErr = cmp.Or(strings.TrimSpace(textutil.Tail(text.String(), 400)), err.Error())
			}
			loginCmd, loginInfo = nil, nil
		}
		loginMu.Unlock()
		if !sent {
			close(found)
		}
	}()
	loginMu.Unlock()
	info, ok := <-found
	loginMu.Lock()
	if !ok {
		return nil, fmt.Errorf("Codex didn't start a sign-in: %s", loginErr)
	}
	loginInfo = info
	return info, nil
}

// CodexStatus is whether Codex is signed in, and the sign-in in progress if any.
func CodexStatus(r *http.Request) (any, error) {
	loginMu.Lock()
	defer loginMu.Unlock()
	return map[string]any{"signed_in": CodexReady(), "pending": loginInfo, "error": loginErr}, nil
}

// CodexLogout signs Codex out of this copy.
func CodexLogout(r *http.Request) (any, error) {
	loginMu.Lock()
	if loginCmd != nil {
		syscall.Kill(-loginCmd.Process.Pid, syscall.SIGKILL)
	}
	loginCmd, loginInfo, loginErr = nil, nil, ""
	loginMu.Unlock()
	codexCmd("logout").Run()
	os.Remove(filepath.Join(codexHome(), "auth.json"))
	return map[string]bool{"signed_in": false}, nil
}
