// Package files knows where things live in the data folder, and the file helpers around it.
package files

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/httpapi"
)

func Copy(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}

var AssetDirs = []string{"logo", "people", "images", "graphics", "videos", "fonts", "audio", "design", "other"}

var TenantDocs = map[string]string{"knowledge": "knowledge.md", "design": "design.md", "replies": "replies.md"}

func TenantDir(id int64) string {
	return filepath.Join(config.DataDir, "tenants", fmt.Sprint(id), "assets")
}

func ProjectDir(id int64) string { return filepath.Join(config.DataDir, "projects", fmt.Sprint(id)) }

func TenantRoot(id int64) string { return filepath.Dir(TenantDir(id)) }

// TenantDoc reads one of the client's markdown docs (knowledge.md, design.md): written by the website scan,
// editable in the UI, read by every project agent.
func TenantDoc(id int64, name string) string {
	b, _ := os.ReadFile(filepath.Join(TenantRoot(id), name))
	return string(b)
}

// assets.json maps a library path ("people/jane.png") to what the file is: caption, site section, source.
// The website scan writes it; editors fix captions in the UI; project agents read it to pick the right asset.
func AssetMeta(id int64) map[string]map[string]any {
	m := map[string]map[string]any{}
	if b, err := os.ReadFile(filepath.Join(TenantRoot(id), "assets.json")); err == nil {
		json.Unmarshal(b, &m)
	}
	return m
}

func SaveAssetMeta(id int64, m map[string]map[string]any) error {
	b, _ := json.MarshalIndent(m, "", "  ")
	return os.WriteFile(filepath.Join(TenantRoot(id), "assets.json"), b, 0o644)
}

type Info struct {
	Path    string    `json:"path"`
	Name    string    `json:"name"`
	URL     string    `json:"url"`
	Kind    string    `json:"kind"`
	Size    int64     `json:"size"`
	Seconds float64   `json:"seconds,omitempty"`
	Mod     time.Time `json:"mod"`
}

func List(root, urlBase string, recursive bool) []Info {
	out := []Info{}
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if p != root && (strings.HasPrefix(d.Name(), ".") || strings.HasSuffix(d.Name(), ".part")) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if p != root && !recursive {
				return filepath.SkipDir
			}
			return nil
		}
		info, _ := d.Info()
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		f := Info{Path: rel, Name: d.Name(), URL: urlBase + "/" + rel, Kind: Kind(p), Size: info.Size(), Mod: info.ModTime()}
		if f.Kind == "video" || f.Kind == "audio" {
			f.Seconds = MediaSeconds(p)
		}
		out = append(out, f)
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Mod.After(out[j].Mod) })
	return out
}

func Kind(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp4", ".mov", ".webm", ".mkv", ".m4v", ".avi":
		return "video"
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".svg", ".avif":
		return "image"
	case ".mp3", ".wav", ".m4a", ".aac", ".ogg", ".flac":
		return "audio"
	case ".ttf", ".otf", ".woff", ".woff2":
		return "font"
	}
	return "other"
}

var durCache sync.Map // "path|mtime" -> seconds; ponytail: unbounded, fine for an internal tool's file count

func MediaSeconds(p string) float64 {
	st, err := os.Stat(p)
	if err != nil {
		return 0
	}
	key := fmt.Sprint(p, "|", st.ModTime().UnixNano())
	if v, ok := durCache.Load(key); ok {
		return v.(float64)
	}
	out, _ := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", p).Output()
	d, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	durCache.Store(key, d)
	return d
}

// SaveUploads streams multipart files to disk; raw lectures can be several GB, so nothing is buffered in memory.
func SaveUploads(r *http.Request, dir string) (any, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, httpapi.Bad("expected multipart upload")
	}
	saved := []string{}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return saved, err
		}
		if part.FileName() == "" {
			continue
		}
		name := SafeName(part.FileName())
		dst := filepath.Join(dir, name)
		f, err := os.Create(dst + ".part")
		if err != nil {
			return saved, err
		}
		_, err = io.Copy(f, part)
		f.Close()
		if err != nil {
			os.Remove(dst + ".part")
			return saved, err
		}
		if err := os.Rename(dst+".part", dst); err != nil {
			return saved, err
		}
		saved = append(saved, name)
	}
	return saved, nil
}

var unsafeChars = regexp.MustCompile(`[^\p{L}\p{N}._ -]+`)

func SafeName(s string) string {
	s = path.Base(strings.ReplaceAll(s, `\`, "/"))
	s = strings.TrimLeft(unsafeChars.ReplaceAllString(s, "_"), ". ")
	if s == "" {
		return "file"
	}
	return s
}

// Within joins rel onto root without letting it escape root.
func Within(root, rel string) string {
	return filepath.Join(root, filepath.FromSlash(path.Clean("/"+rel)))
}

var headTag = regexp.MustCompile(`(?i)<head[^>]*>`)

// Handler serves tenant assets and project files. A project's comp/index.html gets the
// HyperFrames runtime injected, same as `hyperframes play` does, so <hyperframes-player> can drive it.
func Handler() http.Handler {
	fsrv := http.FileServer(http.Dir(config.DataDir))
	runtime := fmt.Sprintf(`<script src="https://cdn.jsdelivr.net/npm/@hyperframes/core@%s/dist/hyperframe.runtime.iife.js"></script>`, config.HFVersion)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := path.Clean("/" + r.URL.Path)
		if !strings.HasPrefix(p, "/tenants/") && !strings.HasPrefix(p, "/projects/") && !strings.HasPrefix(p, "/templates/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		if strings.HasPrefix(p, "/projects/") && strings.HasSuffix(p, "/comp/index.html") {
			b, err := os.ReadFile(filepath.Join(config.DataDir, filepath.FromSlash(p)))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			html := string(b)
			if loc := headTag.FindStringIndex(html); loc != nil {
				html = html[:loc[1]] + runtime + html[loc[1]:]
			} else {
				html = runtime + html
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.WriteString(w, html)
			return
		}
		fsrv.ServeHTTP(w, r)
	})
}

func VideoSize(p string) (w, h int) {
	out, _ := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=p=0", p).Output()
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d,%d", &w, &h)
	return
}

func ReadFile(path string) string {
	b, _ := os.ReadFile(path)
	return string(b)
}
