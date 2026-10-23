package main

import (
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
)

var (
	tenantCols  = []string{"name", "contact", "email", "phone", "website", "teacher", "bio", "language", "brand_color", "accent_color", "font", "notes", "inbox_mode"}
	projectCols = []string{"tenant_id", "campaign_id", "title", "kind", "aspect", "seconds", "quality", "brief", "focus", "language", "status"}
	assetDirs   = []string{"logo", "people", "images", "graphics", "videos", "fonts", "audio", "design", "other"}
	tenantDocs  = map[string]string{"knowledge": "knowledge.md", "design": "design.md", "replies": "replies.md"}
)

func tenantDir(id int64) string  { return filepath.Join(dataDir, "tenants", fmt.Sprint(id), "assets") }
func projectDir(id int64) string { return filepath.Join(dataDir, "projects", fmt.Sprint(id)) }
func tenantRoot(id int64) string { return filepath.Dir(tenantDir(id)) }

// tenantDoc reads one of the client's markdown docs (knowledge.md, design.md): written by the website scan,
// editable in the UI, read by every project agent.
func tenantDoc(id int64, name string) string {
	b, _ := os.ReadFile(filepath.Join(tenantRoot(id), name))
	return string(b)
}

// ---- tenants

func listTenants(r *http.Request) (any, error) {
	return rows(`select t.*,
	  (select count(*) from projects p where p.tenant_id=t.id) as project_count,
	  (select coalesce(sum(cost),0) from usage u where u.tenant_id=t.id) as cost_usd
	  from tenants t order by t.name collate nocase`)
}

func getTenant(r *http.Request) (any, error) {
	id := pathID(r)
	t, err := row("select * from tenants where id=?", id)
	if err != nil {
		return nil, err
	}
	t["assets"] = listFiles(tenantDir(id), fmt.Sprintf("/files/tenants/%d/assets", id), true)
	t["asset_meta"] = assetMeta(id)
	for k, f := range tenantDocs {
		t[k] = tenantDoc(id, f)
	}
	t["source"] = sourceInfo(id)
	t["scanning"] = isRunning(-id)
	t["projects"], _ = rows("select id,title,kind,status,created_at from projects where tenant_id=? order by id desc", id)
	return t, nil
}

func saveTenant(r *http.Request) (any, error) {
	b, err := body(r)
	if err != nil {
		return nil, err
	}
	id := pathID(r)
	if id == 0 && strings.TrimSpace(str(b["name"])) == "" {
		return nil, bad("name is required")
	}
	if m := str(b["inbox_mode"]); m == "draft" || m == "send" {
		if err := needLicense("Inbox auto-replies"); err != nil {
			return nil, err
		}
	}
	if id, err = save("tenants", tenantCols, id, b); err != nil {
		return nil, err
	}
	for _, d := range assetDirs {
		os.MkdirAll(filepath.Join(tenantDir(id), d), 0o755)
	}
	for k, f := range tenantDocs {
		if v, ok := b[k].(string); ok {
			if err := os.WriteFile(filepath.Join(tenantRoot(id), f), []byte(v), 0o644); err != nil {
				return nil, err
			}
		}
	}
	return row("select * from tenants where id=?", id)
}

func deleteTenant(r *http.Request) (any, error) {
	id := pathID(r)
	var n int
	db.QueryRow("select count(*) from projects where tenant_id=?", id).Scan(&n)
	if n > 0 {
		return nil, bad("this company still has projects; delete those first")
	}
	if _, err := db.Exec("delete from tenants where id=?", id); err != nil {
		return nil, err
	}
	return nil, os.RemoveAll(tenantRoot(id))
}

func uploadAssets(r *http.Request) (any, error) {
	folder := r.URL.Query().Get("folder")
	if !contains(assetDirs, folder) {
		return nil, bad("unknown folder")
	}
	return saveUploads(r, filepath.Join(tenantDir(pathID(r)), folder))
}

func deleteAsset(r *http.Request) (any, error) {
	id, p := pathID(r), r.URL.Query().Get("path")
	if err := os.Remove(within(tenantDir(id), p)); err != nil {
		return nil, err
	}
	meta := assetMeta(id)
	delete(meta, p)
	return nil, saveAssetMeta(id, meta)
}

// assets.json maps a library path ("people/jane.png") to what the file is: caption, site section, source.
// The website scan writes it; editors fix captions in the UI; project agents read it to pick the right asset.
func assetMeta(id int64) map[string]map[string]any {
	m := map[string]map[string]any{}
	if b, err := os.ReadFile(filepath.Join(tenantRoot(id), "assets.json")); err == nil {
		json.Unmarshal(b, &m)
	}
	return m
}

func saveAssetMeta(id int64, m map[string]map[string]any) error {
	b, _ := json.MarshalIndent(m, "", "  ")
	return os.WriteFile(filepath.Join(tenantRoot(id), "assets.json"), b, 0o644)
}

