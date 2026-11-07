package social

import (
	"strings"
	"testing"
	"time"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/store"
)

// Claude's review replaces only its own section; the team's plan and the agent's notes stay.
func TestStrategySections(t *testing.T) {
	doc := "## Our plan\nExam tips.\n\n## What the numbers say\n\n- old lesson\n\n## Notes\n\n- 2026-11-01: keep it short\n"
	got := setSection(doc, numbersHeading, "- new lesson")
	if !strings.Contains(got, "## Our plan\nExam tips.") || !strings.Contains(got, "- new lesson") || strings.Contains(got, "old lesson") || !strings.Contains(got, "keep it short") {
		t.Fatalf("replace:\n%s", got)
	}
	if got := setSection("## Our plan\nExam tips.", numbersHeading, "- first"); !strings.HasPrefix(got, numbersHeading+"\n\n- first") || !strings.Contains(got, "Exam tips.") {
		t.Fatalf("insert:\n%s", got)
	}
	day := time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)
	if got := addNote("", "Map  explainers\nwin", day); got != "## Notes\n\n- 2026-11-07: Map explainers win\n" {
		t.Fatalf("first note: %q", got)
	}
	if got := addNote(doc, "second", day); !strings.HasSuffix(got, "- 2026-11-01: keep it short\n- 2026-11-07: second\n") {
		t.Fatalf("append: %q", got)
	}
}

// One video posted to two platforms is one row with both platforms' numbers, the totals and where it did best.
func TestPublishedVideos(t *testing.T) {
	config.DataDir = t.TempDir()
	store.Open()
	t.Cleanup(store.Close)
	store.DB.Exec("insert into tenants(id,name) values(1,'Shop')")
	store.DB.Exec("insert into projects(id,tenant_id,title) values(5,1,'Eid promo')")
	store.DB.Exec("insert into social_accounts(id,tenant_id,platform,account_id,name) values(1,1,'facebook','p','Shop'),(2,1,'instagram','i','@shop')")
	store.DB.Exec(`insert into posts(project_id,account_id,platform,account_name,file,status,publish_at,remote_id) values
	  (5,1,'facebook','Shop','out/a.mp4','posted','2026-11-01 10:00:00','fa'),(5,2,'instagram','@shop','out/a.mp4','posted','2026-11-01 10:00:00','ia'),
	  (5,1,'facebook','Shop','out/b.mp4','failed','2026-11-02 10:00:00','')`)
	saveStats(1, "facebook", []videoStat{{MediaID: "fa", Views: ptrI(200), Likes: ptrI(2)}})
	saveStats(2, "instagram", []videoStat{{MediaID: "ia", Views: ptrI(800), Likes: ptrI(78)}})
	vs, err := publishedVideos(1)
	if err != nil || len(vs) != 1 {
		t.Fatalf("got %v %v", vs, err)
	}
	v := vs[0]
	if len(v["posts"].([]map[string]any)) != 2 || v["views"] != 1000.0 || v["best"] != "instagram" || v["engagement_rate"] != 8.0 || v["url"] != "/files/projects/5/out/a.mp4" {
		t.Errorf("video: %v", v)
	}
}
