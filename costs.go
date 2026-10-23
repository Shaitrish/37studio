package main

import (
	"fmt"
	"math"
	"net/http"
)

func round2(x float64) float64 { return math.Round(x*100) / 100 }

// quote is what a project has cost so far: Claude's own USD figure per agent run plus each Gemini and ElevenLabs
// generation at the unit prices in Settings. Rendering and storage run on the user's own machine, so they are free.
func quote(p map[string]any) map[string]any {
	lines := []map[string]any{}
	var cost float64
	us, _ := rows("select provider, item, count(*) n, sum(quantity) qty, sum(cost) cost from usage where project_id=? group by provider, item", p["id"])
	for _, u := range us {
		c, qty := num(u["cost"]), num(u["qty"])
		label, detail := "Gemini "+str(u["item"]), fmt.Sprintf("%v requests", u["n"])
		if u["provider"] == "elevenlabs" {
			label = "ElevenLabs " + str(u["item"])
		}
		if u["provider"] == "anthropic" || u["provider"] == "opencode" {
			name := map[string]string{"anthropic": "Claude", "opencode": "OpenCode"}[str(u["provider"])]
			label, detail = name+" agent ("+str(u["item"])+")", fmt.Sprintf("%v runs, %.1fM tokens", u["n"], qty/1e6)
		} else if str(u["item"]) == setting("video_model") {
			detail = fmt.Sprintf("%.0f s of video", qty)
		}
		lines = append(lines, map[string]any{"label": label, "detail": detail, "cost": c})
		cost += c
	}
	return map[string]any{"lines": lines, "cost": round2(cost)}
}

// getCosts is the spend report: by provider, by company and by month, plus the total.
func getCosts(r *http.Request) (any, error) {
	byProvider, err := rows("select provider, item, count(*) n, sum(quantity) qty, sum(cost) cost from usage group by provider, item order by cost desc")
	if err != nil {
		return nil, err
	}
	byTenant, _ := rows("select u.tenant_id, coalesce(t.name,'(deleted)') name, sum(u.cost) cost, count(distinct nullif(u.project_id,0)) projects from usage u left join tenants t on t.id=u.tenant_id group by u.tenant_id order by cost desc")
	byMonth, _ := rows("select substr(created_at,1,7) month, provider, sum(cost) cost from usage group by 1,2 order by 1")
	var total, month float64
	db.QueryRow("select coalesce(sum(cost),0), coalesce(sum(case when created_at >= date('now','start of month') then cost end),0) from usage").Scan(&total, &month)
	return map[string]any{"by_provider": byProvider, "by_tenant": byTenant, "by_month": byMonth, "total": round2(total), "this_month": round2(month)}, nil
}