// updateAsset edits one asset: {path, caption?, use?, section?, context?, source?} sets its metadata, {path, folder?} moves it to another folder.
func updateAsset(r *http.Request) (any, error) {
	b, err := body(r)
	if err != nil {
		return nil, err
	}
	id, p := pathID(r), str(b["path"])
	meta := assetMeta(id)
	if _, err := os.Stat(within(tenantDir(id), p)); err != nil {
		return nil, bad("no such asset")
	}
	for _, k := range []string{"caption", "use", "section", "context", "source"} {
		if v, ok := b[k].(string); ok {
			if meta[p] == nil {
				meta[p] = map[string]any{}
			}
			meta[p][k] = strings.TrimSpace(v)
		}
	}
	if f := str(b["folder"]); f != "" {
		if !contains(assetDirs, f) {
			return nil, bad("unknown folder")
		}
		np := f + "/" + path.Base(p)
		if _, err := os.Stat(within(tenantDir(id), np)); err == nil {
			return nil, bad("a file with that name already exists there")
		}
		os.MkdirAll(filepath.Join(tenantDir(id), f), 0o755)
		if err := os.Rename(within(tenantDir(id), p), within(tenantDir(id), np)); err != nil {
			return nil, err
		}
		if meta[p] != nil {
			meta[np] = meta[p]
			delete(meta, p)
		}
	}
	return nil, saveAssetMeta(id, meta)
}

// ---- projects

const projectSelect = `select p.*, t.name as tenant_name, c.name as campaign_name
  from projects p join tenants t on t.id=p.tenant_id left join campaigns c on c.id=p.campaign_id`

func listProjects(r *http.Request) (any, error) {
	q, args := projectSelect+" where 1=1", []any{}
	if t := r.URL.Query().Get("tenant"); t != "" {
		q, args = q+" and p.tenant_id=?", append(args, t)
	}
	if c := r.URL.Query().Get("campaign"); c != "" {
		q, args = q+" and p.campaign_id=?", append(args, c)
	}
	ps, err := rows(q+" order by p.id desc", args...)
	for _, p := range ps {
		p["quote"] = quote(p)
		p["running"] = isRunning(p["id"].(int64))
	}
	return ps, err
}

func getProject(r *http.Request) (any, error) {
	id := pathID(r)
	p, err := row(projectSelect+" where p.id=?", id)
	if err != nil {
		return nil, err
	}
	dir, base := projectDir(id), fmt.Sprintf("/files/projects/%d", id)
	p["inputs"] = listFiles(filepath.Join(dir, "input"), base+"/input", true)
	p["outputs"] = listFiles(filepath.Join(dir, "out"), base+"/out", false)
	p["drafts"] = listFiles(filepath.Join(dir, "out", "drafts"), base+"/out/drafts", false) // not recursive: renders leave scratch dirs here
	if _, err := os.Stat(filepath.Join(dir, "comp", "index.html")); err == nil {
		// Version = newest .html anywhere in comp/: Studio edits usually land in compositions/*.html, not index.html.
		var v int64
		filepath.WalkDir(filepath.Join(dir, "comp"), func(p string, d fs.DirEntry, err error) error {
			if err == nil && d.IsDir() && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			if err == nil && strings.HasSuffix(p, ".html") {
				if info, err := d.Info(); err == nil && info.ModTime().UnixNano() > v {
					v = info.ModTime().UnixNano()
				}
			}
			return nil
		})
		p["composition"] = fmt.Sprintf("%s/comp/index.html?v=%d", base, v)
	}
	p["generated"] = generatedFiles(id, base)
	p["comp_assets"] = listFiles(filepath.Join(dir, "comp", "assets"), base+"/comp/assets", true)
	p["quote"] = quote(p)
	p["running"] = isRunning(id)
	p["studio_port"] = studioPort(id)
	p["render"] = getRenderProgress(id)
	p["posts"] = listPosts(id)
	p["social"] = socialCaptions(id)
	return p, nil
}

func saveProject(r *http.Request) (any, error) {
	b, err := body(r)
	if err != nil {
		return nil, err
	}
	id := pathID(r)
	if id == 0 {
		if strings.TrimSpace(str(b["title"])) == "" || b["tenant_id"] == nil {
			return nil, bad("company and title are required")
		}
		if trialLeft() == 0 {
			if err := needLicense(fmt.Sprintf("Your free trial covers %d videos. Making more", trialProjects)); err != nil {
				return nil, err
			}
		}
		p := presetByID(str(b["kind"]))
		if b["aspect"] == nil {
			b["aspect"] = p.Aspect
		}
		if b["seconds"] == nil {
			b["seconds"] = p.Seconds
		}
	} else {
		delete(b, "tenant_id")
	}
	isNew := id == 0
	if id, err = save("projects", projectCols, id, b); err != nil {
		return nil, err
	}
	if isNew {
		db.Exec("insert into settings(key,value) values('projects_made','1') on conflict(key) do update set value=cast(value as integer)+1")
	}
	for _, d := range []string{"input", "work", "out/drafts"} {
		os.MkdirAll(filepath.Join(projectDir(id), d), 0o755)
	}
	return row(projectSelect+" where p.id=?", id)
}

