package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/store"
)

// Two subagents started in one message: each gets a start and a finish, and their steps are filed under them, not
// in the main agent's list. A background one stays open past its "launched" answer until its task_notification.
// One left running when the turn ends is closed then.
func TestParseStreamSubagents(t *testing.T) {
	config.DataDir = t.TempDir()
	store.Open()
	t.Cleanup(store.Close)
	stream := strings.Join([]string{
		`{"type":"assistant","parent_tool_use_id":null,"message":{"content":[` +
			`{"type":"tool_use","id":"toolu_a","name":"Agent","input":{"description":"Find hook moments","subagent_type":"general-purpose","prompt":"..."}},` +
			`{"type":"tool_use","id":"toolu_b","name":"Agent","input":{"description":"Search the registry","prompt":"...","run_in_background":true}}]}}`,
		`{"type":"assistant","parent_tool_use_id":"toolu_a","message":{"content":[{"type":"tool_use","id":"toolu_1","name":"Grep","input":{"pattern":"price"}}]}}`,
		`{"type":"assistant","parent_tool_use_id":"toolu_b","message":{"content":[{"type":"text","text":"Found three lower thirds."}]}}`,
		`{"type":"user","parent_tool_use_id":"toolu_a","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_1","is_error":true,"content":"no such file"}]}}`,
		`{"type":"user","parent_tool_use_id":null,"message":{"content":[{"type":"tool_result","tool_use_id":"toolu_a","content":[{"type":"text","text":"report"}]}]}}`,
		`{"type":"assistant","parent_tool_use_id":null,"message":{"content":[{"type":"tool_use","id":"toolu_2","name":"Read","input":{"file_path":"comp/index.html"}}]}}`,
		// A subagent Claude Code put in the background: its Agent call answers "launched" at once and it finishes later.
		`{"type":"assistant","parent_tool_use_id":null,"message":{"content":[{"type":"tool_use","id":"toolu_c","name":"Agent","input":{"description":"Fetch photos","prompt":"..."}}]}}`,
		`{"type":"user","parent_tool_use_id":null,"message":{"content":[{"type":"tool_result","tool_use_id":"toolu_c","content":[{"type":"text","text":"Async agent launched successfully. agentId: a1"}]}]}}`,
		`{"type":"assistant","parent_tool_use_id":"toolu_c","message":{"content":[{"type":"tool_use","id":"toolu_3","name":"WebFetch","input":{"url":"https://commons.wikimedia.org"}}]}}`,
		`{"type":"system","subtype":"task_notification","task_id":"a1","tool_use_id":"toolu_c","status":"completed"}`,
		`{"type":"result","total_cost_usd":0.5,"num_turns":3,"duration_ms":1000}`,
	}, "\n")
	ParseStream(7, 1, "claude-sonnet-5-5", strings.NewReader(stream))

	rows, _ := store.Rows("select kind, body from events where project_id=7 order by id")
	var got []string
	for _, r := range rows {
		var b map[string]any
		json.Unmarshal([]byte(store.Str(r["body"])), &b)
		s := store.Str(r["kind"])
		for _, k := range []string{"id", "name", "parent", "error"} {
			if v, ok := b[k]; ok {
				s += " " + k + "=" + strings.Trim(string(must(json.Marshal(v))), `"`)
			}
		}
		got = append(got, s)
	}
	want := []string{
		"agent id=toolu_a name=Find hook moments",
		"agent id=toolu_b name=Search the registry",
		"tool name=Grep parent=toolu_a",
		"text parent=toolu_b",
		"tool_error parent=toolu_a",
		"agent_done id=toolu_a error=false",
		"tool name=Read",
		"agent id=toolu_c name=Fetch photos",
		"tool name=WebFetch parent=toolu_c",
		"agent_done id=toolu_c error=false",
		"agent_done id=toolu_b",
		"result error=false",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("events:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func must(b []byte, _ error) []byte { return b }
