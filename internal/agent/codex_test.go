package agent

import (
	"strings"
	"testing"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/store"
)

// `codex exec --json` events become chat events; reconnect noise stays out, a failed turn says what to do.
func TestParseCodex(t *testing.T) {
	config.DataDir = t.TempDir()
	store.Open()
	in := `{"type":"thread.started","thread_id":"01a1-thread"}
{"type":"turn.started"}
{"type":"error","message":"Reconnecting... 2/5 (unexpected status 503)"}
{"type":"item.completed","item":{"id":"item_0","type":"reasoning","text":"thinking"}}
{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"ffprobe in.mp4","aggregated_output":"no such file","exit_code":1,"status":"failed"}}
{"type":"item.completed","item":{"id":"item_2","type":"file_change","changes":[{"path":"comp/index.html","kind":"update"}],"status":"completed"}}
{"type":"item.completed","item":{"id":"item_3","type":"agent_message","text":"The hook is ready."}}
{"type":"turn.completed","usage":{"input_tokens":1200,"cached_input_tokens":0,"output_tokens":300}}
{"type":"turn.failed","error":{"message":"unexpected status 429: usage limit reached"}}
`
	if th := parseCodex(9, 1, codexQualities[0], strings.NewReader(in)); th != "01a1-thread" {
		t.Fatalf("thread %q", th)
	}
	rs, _ := store.Rows("select kind, body from events where project_id=9 order by id")
	var got []string
	for _, r := range rs {
		got = append(got, store.Str(r["kind"])+" "+store.Str(r["body"]))
	}
	all := strings.Join(got, "\n")
	for _, want := range []string{`tool {"name":"Bash","summary":"ffprobe in.mp4"}`, `tool_error {"text":"no such file"}`,
		`tool {"name":"Edit","summary":"comp/index.html"}`, `text {"text":"The hook is ready."}`, "limit is used up", `"harness":"codex"`, `"tokens":1500`} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %s in:\n%s", want, all)
		}
	}
	if strings.Contains(all, "Reconnecting") || strings.Contains(all, "thinking") {
		t.Errorf("noise in chat:\n%s", all)
	}
}
