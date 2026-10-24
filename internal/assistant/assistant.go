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
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/textutil"
)

// The assistant: a chat in the sidebar that runs the whole studio by talking. It is Claude Code connected to this
// app's own MCP server (package mcp) and nothing else: no shell and no file edits, only the tools the UI's buttons use, plus
// web search. Its chat is stored as the events of project 0, and one Claude session carries the conversation on.
const assistantID int64 = 0

const assistantPrompt = `You are the assistant inside 37Studio, an app that makes branded videos for brands and runs their social media. You work for the person using the app, through the 37studio tools: brands with their brand library and knowledge base, campaigns, video projects, publishing, Autopilot plans, the inbox and costs.

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
	return map[string]any{"events": evs, "running": agent.IsRunning(assistantID), "started": store.Setting("assistant_session") != ""}, err
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

// Reset starts a new conversation: the old chat and Claude session are dropped.
func Reset(r *http.Request) (any, error) {
	if agent.IsRunning(assistantID) {
		return nil, httpapi.Bad("stop the assistant first")
	}
	store.DB.Exec("delete from events where project_id=?", assistantID)
	_, err := store.DB.Exec("delete from settings where key='assistant_session'")
	return nil, err
}

func startAssistant(text, quality string) error {
	if err := keys.ClaudeReady(); err != nil {
		return err
	}
	dir := filepath.Join(config.DataDir, "assistant")
	cfg, err := assistantMCP(dir)
	if err != nil {
		return err
	}
	q := agent.QualityByID(cmp.Or(quality, "draft")) // Sonnet is plenty for driving the app's tools
	if q.Harness != "" {
		q = agent.QualityByID("draft")
	}
	sid := store.Setting("assistant_session")
	session := []string{"--resume", sid}
	if sid == "" {
		sid = uuid.NewString()
		session = []string{"--session-id", sid}
		store.DB.Exec("insert into settings(key,value) values('assistant_session',?) on conflict(key) do update set value=excluded.value", sid)
	}
	args := append([]string{"-p", "--output-format", "stream-json", "--verbose", "--model", q.Model, "--effort", q.Effort,
		"--strict-mcp-config", "--mcp-config", cfg, "--settings", `{"disableAllHooks":true}`,
		"--max-budget-usd", store.Setting("run_budget_usd"), "--append-system-prompt", assistantPrompt,
		"--tools", "WebFetch,WebSearch", "--allowedTools", "mcp__37studio,WebFetch,WebSearch"}, session...)
	cmd := exec.Command(config.ClaudeBin, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(text)
	cmd.Env = agent.Env("STUDIO37_DATA=" + config.DataDir)
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	agent.AddEvent(assistantID, "user", map[string]string{"text": text})
	parsed := make(chan struct{})
	err = agent.Launch(assistantID, cmd, func(err error) {
		pw.Close()
		<-parsed
		if err != nil && stderr.Len() > 0 {
			agent.AddEvent(assistantID, "error", map[string]string{"text": textutil.Tail(stderr.String(), 2000)})
			if strings.Contains(stderr.String(), "No conversation found") {
				store.DB.Exec("delete from settings where key='assistant_session'")
			}
		}
	})
	if err != nil {
		agent.AddEvent(assistantID, "error", map[string]string{"text": "Could not start the assistant: " + err.Error()})
		return err
	}
	go func() {
		agent.ParseStream(assistantID, 0, q.Model, pr)
		io.Copy(io.Discard, pr)
		close(parsed)
	}()
	return nil
}

// assistantMCP writes the MCP config that points Claude Code back at this server. It's a file (readable only by this
// user) rather than an argument, because it carries the app's password when one is set.
func assistantMCP(dir string) (string, error) {
	host, port, _ := net.SplitHostPort(config.Addr)
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	srv := map[string]any{"type": "http", "url": "http://" + net.JoinHostPort(host, port) + "/mcp"}
	if pw := config.Env("STUDIO37_PASSWORD", ""); pw != "" {
		srv["headers"] = map[string]string{"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("37studio:"+pw))}
	}
	raw, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{"37studio": srv}})
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(dir, "mcp.json")
	return p, os.WriteFile(p, raw, 0o600)
}
