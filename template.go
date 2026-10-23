package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Templates: a finished project's composition (comp/), saved so later videos start from the same look: layout, fonts,
// motion, caption style, intro and outro. A new project from a template gets a copy in its comp/ before the agent
// starts, and the agent swaps in the new content. Stored in templates/<id>/ with a still from the exported video.

func templateDir(id int64) string { return filepath.Join(dataDir, "templates", fmt.Sprint(id)) }

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
		return copyFile(p, filepath.Join(dst, rel))
	})
}

func listTemplates(r *http.Request) (any, error) {
	q, args := "select m.*, t.name as tenant_name from templates m left join tenants t on t.id=m.tenant_id", []any{}
	if t := r.URL.Query().Get("tenant"); t != "" {
		q, args = q+" where m.tenant_id is null or m.tenant_id=?", append(args, t)
	}
	ts, err := rows(q+" order by m.id desc", args...)
	for _, t := range ts {
		if _, err := os.Stat(filepath.Join(templateDir(t["id"].(int64)), "thumb.jpg")); err == nil {
			t["thumb"] = fmt.Sprintf("/files/templates/%d/thumb.jpg", t["id"])
		}
	}
	return ts, err
}

// saveTemplate is POST /api/projects/{id}/template {"name", "notes", "shared"}: shared templates show for every company.
func saveTemplate(r *http.Request) (any, error) {
	b, err := body(r)
	if err != nil {
		return nil, err
	}
	id := pathID(r)
	p, err := row("select * from projects where id=?", id)
	if err != nil {
		return nil, err
	}
	comp := filepath.Join(projectDir(id), "comp")
	if _, err := os.Stat(filepath.Join(comp, "index.html")); err != nil {
		return nil, bad("this project has no composition yet")
	}
	var tenant any = p["tenant_id"]
	if b["shared"] == true {
		tenant = nil
	}
	res, err := db.Exec("insert into templates(name,notes,tenant_id,project_id,kind,aspect,seconds) values(?,?,?,?,?,?,?)",
		or(strings.TrimSpace(str(b["name"])), str(p["title"])), strings.TrimSpace(str(b["notes"])), tenant, id, p["kind"], p["aspect"], p["seconds"])
	if err != nil {
		return nil, err
	}
	tid, _ := res.LastInsertId()
	if err := copyTree(comp, filepath.Join(templateDir(tid), "comp")); err != nil {
		db.Exec("delete from templates where id=?", tid)
		os.RemoveAll(templateDir(tid))
		return nil, err
	}
	for _, f := range listFiles(filepath.Join(projectDir(id), "out"), "", false) { // newest export first
		if f.Kind == "video" {
			exec.Command("ffmpeg", "-y", "-v", "error", "-ss", "1", "-i", filepath.Join(projectDir(id), "out", f.Path),
				"-frames:v", "1", "-vf", "scale=480:-2", filepath.Join(templateDir(tid), "thumb.jpg")).Run()
			break
		}
	}
	return row("select * from templates where id=?", tid)
}

func deleteTemplate(r *http.Request) (any, error) {
	id := pathID(r)
	if _, err := db.Exec("delete from templates where id=?", id); err != nil {
		return nil, err
	}
	db.Exec("update projects set template_id=null where template_id=?", id)
	return nil, os.RemoveAll(templateDir(id))
}

// useTemplate puts a copy of the template's composition in a new project's comp/; the first prompt says what to keep.
func useTemplate(pid int64, tid any) error {
	if tid == nil || num(tid) == 0 {
		return nil
	}
	t, err := row("select id from templates where id=?", tid)
	if err != nil {
		return bad("that template no longer exists")
	}
	return copyTree(filepath.Join(templateDir(t["id"].(int64)), "comp"), filepath.Join(projectDir(pid), "comp"))
}

// templateText is the first prompt's note for a project started from a template.
func templateText(p map[string]any) string {
	if p["template_id"] == nil {
		return ""
	}
	t, err := row("select * from templates where id=?", p["template_id"])
	if err != nil {
		return ""
	}
	note := fmt.Sprintf(`
Template: comp/ already holds the template %q (from an earlier video), so don't run hyperframes init. Keep its design: layout, fonts, colors, motion, caption style, intro and outro, pacing. Replace its content (footage, text, numbers, voiceover, and music if the brief asks) with this job's, and remove anything left from the old video that doesn't fit. Check it with hyperframes check before the first summary.
`, str(t["name"]))
	if n := str(t["notes"]); n != "" {
		note += "What the editor wants kept: " + n + "\n"
	}
	return note
}
