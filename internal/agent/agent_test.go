package agent

import (
	"os/exec"
	"strings"
	"testing"
)

// Platform app secrets never reach the agents.
func TestAgentEnvSecrets(t *testing.T) {
	t.Setenv("X_CLIENT_SECRET", "s")
	t.Setenv("GEMINI_API_KEY", "g")
	env := strings.Join(Env(), "\n")
	if strings.Contains(env, "X_CLIENT_SECRET=") || !strings.Contains(env, "GEMINI_API_KEY=g") {
		t.Fatal("agentEnv should drop *_SECRET and keep AI keys")
	}
}

// OpenCode's models come from `opencode models`; old tier ids saved on projects still find their model.
func TestOpenCodeQualities(t *testing.T) {
	if _, err := exec.LookPath("opencode"); err != nil {
		t.Skip("opencode not installed")
	}
	qs := OpenCodeQualities()
	for _, q := range qs {
		if !strings.HasPrefix(q.ID, "oc:") || q.Harness != "opencode" || q.Group == "" || q.Model == "" {
			t.Fatalf("bad quality %+v", q)
		}
	}
	if q := QualityByID("kimi"); q.Harness != "opencode" || q.Model != "opencode-go/kimi-k3" {
		t.Errorf("legacy kimi tier = %+v", q)
	}
	if q := QualityByID("standard"); q.Harness != "" {
		t.Errorf("standard = %+v", q)
	}
}
