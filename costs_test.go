package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Cost, key storage, license and path-safety checks; run with `go test`.
func TestQuoteAndPaths(t *testing.T) {
	dataDir = t.TempDir()
	openDB()
	db.Exec("insert into tenants(id,name) values(1,'A')")
	db.Exec("insert into projects(id,tenant_id,title) values(1,1,'p1'),(3,1,'empty')")
	db.Exec("insert into usage(project_id,tenant_id,provider,item,quantity,cost) values(1,1,'anthropic','m',1,6),(1,1,'gemini','img',1,4)")
	cost := func(id int64) float64 {
		p, err := row(projectSelect+" where p.id=?", id)
		if err != nil {
			t.Fatal(err)
		}
		return quote(p)["cost"].(float64)
	}
	if got := cost(1); got != 10 {
		t.Errorf("project cost: got %v, want 10", got)
	}
	if got := cost(3); got != 0 {
		t.Errorf("empty project: got %v, want 0", got)
	}

	// Keys are stored encrypted and come back through the environment.
	enc, _ := seal("sk-test-1234")
	if v, err := unseal(enc); err != nil || v != "sk-test-1234" || string(enc) == "sk-test-1234" {
		t.Errorf("seal/unseal: %q %v", v, err)
	}

	if _, err := parseLicense("37S1.e30.AAAA"); err == nil {
		t.Error("forged license accepted")
	}
	if !newer("0.10.0", "0.9.9") || newer("0.3.0", "0.3.0") {
		t.Error("version compare")
	}

	if got := within("/data/p", "../../etc/passwd"); got != "/data/p/etc/passwd" {
		t.Errorf("within escaped root: %s", got)
	}
	if got := safeName(`..\..\evil video.mp4`); got != "evil video.mp4" {
		t.Errorf("safeName: %q", got)
	}
}

// A website scan fills only fields the editor left unset; the hostname placeholder name counts as unset.
func TestApplyProfile(t *testing.T) {
	dataDir = t.TempDir()
	openDB()
	db.Exec("insert into tenants(id,name,website,teacher) values(7,'x.com','https://x.com','Set by editor')")
	os.MkdirAll(tenantRoot(7), 0o755)
	os.WriteFile(filepath.Join(tenantRoot(7), "profile.json"), []byte(`{"name":"X Academy","teacher":"From site","brand_color":"#112233","margin_pct":"10"}`), 0o644)
	if !applyProfile(7) {
		t.Fatal("profile not applied")
	}
	tn, _ := row("select * from tenants where id=7")
	if tn["name"] != "X Academy" || tn["teacher"] != "Set by editor" || tn["brand_color"] != "#112233" || tn["margin_pct"] != nil {
		t.Errorf("unexpected tenant after scan: %v", tn)
	}
}
