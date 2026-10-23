package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// The MCP server: the handshake, every tool pointing at a real route, and a few calls through the real router.
func TestMCP(t *testing.T) {
	dataDir = t.TempDir()
	openDB()
	mux := routes()
	rpc := func(method string, params any) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("POST", "/mcp", strings.NewReader(string(raw))))
		var out map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s: %d %s", method, rec.Code, rec.Body)
		}
		return out
	}
	tool := func(name string, args map[string]any) (string, bool) {
		t.Helper()
		res, _ := rpc("tools/call", map[string]any{"name": name, "arguments": args})["result"].(map[string]any)
		if res == nil {
			t.Fatalf("%s: no result", name)
		}
		return res["content"].([]any)[0].(map[string]any)["text"].(string), res["isError"].(bool)
	}

	if v := rpc("initialize", map[string]any{"protocolVersion": mcpProtocol})["result"].(map[string]any); v["serverInfo"].(map[string]any)["name"] != "37studio" {
		t.Fatalf("initialize = %v", v)
	}
	if n := len(rpc("tools/list", nil)["result"].(map[string]any)["tools"].([]any)); n != len(mcpTools) {
		t.Fatalf("tools/list has %d tools, want %d", n, len(mcpTools))
	}
	for _, tl := range mcpTools { // every API-backed tool must hit a registered route, not the UI's catch-all
		if tl.Do != nil {
			continue
		}
		path := tl.Path
		for _, id := range []string{"company_id", "project_id", "campaign_id", "autopilot_id"} {
			path = strings.ReplaceAll(path, "{"+id+"}", "1")
		}
		if _, pattern := mux.Handler(httptest.NewRequest(tl.Method, path, nil)); pattern == "" || pattern == "GET /" {
			t.Errorf("%s: %s %s matches no API route (%q)", tl.Name, tl.Method, path, pattern)
		}
	}

	if out, bad := tool("create_company", map[string]any{"name": "Dhaka Bites", "knowledge": "# Dhaka Bites\nBiryani since 1998."}); bad {
		t.Fatal(out)
	}
	if out, _ := tool("list_companies", nil); !strings.Contains(out, "Dhaka Bites") {
		t.Fatalf("list_companies = %s", out)
	}
	if out, _ := tool("get_company", map[string]any{"company_id": 1}); !strings.Contains(out, "Biryani since 1998") {
		t.Fatalf("get_company = %s", out)
	}
	if out, bad := tool("create_campaign", map[string]any{"company_id": 1, "name": "Eid", "brief": "Eid offers"}); bad || !strings.Contains(out, `"tenant_id":1`) {
		t.Fatalf("create_campaign = %s", out)
	}
	out, bad := tool("create_video", map[string]any{"company_id": 1, "title": "Eid reel", "brief": "Our Eid platter", "start": false})
	if bad || !strings.Contains(out, "Eid reel") {
		t.Fatalf("create_video = %s", out)
	}
	if out, bad := tool("get_project", map[string]any{"project_id": 1}); bad || !strings.Contains(out, "Our Eid platter") {
		t.Fatalf("get_project = %s", out)
	}
	if out, bad := tool("add_company_asset", map[string]any{"company_id": 1, "folder": "logo", "filename": "../mark.svg", "content_base64": "PHN2Zy8+"}); bad || !strings.Contains(out, "mark.svg") {
		t.Fatalf("add_company_asset = %s", out)
	}
	if out, bad := tool("delete_company", map[string]any{"company_id": 1}); !bad || !strings.Contains(out, "still has projects") {
		t.Fatalf("delete_company with projects = %s %v", out, bad)
	}
	if e := rpc("tools/call", map[string]any{"name": "nope"})["error"]; e == nil {
		t.Fatal("unknown tool should be a JSON-RPC error")
	}

	// A page on another site can't drive it (DNS rebinding); the app's own pages and MCP clients (no Origin) can.
	for origin, ok := range map[string]bool{"": true, "http://localhost:8080": true, "https://evil.example": false} {
		req := httptest.NewRequest("POST", "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if (rec.Code == 200) != ok {
			t.Errorf("Origin %q: status %d", origin, rec.Code)
		}
	}
}
