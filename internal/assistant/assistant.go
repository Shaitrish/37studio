// Package assistant is the in-app chat that runs the studio through its own MCP tools.
package assistant

import (
	"bytes"
	"cmp"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/Shaitrish/37studio/internal/agent"
	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/keys"
	"github.com/Shaitrish/37studio/internal/proc"
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/textutil"
)

// The assistant: a chat in the sidebar that runs the whole studio by talking. It is Claude Code (or OpenCode, for other
// providers' models) connected to this app's own MCP server (package mcp) and nothing else: no shell and no file edits,
// only the tools the UI's buttons use, plus web search. Its chat is stored as the events of project 0; one Claude session
// and one OpenCode session carry the conversation on, and each hears a recap of the turns the other answered.
const assistantID int64 = 0

const assistantPrompt = `You are the assistant inside 37Studio, an app that makes branded videos for brands and runs their social media. You work for the person using the app, through the 37studio tools: brands (the tools still say company: list_companies, company_id) with their brand library and knowledge base, campaigns, video projects, publishing, Autopilot plans, the inbox and costs.

- Do what they ask with the tools. For a request with several steps (add a brand, then plan a campaign, then start three videos), say the plan in a line, then do it.
- Each video is built by a separate editor agent in the background and takes minutes. After create_video, say it has started and link the project; don't wait for it to finish. Check progress only when asked.
- Link to what you make, as markdown links the app opens: [Project 12](#/projects/12), [Dhaka Bites](#/clients/3), [Inbox](#/inbox).
- Ask before deleting anything, publishing, approving Autopilot videos or sending inbox replies, and before Premium-quality jobs, which cost the most.
- You can also set the app up: API keys and platform credentials (set_keys, only with values the user gives you; never repeat a secret back), settings (like the display currency and its exchange rate, models and budgets: update_settings), and account connections. Bluesky, Mastodon and WhatsApp connect directly; for the platforms with a sign-in page, give the user the connect_link and they finish it in their browser.
- Facts about a brand come from its knowledge base; don't invent prices, offers or claims.
- Reply in the user's language, short and plain.`

func Get(r *http.Request) (any, error) {
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	evs, err := store.Rows("select * from events where project_id=? and id>? order by id", assistantID, after)
	return map[string]any{"events": evs, "running": agent.IsRunning(assistantID), "started": store.Setting("assistant_session") != "" || store.Setting("assistant_oc_session") != ""}, err
}

func Post(r *http.Request) (any, error) {
	b, err := httpapi.Body(r)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(store.Str(b["text"]))
	if text == "" {
		return nil, httpapi.Bad("message is empty")
	}
	if agent.IsRunning(assistantID) {
		return nil, httpapi.Bad("the assistant is still answering; wait for it, or press Stop")
	}
	return map[string]bool{"ok": true}, startAssistant(text, store.Str(b["quality"]))
}

func Stop(r *http.Request) (any, error) { return nil, agent.StopProc(assistantID) }

// Reset starts a new conversation: the old chat and the Claude and OpenCode sessions are dropped.
func Reset(r *http.Request) (any, error) {
	if agent.IsRunning(assistantID) {
		return nil, httpapi.Bad("stop the assistant first")
	}
	store.DB.Exec("delete from events where project_id=?", assistantID)
	_, err := store.DB.Exec("delete from settings where key in ('assistant_session','assistant_oc_session')")
	return nil, err
}

