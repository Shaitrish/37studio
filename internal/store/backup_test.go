package store

import (
	"archive/zip"
	"bytes"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Shaitrish/37studio/internal/config"
)

// A backup holds the database and secret.key, and leaves footage and exports out unless media is asked for.
func TestBackup(t *testing.T) {
	config.DataDir = t.TempDir()
	Open()
	t.Cleanup(Close)
	DB.Exec("insert into settings(key,value) values('currency','BDT')")
	for _, f := range []string{"secret.key", "tenants/1/knowledge.md", "projects/1/comp/index.html", "projects/1/out/final.mp4", "projects/1/input/class.mp4", "codex/auth.json"} {
		os.MkdirAll(filepath.Dir(filepath.Join(config.DataDir, f)), 0o755)
		os.WriteFile(filepath.Join(config.DataDir, f), []byte("x"), 0o644)
	}
	names := func(q string) []string {
		rec := httptest.NewRecorder()
		Backup(rec, httptest.NewRequest("GET", "/api/backup"+q, nil))
		zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, f := range zr.File {
			out = append(out, f.Name)
		}
		slices.Sort(out)
		return out
	}
	want := []string{"37studio.db", "projects/1/comp/index.html", "secret.key", "tenants/1/knowledge.md"}
	if got := names(""); !slices.Equal(got, want) {
		t.Fatalf("backup = %v", got)
	}
	if got := names("?media=1"); len(got) != 6 {
		t.Fatalf("with media = %v", got)
	}
	if left, _ := filepath.Glob(filepath.Join(config.DataDir, ".backup-*")); len(left) > 0 {
		t.Fatal("database snapshot left behind")
	}
}
