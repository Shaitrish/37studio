package agent

import (
	"strings"
	"testing"
	"time"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/store"
)

// The prompt names the most viewed video and the least viewed one that has had 3 days to find its audience.
func TestPerformanceText(t *testing.T) {
	config.DataDir = t.TempDir()
	store.Open()
	t.Cleanup(store.Close)
	if got := performanceText(1); !strings.HasPrefix(got, "no numbers yet") {
		t.Fatalf("empty: %s", got)
	}
	store.DB.Exec("insert into tenants(id,name) values(1,'Shop')")
	store.DB.Exec("insert into social_accounts(id,tenant_id,platform,account_id,name) values(1,1,'instagram','ig','@shop')")
	day := func(d int) string {
		return time.Now().Add(-time.Duration(d) * 24 * time.Hour).UTC().Format(time.RFC3339)
	}
	for _, v := range []struct {
		id, title, at string
		views         int
		ret           any
	}{{"a", "Eid sale", day(10), 9000, 40.0}, {"b", "Old flop", day(5), 120, 12.0}, {"c", "Posted today", day(0), 15, nil}} {
		store.DB.Exec("insert into video_stats(account_id,media_id,platform,title,published_at,views,retention) values(1,?,?,?,?,?,?)", v.id, "instagram", v.title, v.at, v.views, v.ret)
	}
	got := performanceText(1)
	for _, want := range []string{"3 videos tracked", "26% watched on average", `Most viewed: "Eid sale" (instagram, 9000 views, 40% watched)`, `least viewed: "Old flop"`, "37studio stats"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}
