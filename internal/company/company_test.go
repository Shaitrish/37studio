package company

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/store"
)

// Deleting a brand takes its projects, campaigns, accounts and files with it, and leaves other brands alone.
func TestDeleteCascades(t *testing.T) {
	config.DataDir = t.TempDir()
	store.Open()
	t.Cleanup(store.Close)
	store.DB.Exec("insert into tenants(id,name) values(1,'Shop'),(2,'Other')")
	store.DB.Exec("insert into campaigns(id,tenant_id,name) values(1,1,'Eid')")
	store.DB.Exec("insert into projects(id,tenant_id,campaign_id,title,kind,aspect,seconds,quality) values(1,1,1,'A','reel','9:16',30,'standard'),(2,2,null,'B','reel','9:16',30,'standard')")
	store.DB.Exec("insert into social_accounts(id,tenant_id,platform,account_id,name) values(1,1,'x','a','A')")
	store.DB.Exec("insert into posts(project_id,account_id,platform,account_name,file,publish_at,status) values(1,1,'x','','out/a.mp4','','posted')")
	for _, d := range []string{files.ProjectDir(1), files.ProjectDir(2), files.TenantDir(1)} {
		os.MkdirAll(d, 0o755)
	}

	r := httptest.NewRequest("DELETE", "/api/tenants/1", nil)
	r.SetPathValue("id", "1")
	if _, err := Delete(r); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"tenants where id=1", "projects where tenant_id=1", "campaigns", "social_accounts", "posts"} {
		var n int
		store.DB.QueryRow("select count(*) from " + q).Scan(&n)
		if n != 0 {
			t.Fatalf("%s: %d rows left", q, n)
		}
	}
	if _, err := os.Stat(files.ProjectDir(1)); !os.IsNotExist(err) {
		t.Fatal("project folder left behind")
	}
	if _, err := os.Stat(files.ProjectDir(2)); err != nil {
		t.Fatal("another brand's project was touched")
	}
}
