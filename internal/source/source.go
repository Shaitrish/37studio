// Package source keeps a brand's own project folder: the upload, the skip rules and its summary.
package source

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/store"
)

// Never copied: dependencies, build output, version control, and anything that looks like a secret, since the
// agents may read every file here and so send it to Anthropic. The web UI filters the same way before uploading.
var skipDirs = map[string]bool{"node_modules": true, "vendor": true, "dist": true, "build": true, "out": true, "target": true, "coverage": true, "__pycache__": true, "venv": true, "Pods": true, "DerivedData": true}

var secretFile = regexp.MustCompile(`(?i)(\.(pem|key|p12|pfx|jks|keystore|kdbx|sqlite3?|db)$|^id_(rsa|dsa|ecdsa|ed25519)|credential|secret|service-account)`)

func skipSource(rel string) bool {
	parts := strings.Split(rel, "/")
	for _, p := range parts[:len(parts)-1] {
		if skipDirs[p] || strings.HasPrefix(p, ".") {
			return true
		}
	}
	f := parts[len(parts)-1]
	return strings.HasPrefix(f, ".") || secretFile.MatchString(f)
}

func Dir(id int64) string { return filepath.Join(files.TenantRoot(id), "source") }

// Info is what the last folder upload recorded: name, files, bytes, uploaded. nil when there is none.
func Info(id int64) map[string]any {
	var m map[string]any
	if json.Unmarshal([]byte(files.ReadFile(filepath.Join(files.TenantRoot(id), "source.json"))), &m) != nil {
		return nil
	}
	return m
}

// Upload replaces source/ with the uploaded folder. Each part is named by its URL-encoded path inside the folder,
// starting with the folder's own name (the browser's webkitRelativePath), which is dropped.
func Upload(r *http.Request) (any, error) {
	id := httpapi.PathID(r)
	if _, err := store.Row("select id from tenants where id=?", id); err != nil {
		return nil, err
	}
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, httpapi.Bad("expected multipart upload")
	}
	stage := Dir(id) + ".part"
	os.RemoveAll(stage)
	defer os.RemoveAll(stage)
	var name string
	var count, size int64
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		rel, err := url.PathUnescape(part.FormName())
		if err != nil {
			continue
		}
		top, inner, _ := strings.Cut(strings.TrimPrefix(path.Clean("/"+strings.ReplaceAll(rel, `\`, "/")), "/"), "/")
		if inner == "" || skipSource(inner) {
			continue
		}
		dst := files.Within(stage, inner)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, err
		}
		f, err := os.Create(dst)
		if err != nil {
			return nil, err
		}
		n, err := io.Copy(f, part)
		f.Close()
		if err != nil {
			return nil, err
		}
		name, count, size = top, count+1, size+n
	}
	if count == 0 {
		return nil, httpapi.Bad("that folder has no files 37Studio can use")
	}
	os.RemoveAll(Dir(id))
	if err := os.Rename(stage, Dir(id)); err != nil {
		return nil, err
	}
	info := map[string]any{"name": name, "files": count, "bytes": size, "uploaded": time.Now().Format(time.DateTime)}
	b, _ := json.Marshal(info)
	return info, os.WriteFile(filepath.Join(files.TenantRoot(id), "source.json"), b, 0o644)
}
