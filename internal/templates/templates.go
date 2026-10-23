// Package templates saves a finished project's composition and seeds new projects from it.
package templates

import (
	"cmp"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/store"
)

func Dir(id int64) string { return filepath.Join(config.DataDir, "templates", fmt.Sprint(id)) }

// copyTree copies src into dst, skipping dot files and folders (HyperFrames history belongs to the old project).
// Copies, not hard links: the agent edits files in place, and a template must not change under it.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if rel != "." && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		return files.Copy(p, filepath.Join(dst, rel))
	})
}

func List(r *http.Request) (any, error) {
	q, args := "select m.*, t.name as tenant_name from templates m left join tenants t on t.id=m.tenant_id", []any{}
	if t := r.URL.Query().Get("tenant"); t != "" {
		q, args = q+" where m.tenant_id is null or m.tenant_id=?", append(args, t)
	}
	ts, err := store.Rows(q+" order by m.id desc", args...)
	for _, t := range ts {
		if _, err := os.Stat(filepath.Join(Dir(t["id"].(int64)), "thumb.jpg")); err == nil {
			t["thumb"] = fmt.Sprintf("/files/templates/%d/thumb.jpg", t["id"])
		}
	}
	return ts, err
}

// Save is POST /api/projects/{id}/template {"name", "notes", "shared"}: shared templates show for every company.
func Save(r *http.Request) (any, error) {
	b, err := httpapi.Body(r)
	if err != nil {
		return nil, err
	}
	id := httpapi.PathID(r)
	p, err := store.Row("select * from projects where id=?", id)
	if err != nil {
		return nil, err
	}
	comp := filepath.Join(files.ProjectDir(id), "comp")
	if _, err := os.Stat(filepath.Join(comp, "index.html")); err != nil {
		return nil, httpapi.Bad("this project has no composition yet")
	}
	var tenant any = p["tenant_id"]
	if b["shared"] == true {
		tenant = nil
	}
	res, err := store.DB.Exec("insert into templates(name,notes,tenant_id,project_id,kind,aspect,seconds) values(?,?,?,?,?,?,?)",
		cmp.Or(strings.TrimSpace(store.Str(b["name"])), store.Str(p["title"])), strings.TrimSpace(store.Str(b["notes"])), tenant, id, p["kind"], p["aspect"], p["seconds"])
	if err != nil {
		return nil, err
	}
	tid, _ := res.LastInsertId()
	if err := copyTree(comp, filepath.Join(Dir(tid), "comp")); err != nil {
		store.DB.Exec("delete from templates where id=?", tid)
		os.RemoveAll(Dir(tid))
		return nil, err
	}
	for _, f := range files.List(filepath.Join(files.ProjectDir(id), "out"), "", false) { // newest export first
		if f.Kind == "video" {
			exec.Command("ffmpeg", "-y", "-v", "error", "-ss", "1", "-i", filepath.Join(files.ProjectDir(id), "out", f.Path),
				"-frames:v", "1", "-vf", "scale=480:-2", filepath.Join(Dir(tid), "thumb.jpg")).Run()
			break
		}
	}
	return store.Row("select * from templates where id=?", tid)
}

func Delete(r *http.Request) (any, error) {
	id := httpapi.PathID(r)
	if _, err := store.DB.Exec("delete from templates where id=?", id); err != nil {
		return nil, err
	}
	store.DB.Exec("update projects set template_id=null where template_id=?", id)
	return nil, os.RemoveAll(Dir(id))
}

// Use puts a copy of the template's composition in a new project's comp/; the first prompt says what to keep.
func Use(pid int64, tid any) error {
	if tid == nil || store.Num(tid) == 0 {
		return nil
	}
	t, err := store.Row("select id from templates where id=?", tid)
	if err != nil {
		return httpapi.Bad("that template no longer exists")
	}
	return copyTree(filepath.Join(Dir(t["id"].(int64)), "comp"), filepath.Join(files.ProjectDir(pid), "comp"))
}

// Text is the first prompt's note for a project started from a template.
func Text(p map[string]any) string {
	if p["template_id"] == nil {
		return ""
	}
	t, err := store.Row("select * from templates where id=?", p["template_id"])
	if err != nil {
		return ""
	}
	note := fmt.Sprintf(`
Template: comp/ already holds the template %q (from an earlier video), so don't run hyperframes init. Keep its design: layout, fonts, colors, motion, caption style, intro and outro, pacing. Replace its content (footage, text, numbers, voiceover, and music if the brief asks) with this job's, and remove anything left from the old video that doesn't fit. Check it with hyperframes check before the first summary.
`, store.Str(t["name"]))
	if n := store.Str(t["notes"]); n != "" {
		note += "What the editor wants kept: " + n + "\n"
	}
	return note
}
