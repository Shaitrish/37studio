package project

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/store"
)

// A finished video can be deleted, unless a post still waiting to go out would lose its file.
func TestDeleteProjectFile(t *testing.T) {
	config.DataDir = t.TempDir()
	store.Open()
	t.Cleanup(store.Close)
	out := filepath.Join(files.ProjectDir(1), "out")
	os.MkdirAll(out, 0o755)
	os.WriteFile(filepath.Join(out, "a.mp4"), []byte("v"), 0o644)
	os.WriteFile(filepath.Join(out, "b.mp4"), []byte("v"), 0o644)
	store.DB.Exec("insert into posts(project_id,account_id,platform,account_name,file,title,caption,privacy,publish_at,status) values(1,1,'x','','out/a.mp4','','','','','scheduled')")

	del := func(p string) error {
		r := httptest.NewRequest("DELETE", "/api/projects/1/files?path="+p, nil)
		r.SetPathValue("id", "1")
		_, err := DeleteProjectFile(r)
		return err
	}
	if del("out/./a.mp4") == nil {
		t.Fatal("deleted a file a scheduled post needs")
	}
	if err := del("out/b.mp4"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "b.mp4")); !os.IsNotExist(err) {
		t.Fatal("b.mp4 still there")
	}
	if del("comp/index.html") == nil {
		t.Fatal("deleted outside input/ and out/")
	}
}

// Cleanup removes only old drafts, keeps one a scheduled post needs, and storage counts a hard-linked recording once.
func TestStorageAndCleanup(t *testing.T) {
	config.DataDir = t.TempDir()
	store.Open()
	t.Cleanup(store.Close)
	store.DB.Exec("insert into tenants(id,name) values(1,'Shop')")
	store.DB.Exec("insert into projects(id,tenant_id,title,kind,aspect,seconds,quality) values(1,1,'A','reel','9:16',30,'standard'),(2,1,'B','reel','9:16',30,'standard')")
	d := filepath.Join(files.ProjectDir(1), "out", "drafts")
	os.MkdirAll(d, 0o755)
	old := time.Now().Add(-40 * 24 * time.Hour)
	for _, n := range []string{"old.mp4", "kept.mp4", "new.mp4"} {
		os.WriteFile(filepath.Join(d, n), []byte("12345"), 0o644)
		if n != "new.mp4" {
			os.Chtimes(filepath.Join(d, n), old, old)
		}
	}
	store.DB.Exec("insert into posts(project_id,account_id,platform,account_name,file,publish_at,status) values(1,1,'x','','out/drafts/kept.mp4','','approval')")
	for _, id := range []int64{1, 2} {
		os.MkdirAll(filepath.Join(files.ProjectDir(id), "input"), 0o755)
	}
	os.WriteFile(filepath.Join(files.ProjectDir(1), "input", "rec.mp4"), make([]byte, 1000), 0o644)
	os.Link(filepath.Join(files.ProjectDir(1), "input", "rec.mp4"), filepath.Join(files.ProjectDir(2), "input", "rec.mp4"))

	s, err := Storage(httptest.NewRequest("GET", "/api/storage", nil))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.(map[string]any)["total"].(int64); got != 1015 {
		t.Fatalf("total = %d, want 1015 (the linked recording once)", got)
	}

	res, err := CleanDrafts(httptest.NewRequest("POST", "/api/storage/cleanup", strings.NewReader(`{"days":30}`)))
	if err != nil {
		t.Fatal(err)
	}
	if res.(map[string]any)["files"] != 1 {
		t.Fatalf("cleaned %v", res)
	}
	for n, want := range map[string]bool{"old.mp4": false, "kept.mp4": true, "new.mp4": true} {
		if _, err := os.Stat(filepath.Join(d, n)); (err == nil) != want {
			t.Fatalf("%s present = %v", n, err == nil)
		}
	}
	if _, err := CleanDrafts(httptest.NewRequest("POST", "/api/storage/cleanup", strings.NewReader(`{"days":0}`))); err == nil {
		t.Fatal("days 0 should be refused")
	}
}
