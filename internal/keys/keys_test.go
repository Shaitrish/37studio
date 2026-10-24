package keys

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Shaitrish/37studio/internal/config"
)

// Without any Claude sign-in the app says where to add a key, instead of Claude Code's "Please run /login".
func TestClaudeReadyAndHints(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")
	if err := ClaudeReady(); err == nil || !strings.Contains(err.Error(), "Settings → Keys") {
		t.Fatalf("no key: want the Settings hint, got %v", err)
	}
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "sk-ant-oat01-x")
	if err := ClaudeReady(); err != nil {
		t.Fatalf("with a token: %v", err)
	}
	for msg, want := range map[string]string{
		"Not logged in · Please run /login":                          "no Claude key",
		`API Error: 401 {"type":"authentication_error"}`:             "didn't accept",
		"OAuth token has expired":                                    "expired",
		"Your credit balance is too low to access the Anthropic API": "out of credit",
		"some other failure":                                         "",
	} {
		if got := ClaudeHint(msg); (want == "") != (got == "") || !strings.Contains(got, want) {
			t.Errorf("%q: got %q, want something with %q", msg, got, want)
		}
	}
}

// Platform tokens are stored sealed and read back; rows from before sealing still read as they are.
func TestSealText(t *testing.T) {
	config.DataDir = t.TempDir()
	s := SealText("EAAB-page-token")
	if !strings.HasPrefix(s, sealedPrefix) || strings.Contains(s, "EAAB") || OpenText(s) != "EAAB-page-token" {
		t.Fatalf("sealed %q opens to %q", s, OpenText(s))
	}
	if SealText(s) != s || SealText("") != "" || OpenText("legacy-plain") != "legacy-plain" {
		t.Error("sealing twice, empty values or legacy rows")
	}
	os.WriteFile(filepath.Join(config.DataDir, "secret.key"), make([]byte, 32), 0o600) // a replaced key
	if OpenText(s) != "" {
		t.Error("a token sealed with another key should read as empty")
	}
}
