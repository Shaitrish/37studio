package company

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Shaitrish/37studio/internal/agent"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/source"
	"github.com/Shaitrish/37studio/internal/store"
)

func List(r *http.Request) (any, error) {
	rs, err := store.Rows(`select t.*,
	  (select count(*) from projects p where p.tenant_id=t.id) as project_count,
	  (select coalesce(sum(cost),0) from usage u where u.tenant_id=t.id) as cost_usd
	  from tenants t order by t.name collate nocase`)
	for _, t := range rs { // the newest logo image, for the card; the whole library only comes with Get
		id := t["id"].(int64)
		for _, f := range files.List(filepath.Join(files.TenantDir(id), "logo"), fmt.Sprintf("/files/tenants/%d/assets/logo", id), false) {
			if f.Kind == "image" {
				t["logo"] = f.URL
				break
			}
		}
	}
	return rs, err
}

func Get(r *http.Request) (any, error) {
	id := httpapi.PathID(r)
	t, err := store.Row("select * from tenants where id=?", id)
	if err != nil {
		return nil, err
	}
	t["assets"] = files.List(files.TenantDir(id), fmt.Sprintf("/files/tenants/%d/assets", id), true)
	t["asset_meta"] = files.AssetMeta(id)
	for k, f := range files.TenantDocs {
		t[k] = files.TenantDoc(id, f)
	}
	t["source"] = source.Info(id)
	t["scanning"] = agent.IsRunning(-id)
	t["projects"], _ = store.Rows("select id,title,kind,status,created_at from projects where tenant_id=? order by id desc", id)
	return t, nil
}

func Save(r *http.Request) (any, error) {
	b, err := httpapi.Body(r)
	if err != nil {
		return nil, err
	}
	id := httpapi.PathID(r)
	if id == 0 && strings.TrimSpace(store.Str(b["name"])) == "" {
		return nil, httpapi.Bad("name is required")
	}
	if id, err = store.Save("tenants", store.TenantCols, id, b); err != nil {
		return nil, err
	}
	for _, d := range files.AssetDirs {
		os.MkdirAll(filepath.Join(files.TenantDir(id), d), 0o755)
	}
	for k, f := range files.TenantDocs {
		if v, ok := b[k].(string); ok {
			if err := os.WriteFile(filepath.Join(files.TenantRoot(id), f), []byte(v), 0o644); err != nil {
				return nil, err
			}
		}
	}
	return store.Row("select * from tenants where id=?", id)
}

func Delete(r *http.Request) (any, error) {
	id := httpapi.PathID(r)
	var n int
	store.DB.QueryRow("select count(*) from projects where tenant_id=?", id).Scan(&n)
	if n > 0 {
		return nil, httpapi.Bad("this brand still has projects; delete those first")
	}
	if _, err := store.DB.Exec("delete from tenants where id=?", id); err != nil {
		return nil, err
	}
	return nil, os.RemoveAll(files.TenantRoot(id))
}

func UploadAssets(r *http.Request) (any, error) {
	folder := r.URL.Query().Get("folder")
	if !slices.Contains(files.AssetDirs, folder) {
		return nil, httpapi.Bad("unknown folder")
	}
	return files.SaveUploads(r, filepath.Join(files.TenantDir(httpapi.PathID(r)), folder))
}

func DeleteAsset(r *http.Request) (any, error) {
	id, p := httpapi.PathID(r), r.URL.Query().Get("path")
	if err := os.Remove(files.Within(files.TenantDir(id), p)); err != nil {
		return nil, err
	}
	meta := files.AssetMeta(id)
	delete(meta, p)
	return nil, files.SaveAssetMeta(id, meta)
}

// UpdateAsset edits one asset: {path, caption?, use?, section?, context?, source?} sets its metadata, {path, folder?} moves it to another folder.
func UpdateAsset(r *http.Request) (any, error) {
	b, err := httpapi.Body(r)
	if err != nil {
		return nil, err
	}
	id, p := httpapi.PathID(r), store.Str(b["path"])
	meta := files.AssetMeta(id)
	if _, err := os.Stat(files.Within(files.TenantDir(id), p)); err != nil {
		return nil, httpapi.Bad("no such asset")
	}
	for _, k := range []string{"caption", "use", "section", "context", "source"} {
		if v, ok := b[k].(string); ok {
			if meta[p] == nil {
				meta[p] = map[string]any{}
			}
			meta[p][k] = strings.TrimSpace(v)
		}
	}
	if f := store.Str(b["folder"]); f != "" {
		if !slices.Contains(files.AssetDirs, f) {
			return nil, httpapi.Bad("unknown folder")
		}
		np := f + "/" + path.Base(p)
		if _, err := os.Stat(files.Within(files.TenantDir(id), np)); err == nil {
			return nil, httpapi.Bad("a file with that name already exists there")
		}
		os.MkdirAll(filepath.Join(files.TenantDir(id), f), 0o755)
		if err := os.Rename(files.Within(files.TenantDir(id), p), files.Within(files.TenantDir(id), np)); err != nil {
			return nil, err
		}
		if meta[p] != nil {
			meta[np] = meta[p]
			delete(meta, p)
		}
	}
	return nil, files.SaveAssetMeta(id, meta)
}
