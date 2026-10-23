package main

import (
	"strings"
	"testing"
)

// Without any Claude sign-in the app says where to add a key, instead of Claude Code's "Please run /login".
func TestClaudeReadyAndHints(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")
	if err := claudeReady(); err == nil || !strings.Contains(err.Error(), "Settings → Keys") {
		t.Fatalf("no key: want the Settings hint, got %v", err)
	}
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "sk-ant-oat01-x")
	if err := claudeReady(); err != nil {
		t.Fatalf("with a token: %v", err)
	}
	for msg, want := range map[string]string{
		"Not logged in · Please run /login":                          "no Claude key",
		`API Error: 401 {"type":"authentication_error"}`:             "didn't accept",
		"OAuth token has expired":                                    "expired",
		"Your credit balance is too low to access the Anthropic API": "out of credit",
		"some other failure":                                         "",
	} {
		if got := claudeHint(msg); (want == "") != (got == "") || !strings.Contains(got, want) {
			t.Errorf("%q: got %q, want something with %q", msg, got, want)
		}
	}
}
