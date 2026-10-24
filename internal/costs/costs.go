// Package costs records what AI providers charge and reports it per project, brand, provider and month.
package costs

import (
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/store"
)

func Round2(x float64) float64 { return math.Round(x*100) / 100 }

// Quote is what a project has cost so far: Claude's own USD figure per agent run plus each Gemini and ElevenLabs
// generation at the unit prices in Settings. Rendering and storage run on the user's own machine, so they are free.
func Quote(p map[string]any) map[string]any {
	lines := []map[string]any{}
	var cost float64
	us, _ := store.Rows("select provider, item, count(*) n, sum(quantity) qty, sum(cost) cost from usage where project_id=? group by provider, item", p["id"])
	for _, u := range us {
		c, qty := store.Num(u["cost"]), store.Num(u["qty"])
		label, detail := "Gemini "+store.Str(u["item"]), fmt.Sprintf("%v requests", u["n"])
		if u["provider"] == "elevenlabs" {
			label = "ElevenLabs " + store.Str(u["item"])
		}
		if u["provider"] == "fal" {
			label = "fal.ai " + store.Str(u["item"])
		}
		if u["provider"] == "anthropic" || u["provider"] == "opencode" || u["provider"] == "codex" {
			name := map[string]string{"anthropic": "Claude", "opencode": "OpenCode", "codex": "Codex"}[store.Str(u["provider"])]
			label, detail = name+" agent ("+store.Str(u["item"])+")", fmt.Sprintf("%v runs, %.1fM tokens", u["n"], qty/1e6)
		} else if store.Str(u["item"]) == store.Setting("video_model") {
			detail = fmt.Sprintf("%.0f s of video", qty)
		}
		lines = append(lines, map[string]any{"label": label, "detail": detail, "cost": c})
		cost += c
	}
	return map[string]any{"lines": lines, "cost": Round2(cost)}
}

// Report is the spend report: by provider, by brand and by month, plus the total.
func Report(r *http.Request) (any, error) {
	byProvider, err := store.Rows("select provider, item, count(*) n, sum(quantity) qty, sum(cost) cost from usage group by provider, item order by cost desc")
	if err != nil {
		return nil, err
	}
	byTenant, _ := store.Rows("select u.tenant_id, case when u.tenant_id=0 then 'Assistant' else coalesce(t.name,'(deleted)') end name, sum(u.cost) cost, count(distinct nullif(u.project_id,0)) projects from usage u left join tenants t on t.id=u.tenant_id group by u.tenant_id order by cost desc")
	byMonth, _ := store.Rows("select substr(created_at,1,7) month, provider, sum(cost) cost from usage group by 1,2 order by 1")
	var total, month float64
	store.DB.QueryRow("select coalesce(sum(cost),0), coalesce(sum(case when created_at >= date('now','start of month') then cost end),0) from usage").Scan(&total, &month)
	return map[string]any{"by_provider": byProvider, "by_tenant": byTenant, "by_month": byMonth, "total": Round2(total), "this_month": Round2(month)}, nil
}

// Record books a generation's cost on the project and remembers its file, so the UI can list what was generated.
func Record(provider string, pid, tenant int64, item string, qty, cost float64, file string) {
	if pid <= 0 {
		return
	}
	if rel, err := filepath.Rel(files.ProjectDir(pid), file); err == nil && !strings.HasPrefix(rel, "..") {
		file = filepath.ToSlash(rel)
	}
	store.DB.Exec("insert into usage(project_id,tenant_id,provider,item,quantity,cost,file) values(?,?,?,?,?,?,?)", pid, tenant, provider, item, qty, cost, file)
}
