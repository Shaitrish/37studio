package files

import (
	"os"
	"path/filepath"
	"testing"
)

// The Storage page counts a recording cut into many projects once, wherever it is first seen, so a hard link has
// to be recognisable as the same file from either name -- and an ordinary file must not look shared.
func TestLinkKey(t *testing.T) {
	dir := t.TempDir()
	alone := filepath.Join(dir, "alone.mp4")
	if err := os.WriteFile(alone, []byte("one name"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, shared := LinkKey(alone, stat(t, alone)); shared {
		t.Error("a file with one name reads as shared")
	}

	first := filepath.Join(dir, "recording.mp4")
	second := filepath.Join(dir, "clip.mp4")
	if err := os.WriteFile(first, []byte("shared footage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(first, second); err != nil {
		t.Skipf("no hard links on this filesystem: %v", err)
	}
	a, okA := LinkKey(first, stat(t, first))
	b, okB := LinkKey(second, stat(t, second))
	if !okA || !okB {
		t.Fatalf("hard-linked file doesn't read as shared: %v %v", okA, okB)
	}
	if a != b {
		t.Errorf("the two names give different keys: %v and %v", a, b)
	}
	if c, _ := LinkKey(alone, stat(t, alone)); c == a {
		t.Error("an unrelated file shares the key")
	}
}

// Storage shows what is left on the disk; 0 means it couldn't be read, which a real folder should never give.
func TestFreeSpace(t *testing.T) {
	if FreeSpace(t.TempDir()) == 0 {
		t.Error("no free space reported for a temp folder")
	}
	if FreeSpace(filepath.Join(t.TempDir(), "not-there")) != 0 {
		t.Error("a missing folder reported free space")
	}
}

func stat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}
