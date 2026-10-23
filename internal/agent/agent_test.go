package agent

import (
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
