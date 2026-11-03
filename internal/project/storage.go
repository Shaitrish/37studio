package project

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/store"
)

// postWaits says whether a post still waiting to go out uses this project file; deleting it would fail that upload.
func postWaits(id int64, rel string) bool {
	var n int
	store.DB.QueryRow("select count(*) from posts where project_id=? and file=? and status in ('approval','scheduled','posting')", id, rel).Scan(&n)
	return n > 0
}

// Storage is the disk space each project and brand takes, biggest first, and what is left on the disk.
// A file hard-linked into several projects (one recording cut into many videos) counts once, where it is first seen.
func Storage(r *http.Request) (any, error) {
	seen := map[[2]uint64]bool{}
	size := func(root string, bucket func(rel string) string) map[string]int64 {
		out := map[string]int64{}
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if k, shared := files.LinkKey(p, info); shared {
				if seen[k] {
					return nil
				}
				seen[k] = true
			}
			rel, _ := filepath.Rel(root, p)
			out[bucket(filepath.ToSlash(rel))] += info.Size()
			return nil
		})
		return out
	}

	ps, err := store.Rows("select p.id, p.title, p.tenant_id, t.name as tenant_name from projects p join tenants t on t.id=p.tenant_id order by p.id")
	if err != nil {
		return nil, err
	}
	var total int64
	for _, p := range ps {
		b := size(files.ProjectDir(p["id"].(int64)), func(rel string) string {
			switch {
			case strings.HasPrefix(rel, "out/drafts/"):
				return "drafts"
			case strings.HasPrefix(rel, "out/"):
				return "finals"
			case strings.HasPrefix(rel, "input/"):
				return "sources"
			}
			return "other"
		})
		p["finals"], p["drafts"], p["sources"], p["other"] = b["finals"], b["drafts"], b["sources"], b["other"]
		p["total"] = b["finals"] + b["drafts"] + b["sources"] + b["other"]
		total += p["total"].(int64)
	}
	sort.SliceStable(ps, func(i, j int) bool { return ps[i]["total"].(int64) > ps[j]["total"].(int64) })

	ts, err := store.Rows("select id, name from tenants order by name")
	for _, t := range ts {
		t["total"] = size(files.TenantRoot(t["id"].(int64)), func(string) string { return "" })[""]
		total += t["total"].(int64)
	}
	free := files.FreeSpace(config.DataDir)
	return map[string]any{"total": total, "free": free, "projects": ps, "brands": ts}, err
}

// CleanDrafts deletes draft renders and generated media older than the given number of days, from every project
// or one. Finals, sources and files a waiting post needs stay.
func CleanDrafts(r *http.Request) (any, error) {
	b, err := httpapi.Body(r)
	if err != nil {
		return nil, err
	}
	days := store.Num(b["days"])
	if days < 1 {
		return nil, httpapi.Bad("days must be 1 or more, so a render running right now is never touched")
	}
	q, args := "select id from projects", []any{}
	if pid := int64(store.Num(b["project_id"])); pid > 0 {
		q, args = q+" where id=?", append(args, pid)
	}
	ps, err := store.Rows(q, args...)
	if err != nil {
		return nil, err
	}
	cutoff := time.Now().Add(-time.Duration(days * 24 * float64(time.Hour)))
	var n int
	var freed int64
	for _, p := range ps {
		id := p["id"].(int64)
		for _, f := range files.List(filepath.Join(files.ProjectDir(id), "out", "drafts"), "", true) {
			if f.Mod.After(cutoff) || postWaits(id, "out/drafts/"+f.Path) {
				continue
			}
			if os.Remove(filepath.Join(files.ProjectDir(id), "out", "drafts", filepath.FromSlash(f.Path))) == nil {
				n, freed = n+1, freed+f.Size
			}
		}
	}
	return map[string]any{"files": n, "bytes": freed}, nil
}
