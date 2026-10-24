// Package brand holds brands (tenants in the database): their profile and brand library, the website and folder scan, and campaigns.
package company

import (
	"net/http"

	"github.com/Shaitrish/37studio/internal/agent"
	"github.com/Shaitrish/37studio/internal/costs"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/store"
)

const campaignSelect = `select c.*, t.name as tenant_name, (select count(*) from projects p where p.campaign_id=c.id) as video_count
  from campaigns c join tenants t on t.id=c.tenant_id`

func ListCampaigns(r *http.Request) (any, error) {
	if t := r.URL.Query().Get("tenant"); t != "" {
		return store.Rows(campaignSelect+" where c.tenant_id=? order by c.id desc", t)
	}
	return store.Rows(campaignSelect + " order by c.id desc")
}

func GetCampaign(r *http.Request) (any, error) {
	id := httpapi.PathID(r)
	c, err := store.Row(campaignSelect+" where c.id=?", id)
	if err != nil {
		return nil, err
	}
	ps, _ := store.Rows(store.ProjectSelect+" where p.campaign_id=? order by p.id desc", id)
	var cost float64
	for _, p := range ps {
		q := costs.Quote(p)
		p["quote"], p["running"] = q, agent.IsRunning(p["id"].(int64))
		cost += q["cost"].(float64)
	}
	c["projects"], c["cost"] = ps, costs.Round2(cost)
	return c, nil
}

func SaveCampaign(r *http.Request) (any, error) {
	b, err := httpapi.Body(r)
	if err != nil {
		return nil, err
	}
	id := httpapi.PathID(r)
	if id == 0 && (b["tenant_id"] == nil || store.Str(b["name"]) == "") {
		return nil, httpapi.Bad("brand and campaign name are required")
	}
	if id != 0 {
		delete(b, "tenant_id")
	}
	if id, err = store.Save("campaigns", []string{"tenant_id", "name", "brief"}, id, b); err != nil {
		return nil, err
	}
	return store.Row(campaignSelect+" where c.id=?", id)
}

// DeleteCampaign keeps the videos; they just stop belonging to a campaign.
func DeleteCampaign(r *http.Request) (any, error) {
	id := httpapi.PathID(r)
	if _, err := store.DB.Exec("update projects set campaign_id=null where campaign_id=?", id); err != nil {
		return nil, err
	}
	_, err := store.DB.Exec("delete from campaigns where id=?", id)
	return nil, err
}
