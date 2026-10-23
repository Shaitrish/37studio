package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// OpenCode runs the same job on other models (DeepSeek, GLM, Kimi...) through `opencode run --format json`.
// It has no --append-system-prompt, so the harness goes into the project's AGENTS.md for the run.
// Claude and OpenCode keep separate sessions; each is caught up on the other's turns with recap().

func startOpenCode(id int64, p map[string]any, q Quality, text string, announce bool) error {
	if _, err := exec.LookPath("opencode"); err != nil {
		return bad("OpenCode is not installed (npm i -g opencode-ai)")
	}
	sid := str(p["oc_session"])
	prompt := recap(id, lastTurn(id, "opencode")) + text
	if sid == "" {
		prompt = strings.TrimSpace(firstPrompt(p) + joinRecap(recap(id, -1)) + text)
	} else if text == "" {
		return bad("message is empty")
	}
	dir := projectDir(id)
	agents := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(agents, []byte(harnessPrompt(p)), 0o644); err != nil {
		return err
	}
	args := []string{"run", "--format", "json", "-m", q.Model}
	if sid != "" {
		args = append(args, "--session", sid)
	}
	if env("STUDIO37_AGENT_PERMISSIONS", "") == "bypass" {
		args = append(args, "--auto")
	}
	exe, _ := os.Executable()
	cmd := exec.Command("opencode", append(args, prompt)...)
	cmd.Dir = dir
	cmd.Env = agentEnv(fmt.Sprintf("STUDIO37_PROJECT=%d", id), "STUDIO37_DATA="+dataDir,
		"PATH="+filepath.Dir(exe)+string(os.PathListSeparator)+os.Getenv("PATH"))
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if isRunning(id) {
		return bad("something is already running on this project")
	}
	if announce {
		addEvent(id, "user", map[string]string{"text": or(text, "Start the job.")})
	}
	setStatus(id, "running")
	parsed := make(chan struct{})
	err := launch(id, cmd, func(err error) {
		pw.Close()
		<-parsed
		os.Remove(agents) // Claude Code reads AGENTS.md too; don't hand it a second copy of the harness
		if err != nil && cmd.ProcessState != nil && !cmd.ProcessState.Success() && stderr.Len() > 0 {
			addEvent(id, "error", map[string]string{"text": tail(ansi.ReplaceAllString(stderr.String(), ""), 2000)})
		}
		setStatus(id, "review")
		drainQueue(id)
	})
	if err != nil {
		os.Remove(agents)
		setStatus(id, str(p["status"]))
		return err
	}
	go func() {
		if s := parseOpenCode(id, p["tenant_id"].(int64), q.Model, pr); s != "" && sid == "" {
			db.Exec("update projects set oc_session=? where id=?", s, id)
		}
		io.Copy(io.Discard, pr)
		close(parsed)
	}()
	return nil
}

// parseOpenCode maps OpenCode's JSON events onto the same chat events and usage rows as Claude runs.
func parseOpenCode(pid, tenant int64, model string, r io.Reader) (session string) {
	start := time.Now()
	var cost, tokens float64
	steps := 0
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var ev struct {
			Type    string `json:"type"`
			Session string `json:"sessionID"`
			Part    struct {
				Text  string `json:"text"`
				Tool  string `json:"tool"`
				State struct {
					Status string          `json:"status"`
					Title  string          `json:"title"`
					Input  json.RawMessage `json:"input"`
					Error  string          `json:"error"`
				} `json:"state"`
				Cost   float64 `json:"cost"`
				Tokens struct {
					Total float64 `json:"total"`
				} `json:"tokens"`
			} `json:"part"`
		}
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		if session == "" {
			session = ev.Session
		}
		switch ev.Type {
		case "text":
			if strings.TrimSpace(ev.Part.Text) != "" {
				addEvent(pid, "text", map[string]string{"text": ev.Part.Text})
			}
		case "tool_use":
			addEvent(pid, "tool", map[string]string{"name": ev.Part.Tool, "summary": or(ev.Part.State.Title, toolSummary(ev.Part.State.Input))})
			imageEvent(pid, ev.Part.Tool, ev.Part.State.Input)
			if ev.Part.State.Status == "error" {
				addEvent(pid, "tool_error", map[string]string{"text": tail(ev.Part.State.Error, 600)})
			}
		case "step_finish":
			cost += ev.Part.Cost
			tokens += ev.Part.Tokens.Total
			steps++
		}
	}
	db.Exec("insert into usage(project_id,tenant_id,provider,item,quantity,cost) values(?,?,?,?,?,?)", pid, tenant, "opencode", model, tokens, cost)
	addEvent(pid, "result", map[string]any{"cost": cost, "turns": steps, "ms": time.Since(start).Milliseconds(), "tokens": tokens,
		"model": model, "harness": "opencode"})
	return session
}

// lastTurn is the id of the last finished turn by a harness (-1 if it never ran), so recap knows where to start.
func lastTurn(id int64, harness string) int64 {
	var last int64 = -1
	evs, _ := rows("select id, body from events where project_id=? and kind='result' order by id", id)
	for _, e := range evs {
		var b map[string]any
		json.Unmarshal([]byte(str(e["body"])), &b)
		h := str(b["harness"])
		if h == harness || (harness == "claude" && h == "") {
			last = e["id"].(int64)
		}
	}
	return last
}

// recap retells the conversation since event `after`, so a harness that missed turns (because another model ran
// them) knows what happened. Returns "" when there is nothing new. after = -1 recaps everything.
func recap(id, after int64) string {
	if after == 0 {
		return ""
	}
	evs, _ := rows("select kind, body from events where project_id=? and id>? and kind in ('user','text') order by id", id, max(after, 0))
	var b strings.Builder
	for _, e := range evs {
		var m map[string]any
		json.Unmarshal([]byte(str(e["body"])), &m)
		if m["queued"] == true {
			continue // queued messages are sent verbatim as the new message
		}
		who := "Editor"
		if e["kind"] == "text" {
			who = "Another agent (different model)"
		}
		fmt.Fprintf(&b, "%s: %s\n", who, str(m["text"]))
	}
	if b.Len() == 0 {
		return ""
	}
	return "While you were away, this happened in the project (files on disk already reflect it):\n" + head(b.String(), 20000) + "\nNow the editor says:\n"
}

// joinRecap follows the job brief: with no history it's a fresh job, otherwise the recap tells the agent where things stand.
func joinRecap(r string) string {
	if r == "" {
		return firstTurn + "\n\n"
	}
	return "\n\nThis job is already in progress.\n" + r
}
