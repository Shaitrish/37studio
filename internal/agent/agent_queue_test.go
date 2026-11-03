package agent

import (
	"slices"
	"testing"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/store"
)

// Messages typed mid-turn survive a restart (they live in the database) and come back in order, once.
func TestQueuedMessages(t *testing.T) {
	config.DataDir = t.TempDir()
	store.Open()
	t.Cleanup(store.Close)
	store.DB.Exec("insert into tenants(id,name) values(1,'Shop')")
	store.DB.Exec("insert into projects(id,tenant_id,title,kind,aspect,seconds,quality) values(1,1,'A','reel','9:16',30,'standard')")
	store.DB.Exec("insert into queued_messages(project_id,text,quality) values(1,'first',''),(1,'second','fast')")
	msgs, q := takeQueued(1)
	if !slices.Equal(msgs, []string{"first", "second"}) || q != "fast" {
		t.Fatalf("got %v %q", msgs, q)
	}
	if msgs, _ := takeQueued(1); len(msgs) != 0 {
		t.Fatalf("sent twice: %v", msgs)
	}
}
