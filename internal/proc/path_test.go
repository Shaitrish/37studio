package proc

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestMergePaths(t *testing.T) {
	got := mergePaths([]string{"/usr/bin", "/bin"}, []string{"/home/u/.nvm/versions/node/v24.1.0/bin", "/usr/bin"}, []string{"", "/home/u/.local/bin"})
	want := []string{"/usr/bin", "/bin", "/home/u/.nvm/versions/node/v24.1.0/bin", "/home/u/.local/bin"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestKnownDirsNewestNodeFirst(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("NVM_DIR", "")
	for _, v := range []string{"v9.11.0", "v24.22.0", "v20.5.1"} {
		os.MkdirAll(filepath.Join(home, ".nvm", "versions", "node", v, "bin"), 0o755)
	}
	os.MkdirAll(filepath.Join(home, ".local", "bin"), 0o755)
	got := knownDirs()
	if len(got) < 4 || got[0] != filepath.Join(home, ".local", "bin") || filepath.Base(filepath.Dir(got[1])) != "v24.22.0" || filepath.Base(filepath.Dir(got[3])) != "v9.11.0" {
		t.Fatalf("got %v", got)
	}
}

func TestShellPath(t *testing.T) {
	// A stand-in shell that, like zsh with a chatty .zshrc, prints a banner before the line asked for.
	sh := filepath.Join(t.TempDir(), "fakesh")
	os.WriteFile(sh, []byte("#!/bin/sh\necho 'Welcome back!'\nprintf '\\n__37studio_path__/opt/x:/usr/bin\\n'\n"), 0o755)
	t.Setenv("SHELL", sh)
	if got := shellPath(); !slices.Equal(got, []string{"/opt/x", "/usr/bin"}) {
		t.Fatalf("got %v", got)
	}
}
