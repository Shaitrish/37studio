package company

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/store"
)

// A website scan fills only fields the editor left unset; the hostname placeholder name counts as unset.
func TestApplyProfile(t *testing.T) {
	config.DataDir = t.TempDir()
	store.Open()
	store.DB.Exec("insert into tenants(id,name,website,teacher) values(7,'x.com','https://x.com','Set by editor')")
	os.MkdirAll(files.TenantRoot(7), 0o755)
	os.WriteFile(filepath.Join(files.TenantRoot(7), "profile.json"), []byte(`{"name":"X Academy","teacher":"From site","brand_color":"#112233","margin_pct":"10"}`), 0o644)
	if !applyProfile(7) {
		t.Fatal("profile not applied")
	}
	tn, _ := store.Row("select * from tenants where id=7")
	if tn["name"] != "X Academy" || tn["teacher"] != "Set by editor" || tn["brand_color"] != "#112233" || tn["margin_pct"] != nil {
		t.Errorf("unexpected tenant after scan: %v", tn)
	}
}
