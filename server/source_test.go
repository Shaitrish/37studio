package main

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

// A picked folder lands in source/ without its own top-level name, minus dependencies, dotfiles and secrets,
// and can't write outside source/.
func TestUploadSource(t *testing.T) {
	dataDir = t.TempDir()
	openDB()
	db.Exec("insert into tenants(id,name) values(1,'Shop')")

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, p := range []string{"shop-web/README.md", "shop-web/src/ক্লাস.tsx", "shop-web/node_modules/x/index.js", "shop-web/.env",
		"shop-web/.git/config", "shop-web/certs/server.key", "shop-web/../../escape.txt", "shop-web/public/logo.svg"} {
		w, _ := mw.CreateFormFile(url.PathEscape(p), "x")
		w.Write([]byte("data"))
	}
	mw.Close()
	r := httptest.NewRequest("POST", "/api/tenants/1/source", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.SetPathValue("id", "1")
	if _, err := uploadSource(r); err != nil {
		t.Fatal(err)
	}

	var got []string
	filepath.Walk(sourceDir(1), func(p string, fi os.FileInfo, err error) error {
		if !fi.IsDir() {
			rel, _ := filepath.Rel(sourceDir(1), p)
			got = append(got, filepath.ToSlash(rel))
		}
		return nil
	})
	want := []string{"README.md", "public/logo.svg", "src/ক্লাস.tsx"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(dataDir, "escape.txt")); err == nil {
		t.Fatal("a ../ path escaped the folder")
	}
	if src := sourceInfo(1); src["name"] != "shop-web" || src["files"] != float64(3) {
		t.Fatalf("source.json = %v", src)
	}
}