func startAssistant(text, quality string) error {
	q := agent.QualityByID(cmp.Or(quality, "draft")) // Sonnet is plenty for driving the app's tools
	dir := filepath.Join(config.DataDir, "assistant")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if q.Harness == "opencode" {
		return startOpenCode(dir, text, q)
	}
	if err := keys.ClaudeReady(); err != nil {
		return err
	}
	url, headers := mcpServer()
	srv := map[string]any{"type": "http", "url": url}
	if headers != nil {
		srv["headers"] = headers
	}
	cfg, err := writeConfig(dir, "mcp.json", map[string]any{"mcpServers": map[string]any{"37studio": srv}})
	if err != nil {
		return err
	}
	sid := store.Setting("assistant_session")
	session := []string{"--resume", sid}
	if sid == "" {
		sid = uuid.NewString()
		session = []string{"--session-id", sid}
		saveSetting("assistant_session", sid)
	}
	args := append([]string{"-p", "--output-format", "stream-json", "--verbose", "--model", q.Model, "--effort", q.Effort,
		"--strict-mcp-config", "--mcp-config", cfg, "--settings", `{"disableAllHooks":true}`,
		"--max-budget-usd", store.Setting("run_budget_usd"), "--append-system-prompt", assistantPrompt,
		"--tools", "WebFetch,WebSearch", "--allowedTools", "mcp__37studio,WebFetch,WebSearch"}, session...)
	cmd := proc.Command(config.ClaudeBin, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(agent.Recap(assistantID, "claude") + text) // turns another model answered meanwhile
	cmd.Env = agent.Env("STUDIO37_DATA=" + config.DataDir)
	return run(cmd, text, func(r io.Reader) { agent.ParseStream(assistantID, 0, q.Model, r) }, func(stderr string) {
		if strings.Contains(stderr, "No conversation found") {
			store.DB.Exec("delete from settings where key='assistant_session'")
		}
	})
}

// startOpenCode runs the other providers' models the same way: an OpenCode agent whose only tools are this app's MCP
// server and the web, with no shell, file or other MCP tools. OpenCode has nothing like --max-budget-usd, so
// ParseOpenCode stops the turn once its cost passes the per-turn budget in Settings.
func startOpenCode(dir, text string, q agent.Quality) error {
	if _, err := exec.LookPath("opencode"); err != nil {
		return httpapi.Bad("OpenCode is not installed (npm i -g opencode-ai)")
	}
	url, headers := mcpServer()
	srv := map[string]any{"type": "remote", "url": url, "oauth": false}
	if headers != nil {
		srv["headers"] = headers
	}
	// OpenCode's free models refuse requests from a custom agent or prompt, so this is its own "build" agent with the
	// tools narrowed, and the prompt goes in as instructions on top of OpenCode's.
	prompt := filepath.Join(dir, "assistant.md")
	if err := os.WriteFile(prompt, []byte(assistantPrompt), 0o644); err != nil {
		return err
	}
	cfg, err := writeConfig(dir, "opencode.json", map[string]any{
		"$schema":      "https://opencode.ai/config.json",
		"mcp":          map[string]any{"37studio": srv},
		"instructions": []string{prompt},
		"agent": map[string]any{"build": map[string]any{
			// The last matching rule wins, and JSON object keys come out sorted, so "*" (deny everything) stays first.
			"permission": map[string]string{"*": "deny", "37studio_*": "allow", "webfetch": "allow", "websearch": "allow"}}},
	})
	if err != nil {
		return err
	}
	args := []string{"run", "--format", "json", "--agent", "build", "-m", q.Model}
	sid := store.Setting("assistant_oc_session")
	if sid != "" {
		args = append(args, "--session", sid)
	}
	cmd := proc.Command("opencode", append(args, "--", agent.Recap(assistantID, "opencode")+text)...)
	cmd.Dir = dir
	cmd.Env = agent.Env("STUDIO37_DATA="+config.DataDir, "OPENCODE_CONFIG="+cfg)
	return run(cmd, text, func(r io.Reader) {
		if s := agent.ParseOpenCode(assistantID, 0, q.Model, r); s != "" && sid == "" {
			saveSetting("assistant_oc_session", s)
		}
	}, nil)
}

// run starts the assistant's process, shows the user's message, and streams the reply into the chat.
func run(cmd *exec.Cmd, text string, parse func(io.Reader), failed func(stderr string)) error {
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	agent.AddEvent(assistantID, "user", map[string]string{"text": text})
	parsed := make(chan struct{})
	err := agent.Launch(assistantID, cmd, func(err error) {
		pw.Close()
		<-parsed
		if err != nil && stderr.Len() > 0 {
			agent.AddEvent(assistantID, "error", map[string]string{"text": textutil.Tail(textutil.ANSI.ReplaceAllString(stderr.String(), ""), 2000)})
			if failed != nil {
				failed(stderr.String())
			}
		}
	})
	if err != nil {
		agent.AddEvent(assistantID, "error", map[string]string{"text": "Could not start the assistant: " + err.Error()})
		return err
	}
	go func() {
		parse(pr)
		io.Copy(io.Discard, pr)
		close(parsed)
	}()
	return nil
}

func saveSetting(key, value string) {
	store.DB.Exec("insert into settings(key,value) values(?,?) on conflict(key) do update set value=excluded.value", key, value)
}

// mcpServer is this app's own MCP endpoint, with the app's password when one is set.
func mcpServer() (url string, headers map[string]string) {
	host, port, _ := net.SplitHostPort(config.Addr)
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	if pw := config.Env("STUDIO37_PASSWORD", ""); pw != "" {
		headers = map[string]string{"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("37studio:"+pw))}
	}
	return "http://" + net.JoinHostPort(host, port) + "/mcp", headers
}

// writeConfig saves a harness config as a file readable only by this user, rather than an argument, because it
// carries the app's password when one is set.
func writeConfig(dir, name string, cfg any) (string, error) {
	raw, _ := json.Marshal(cfg)
	p := filepath.Join(dir, name)
	return p, os.WriteFile(p, raw, 0o600)
}