func deleteProject(r *http.Request) (any, error) {
	id := pathID(r)
	if isRunning(id) {
		return nil, bad("stop the agent first")
	}
	// Usage rows stay: the money was spent, cost reports must still see it.
	db.Exec("delete from events where project_id=?", id)
	db.Exec("delete from posts where project_id=? and status!='posting'", id)
	if _, err := db.Exec("delete from projects where id=?", id); err != nil {
		return nil, err
	}
	return nil, os.RemoveAll(projectDir(id))
}

func uploadInputs(r *http.Request) (any, error) {
	return saveUploads(r, filepath.Join(projectDir(pathID(r)), "input"))
}

// uploadAttachments stores images pasted into the agent chat; messages reference them as [image: chat/<file>].
func uploadAttachments(r *http.Request) (any, error) {
	return saveUploads(r, filepath.Join(projectDir(pathID(r)), "chat"))
}

func deleteProjectFile(r *http.Request) (any, error) {
	p := r.URL.Query().Get("path")
	if !strings.HasPrefix(p, "out/") && !strings.HasPrefix(p, "input/") {
		return nil, bad("only input/ and out/ files can be deleted")
	}
	return nil, os.Remove(within(projectDir(pathID(r)), p))
}

func listEvents(r *http.Request) (any, error) {
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	return rows("select id,kind,body,created_at from events where project_id=? and id>? order by id limit 500", pathID(r), after)
}

// ---- files

type fileInfo struct {
	Path    string    `json:"path"`
	Name    string    `json:"name"`
	URL     string    `json:"url"`
	Kind    string    `json:"kind"`
	Size    int64     `json:"size"`
	Seconds float64   `json:"seconds,omitempty"`
	Mod     time.Time `json:"mod"`
}

func listFiles(root, urlBase string, recursive bool) []fileInfo {
	out := []fileInfo{}
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
		f := fileInfo{Path: rel, Name: d.Name(), URL: urlBase + "/" + rel, Kind: kindOf(p), Size: info.Size(), Mod: info.ModTime()}
		if f.Kind == "video" || f.Kind == "audio" {
			f.Seconds = mediaSeconds(p)
		}
		out = append(out, f)
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Mod.After(out[j].Mod) })
	return out
}

func kindOf(name string) string {
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

func mediaSeconds(p string) float64 {
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

// saveUploads streams multipart files to disk; raw lectures can be several GB, so nothing is buffered in memory.
func saveUploads(r *http.Request, dir string) (any, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, bad("expected multipart upload")
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
		name := safeName(part.FileName())
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

func safeName(s string) string {
	s = path.Base(strings.ReplaceAll(s, `\`, "/"))
	s = strings.TrimLeft(unsafeChars.ReplaceAllString(s, "_"), ". ")
	if s == "" {
		return "file"
	}
	return s
}

// within joins rel onto root without letting it escape root.
func within(root, rel string) string {
	return filepath.Join(root, filepath.FromSlash(path.Clean("/"+rel)))
}

var headTag = regexp.MustCompile(`(?i)<head[^>]*>`)

// filesHandler serves tenant assets and project files. A project's comp/index.html gets the
// HyperFrames runtime injected, same as `hyperframes play` does, so <hyperframes-player> can drive it.
func filesHandler() http.Handler {
	fsrv := http.FileServer(http.Dir(dataDir))
	runtime := fmt.Sprintf(`<script src="https://cdn.jsdelivr.net/npm/@hyperframes/core@%s/dist/hyperframe.runtime.iife.js"></script>`, hfVersion)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := path.Clean("/" + r.URL.Path)
		if !strings.HasPrefix(p, "/tenants/") && !strings.HasPrefix(p, "/projects/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		if strings.HasPrefix(p, "/projects/") && strings.HasSuffix(p, "/comp/index.html") {
			b, err := os.ReadFile(filepath.Join(dataDir, filepath.FromSlash(p)))
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

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// generatedFiles lists what Gemini and ElevenLabs made for a project (newest first), from the usage ledger.
func generatedFiles(id int64, base string) []map[string]any {
	us, _ := rows("select provider, item, file, cost from usage where project_id=? and file!='' order by id desc", id)
	out := []map[string]any{}
	for _, u := range us {
		rel := str(u["file"])
		st, err := os.Stat(within(projectDir(id), rel))
		if err != nil {
			continue // deleted since
		}
		out = append(out, map[string]any{"path": rel, "name": path.Base(rel), "url": base + "/" + rel, "kind": kindOf(rel),
			"size": st.Size(), "provider": u["provider"], "item": u["item"], "cost": u["cost"], "seconds": mediaSeconds(within(projectDir(id), rel))})
	}
	return out
}
