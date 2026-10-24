package project

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Shaitrish/37studio/internal/agent"
	"github.com/Shaitrish/37studio/internal/costs"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/social"
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/templates"
	"github.com/Shaitrish/37studio/internal/timeline"
	"github.com/Shaitrish/37studio/internal/transcribe"
)

func List(r *http.Request) (any, error) {
	q, args := store.ProjectSelect+" where 1=1", []any{}
	if t := r.URL.Query().Get("tenant"); t != "" {
		q, args = q+" and p.tenant_id=?", append(args, t)
	}
	if c := r.URL.Query().Get("campaign"); c != "" {
		q, args = q+" and p.campaign_id=?", append(args, c)
	}
	ps, err := store.Rows(q+" order by p.id desc", args...)
	for _, p := range ps {
		p["quote"] = costs.Quote(p)
		p["running"] = agent.IsRunning(p["id"].(int64))
	}
	return ps, err
}

func Get(r *http.Request) (any, error) {
	id := httpapi.PathID(r)
	p, err := store.Row(store.ProjectSelect+" where p.id=?", id)
	if err != nil {
		return nil, err
	}
	dir, base := files.ProjectDir(id), fmt.Sprintf("/files/projects/%d", id)
	p["inputs"] = files.List(filepath.Join(dir, "input"), base+"/input", true)
	p["outputs"] = files.List(filepath.Join(dir, "out"), base+"/out", false)
	p["drafts"] = files.List(filepath.Join(dir, "out", "drafts"), base+"/out/drafts", false) // not recursive: renders leave scratch dirs here
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
	p["children"], _ = store.Rows("select id,title,status,clip_stage from projects where parent_id=? order by id", id)
	if parent := p["parent_id"]; parent != nil {
		if t, err := store.Row("select title from projects where id=?", parent); err == nil {
			p["parent_title"] = t["title"]
		}
	}
	p["transcript"] = transcribe.State(id)
	p["import"] = importState(id)
	p["generated"] = generatedFiles(id, base)
	p["comp_assets"] = files.List(filepath.Join(dir, "comp", "assets"), base+"/comp/assets", true)
	p["quote"] = costs.Quote(p)
	p["running"] = agent.IsRunning(id)
	p["studio_port"] = timeline.Port(id)
	p["render"] = agent.GetRenderProgress(id)
	p["posts"] = social.ListPosts(id)
	p["social"] = social.Captions(id)
	return p, nil
}

func Save(r *http.Request) (any, error) {
	b, err := httpapi.Body(r)
	if err != nil {
		return nil, err
	}
	id := httpapi.PathID(r)
	if id == 0 {
		if strings.TrimSpace(store.Str(b["title"])) == "" || b["tenant_id"] == nil {
			return nil, httpapi.Bad("brand and title are required")
		}
		p := agent.PresetByID(store.Str(b["kind"]))
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
	if id, err = store.Save("projects", store.ProjectCols, id, b); err != nil {
		return nil, err
	}
	for _, d := range []string{"input", "work", "out/drafts"} {
		os.MkdirAll(filepath.Join(files.ProjectDir(id), d), 0o755)
	}
	if isNew {
		if err := templates.Use(id, b["template_id"]); err != nil {
			return nil, err
		}
	}
	return store.Row(store.ProjectSelect+" where p.id=?", id)
}

func Delete(r *http.Request) (any, error) {
	id := httpapi.PathID(r)
	if agent.IsRunning(id) {
		return nil, httpapi.Bad("stop the agent first")
	}
	// Usage rows stay: the money was spent, cost reports must still see it.
	store.DB.Exec("delete from events where project_id=?", id)
	store.DB.Exec("delete from posts where project_id=? and status!='posting'", id)
	if _, err := store.DB.Exec("delete from projects where id=?", id); err != nil {
		return nil, err
	}
	return nil, os.RemoveAll(files.ProjectDir(id))
}

func UploadInputs(r *http.Request) (any, error) {
	return files.SaveUploads(r, filepath.Join(files.ProjectDir(httpapi.PathID(r)), "input"))
}

// UploadAttachments stores images pasted into the agent chat; messages reference them as [image: chat/<file>].
func UploadAttachments(r *http.Request) (any, error) {
	return files.SaveUploads(r, filepath.Join(files.ProjectDir(httpapi.PathID(r)), "chat"))
}

func DeleteProjectFile(r *http.Request) (any, error) {
	p := r.URL.Query().Get("path")
	if !strings.HasPrefix(p, "out/") && !strings.HasPrefix(p, "input/") {
		return nil, httpapi.Bad("only input/ and out/ files can be deleted")
	}
	return nil, os.Remove(files.Within(files.ProjectDir(httpapi.PathID(r)), p))
}

func ListEvents(r *http.Request) (any, error) {
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	return store.Rows("select id,kind,body,created_at from events where project_id=? and id>? order by id limit 500", httpapi.PathID(r), after)
}

// generatedFiles lists what Gemini and ElevenLabs made for a project (newest first), from the usage ledger.
func generatedFiles(id int64, base string) []map[string]any {
	us, _ := store.Rows("select provider, item, file, cost from usage where project_id=? and file!='' order by id desc", id)
	out := []map[string]any{}
	for _, u := range us {
		rel := store.Str(u["file"])
		st, err := os.Stat(files.Within(files.ProjectDir(id), rel))
		if err != nil {
			continue // deleted since
		}
		out = append(out, map[string]any{"path": rel, "name": path.Base(rel), "url": base + "/" + rel, "kind": files.Kind(rel),
			"size": st.Size(), "provider": u["provider"], "item": u["item"], "cost": u["cost"], "seconds": files.MediaSeconds(files.Within(files.ProjectDir(id), rel))})
	}
	return out
}
