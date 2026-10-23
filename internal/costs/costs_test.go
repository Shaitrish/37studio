package costs

import (
	"testing"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/keys"
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/update"
)

// Cost, key storage and path-safety checks; run with `go test`.
func TestQuoteAndPaths(t *testing.T) {
	config.DataDir = t.TempDir()
	store.Open()
	store.DB.Exec("insert into tenants(id,name) values(1,'A')")
	store.DB.Exec("insert into projects(id,tenant_id,title) values(1,1,'p1'),(3,1,'empty')")
	store.DB.Exec("insert into usage(project_id,tenant_id,provider,item,quantity,cost) values(1,1,'anthropic','m',1,6),(1,1,'gemini','img',1,4)")
	cost := func(id int64) float64 {
		p, err := store.Row(store.ProjectSelect+" where p.id=?", id)
		if err != nil {
			t.Fatal(err)
		}
		return Quote(p)["cost"].(float64)
	}
	if got := cost(1); got != 10 {
		t.Errorf("project cost: got %v, want 10", got)
	}
	if got := cost(3); got != 0 {
		t.Errorf("empty project: got %v, want 0", got)
	}

	// Keys are stored encrypted and come back through the environment.
	enc, _ := keys.Seal("sk-test-1234")
	if v, err := keys.Unseal(enc); err != nil || v != "sk-test-1234" || string(enc) == "sk-test-1234" {
		t.Errorf("seal/unseal: %q %v", v, err)
	}

	if !update.Newer("0.10.0", "0.9.9") || update.Newer("0.3.0", "0.3.0") {
		t.Error("version compare")
	}

	if got := files.Within("/data/p", "../../etc/passwd"); got != "/data/p/etc/passwd" {
		t.Errorf("within escaped root: %s", got)
	}
	if got := files.SafeName(`..\..\evil video.mp4`); got != "evil video.mp4" {
		t.Errorf("safeName: %q", got)
	}
}
