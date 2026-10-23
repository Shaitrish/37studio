package main

import "net/http"

// Campaigns group a company's videos around one product, launch or offer. Their brief reaches every
// video's agent through the harness.

const campaignSelect = `select c.*, t.name as tenant_name, (select count(*) from projects p where p.campaign_id=c.id) as video_count
  from campaigns c join tenants t on t.id=c.tenant_id`

func listCampaigns(r *http.Request) (any, error) {
	if t := r.URL.Query().Get("tenant"); t != "" {
		return rows(campaignSelect+" where c.tenant_id=? order by c.id desc", t)
	}
	return rows(campaignSelect + " order by c.id desc")
}

func getCampaign(r *http.Request) (any, error) {
	id := pathID(r)
	c, err := row(campaignSelect+" where c.id=?", id)
	if err != nil {
		return nil, err
	}
	ps, _ := rows(projectSelect+" where p.campaign_id=? order by p.id desc", id)
	var cost float64
	for _, p := range ps {
		q := quote(p)
		p["quote"], p["running"] = q, isRunning(p["id"].(int64))
		cost += q["cost"].(float64)
	}
	c["projects"], c["cost"] = ps, round2(cost)
	return c, nil
}

func saveCampaign(r *http.Request) (any, error) {
	b, err := body(r)
	if err != nil {
		return nil, err
	}
	id := pathID(r)
	if id == 0 && (b["tenant_id"] == nil || str(b["name"]) == "") {
		return nil, bad("company and campaign name are required")
	}
	if id != 0 {
		delete(b, "tenant_id")
	}
	if id, err = save("campaigns", []string{"tenant_id", "name", "brief"}, id, b); err != nil {
		return nil, err
	}
	return row(campaignSelect+" where c.id=?", id)
}

// deleteCampaign keeps the videos; they just stop belonging to a campaign.
func deleteCampaign(r *http.Request) (any, error) {
	id := pathID(r)
	if _, err := db.Exec("update projects set campaign_id=null where campaign_id=?", id); err != nil {
		return nil, err
	}
	_, err := db.Exec("delete from campaigns where id=?", id)
	return nil, err
}
