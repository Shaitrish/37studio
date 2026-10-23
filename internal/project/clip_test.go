package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/store"
)

// A queued video copies its source project's settings, shares the recording by hard link and gets the transcript.
func TestQueueClip(t *testing.T) {
	config.DataDir = t.TempDir()
	store.Open()
	store.DB.Exec("insert into tenants(id,name) values(1,'Shop')")
	store.DB.Exec("insert into projects(id,tenant_id,title,kind,aspect,seconds,quality,language) values(1,1,'Class 7','hook-reel','9:16',60,'standard','bn')")
	src := files.ProjectDir(1)
	os.MkdirAll(filepath.Join(src, "input"), 0o755)
	os.MkdirAll(filepath.Join(src, "work", "transcript"), 0o755)
	os.WriteFile(filepath.Join(src, "input", "class.mp4"), []byte("video"), 0o644)
	os.WriteFile(filepath.Join(src, "work", "transcript", "transcript.json"), []byte("{}"), 0o644)

	id, err := queueClip(1, "Q1: Newton's laws", "The first solved question.", "00:12:30", "00:19:10", 0)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := store.Row("select * from projects where id=?", id)
	if p["parent_id"] != int64(1) || p["clip_stage"] != "queued" || p["aspect"] != "9:16" || p["language"] != "bn" || store.Num(p["seconds"]) != 60 {
		t.Fatalf("child = %v", p)
	}
	if b := store.Str(p["brief"]); !strings.Contains(b, "The first solved question.") || !strings.Contains(b, "00:12:30 to 00:19:10") {
		t.Fatalf("brief = %q", b)
	}
	a, _ := os.Stat(filepath.Join(src, "input", "class.mp4"))
	b, err := os.Stat(filepath.Join(files.ProjectDir(id), "input", "class.mp4"))
	if err != nil || !os.SameFile(a, b) {
		t.Fatal("the recording should be hard-linked, not copied")
	}
	if _, err := os.Stat(filepath.Join(files.ProjectDir(id), "work", "transcript", "transcript.json")); err != nil {
		t.Fatal("transcript not copied")
	}
}
