//go:build windows

package agent

import (
	"os"
	"path/filepath"
	"testing"
)

// Claude Code's Bash tool runs the agent's commands through Git for Windows' bash, and finds it where Git
// installs itself rather than on PATH. Finish setup has to say so when it isn't there, or the agent fails on its
// first command with nothing on the Overview page explaining why.
func TestShellReadyFollowsGitForWindows(t *testing.T) {
	empty := t.TempDir()
	for _, k := range []string{"ProgramFiles", "ProgramW6432", "ProgramFiles(x86)", "LOCALAPPDATA", "SystemRoot"} {
		t.Setenv(k, empty)
	}
	t.Setenv("PATH", empty) // no bash of any kind to fall back on

	if ShellReady() {
		t.Error("no Git and no bash on PATH should read as no shell")
	}

	git := filepath.Join(empty, "Git", "bin")
	if err := os.MkdirAll(git, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(git, "bash.exe"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if !ShellReady() {
		t.Error("Git for Windows is installed; the agent can run commands")
	}
}

// System32's bash.exe is the WSL launcher, not a shell: counting it would tell someone the agent is ready when
// its first command is about to fail.
func TestShellReadyIgnoresTheWSLLauncher(t *testing.T) {
	root := t.TempDir()
	system32 := filepath.Join(root, "System32")
	if err := os.MkdirAll(system32, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(system32, "bash.exe"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"ProgramFiles", "ProgramW6432", "ProgramFiles(x86)", "LOCALAPPDATA"} {
		t.Setenv(k, filepath.Join(root, "none"))
	}
	t.Setenv("SystemRoot", root)
	t.Setenv("PATH", system32)

	if ShellReady() {
		t.Error("the WSL launcher in System32 was taken for a shell")
	}
}
