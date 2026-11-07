package social

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/store"
)

// A stand-in Graph API: one Reel (listed under both edges), one regular video, and a retired metric that fails any
// request asking for it, the way Meta answers.
func fakeGraph(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metric := r.URL.Query().Get("metric")
		path := strings.TrimPrefix(r.URL.Path, "/"+graphVer)
		switch {
		case path == "/page1/video_reels":
			fmt.Fprint(w, `{"data":[{"id":"r1","description":"Hook test","created_time":"2026-11-01T10:00:00+0000","permalink_url":"/reel/r1","length":20}]}`)
		case path == "/page1/videos":
			fmt.Fprint(w, `{"data":[{"id":"r1"},{"id":"v2","title":"Long lesson","created_time":"2026-10-01T10:00:00+0000","permalink_url":"https://www.facebook.com/x/videos/v2","length":100}]}`)
		case strings.Contains(metric, "post_impressions_unique"):
			w.WriteHeader(400)
			fmt.Fprint(w, `{"error":{"message":"(#100) The value must be a valid insights metric"}}`)
		case path == "/r1/video_insights":
			vals := map[string]string{"blue_reels_play_count": "1200", "post_video_avg_time_watched": "9000", "post_video_retention_graph": `{"0":1,"1":0.5}`,
				"post_video_social_actions": `{"COMMENT":4,"SHARE":2}`, "post_video_likes_by_reaction_type": `{"REACTION_LIKE":30,"REACTION_LOVE":5}`}
			var data []string
			for _, m := range strings.Split(metric, ",") {
				data = append(data, fmt.Sprintf(`{"name":%q,"values":[{"value":%s}]}`, m, vals[m]))
			}
			fmt.Fprintf(w, `{"data":[%s]}`, strings.Join(data, ","))
		case path == "/v2/video_insights":
			fmt.Fprint(w, `{"data":[{"name":"total_video_views","values":[{"value":300}]},{"name":"total_video_avg_time_watched","values":[{"value":25000}]},
			  {"name":"total_video_stories_by_action_type","values":[{"value":{"like":10,"comment":1}}]}]}`)
		case path == "/ig1/media":
			fmt.Fprint(w, `{"data":[{"id":"m1","caption":"Reel","media_type":"VIDEO","media_product_type":"REELS","timestamp":"2026-11-02T08:00:00+0000","like_count":7,"comments_count":1},
			  {"id":"m2","media_type":"IMAGE"}]}`)
		case path == "/m1/insights":
			fmt.Fprint(w, `{"data":[{"name":"views","values":[{"value":900}]},{"name":"saved","values":[{"value":12}]},{"name":"likes","values":[{"value":40}]},
			  {"name":"ig_reels_avg_watch_time","values":[{"value":4500}]},{"name":"reels_skip_rate","values":[{"value":38.5}]}]}`)
		default:
			t.Errorf("unexpected call %s?%s", r.URL.Path, r.URL.RawQuery)
			w.WriteHeader(404)
		}
	}))
	old := graphURL
	graphURL = srv.URL
	t.Cleanup(func() { graphURL = old; srv.Close() })
	return srv
}

// Page videos: Reels and regular videos get their own metric names, a retired metric doesn't sink the others, and
// retention is average watch time over length, as on YouTube.
func TestFacebookStats(t *testing.T) {
	fakeGraph(t)
	vs, err := facebookStats("page1", "tok")
	if err != nil {
		t.Errorf("a retired metric is not the account's problem: %v", err)
	}
	if len(vs) != 2 || vs[0].MediaID != "r1" || vs[1].MediaID != "v2" {
		t.Fatalf("videos: %+v", vs)
	}
	r := vs[0]
	if *r.Views != 1200 || r.Reach != nil || *r.Likes != 35 || *r.Comments != 4 || *r.Shares != 2 || *r.WatchAvg != 9 {
		t.Errorf("reel numbers: %+v", r)
	}
	if len(r.Curve) != 2 || r.Curve[0] != 100 || r.Curve[1] != 50 || *r.Retention != 45 {
		t.Errorf("reel retention: curve %v, retention %v", r.Curve, *r.Retention)
	}
	if r.URL != "https://www.facebook.com/reel/r1" || r.Published != "2026-11-01T10:00:00Z" {
		t.Errorf("reel link/time: %s %s", r.URL, r.Published)
	}
	v := vs[1]
	if *v.Views != 300 || *v.Comments != 1 || *v.Shares != 0 || *v.Retention != 25 {
		t.Errorf("video numbers: %+v retention %v", v, *v.Retention)
	}
}

// Instagram: only videos, numbers from insights over the media fields, and the skip rate for Reels.
func TestInstagramStats(t *testing.T) {
	fakeGraph(t)
	vs, err := instagramStats("ig1", "tok")
	if err != nil || len(vs) != 1 {
		t.Fatalf("got %+v %v", vs, err)
	}
	m := vs[0]
	if *m.Views != 900 || *m.Likes != 40 || *m.Comments != 1 || *m.Saves != 12 || *m.WatchAvg != 4.5 || *m.SkipRate != 38.5 || m.Retention != nil {
		t.Errorf("numbers: %+v", m)
	}
}

// YouTube: counts from the Data API, retention and shares from YouTube Analytics.
func TestYouTubeStats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/channels":
			fmt.Fprint(w, `{"items":[{"contentDetails":{"relatedPlaylists":{"uploads":"UU1"}}}]}`)
		case "/playlistItems":
			fmt.Fprint(w, `{"items":[{"contentDetails":{"videoId":"y1"}}]}`)
		case "/videos":
			fmt.Fprint(w, `{"items":[{"id":"y1","snippet":{"title":"Short","publishedAt":"2026-11-03T00:00:00Z"},"statistics":{"viewCount":"5000","commentCount":"9"},"contentDetails":{"duration":"PT1M5S"}}]}`)
		case "/reports":
			if !strings.Contains(r.URL.Query().Get("filters"), "y1") {
				t.Errorf("analytics filter: %s", r.URL.RawQuery)
			}
			fmt.Fprint(w, `{"columnHeaders":[{"name":"video"},{"name":"averageViewDuration"},{"name":"averageViewPercentage"},{"name":"shares"}],"rows":[["y1",40,61.5,7]]}`)
		}
	}))
	defer srv.Close()
	oldAPI, oldAnalytics := youtubeAPI, ytAnalytics
	youtubeAPI, ytAnalytics = srv.URL, srv.URL
	defer func() { youtubeAPI, ytAnalytics = oldAPI, oldAnalytics }()
	vs, err := youtubeStats("tok")
	if err != nil || len(vs) != 1 {
		t.Fatalf("got %+v %v", vs, err)
	}
	y := vs[0]
	if *y.Views != 5000 || y.Likes != nil || *y.Comments != 9 || *y.Duration != 65 || *y.WatchAvg != 40 || *y.Retention != 61.5 || *y.Shares != 7 {
		t.Errorf("numbers: %+v", y)
	}
}

// A refresh that couldn't read a number keeps the last one; the list links videos to the project that posted them
// and ranks by engagement rate.
func TestSaveAndRankStats(t *testing.T) {
	config.DataDir = t.TempDir()
	store.Open()
	t.Cleanup(store.Close)
	store.DB.Exec("insert into tenants(id,name) values(1,'Shop')")
	store.DB.Exec("insert into social_accounts(id,tenant_id,platform,account_id,name) values(1,1,'facebook','page1','Shop')")
	store.DB.Exec("insert into projects(id,tenant_id,title) values(5,1,'Eid promo')")
	store.DB.Exec("insert into posts(project_id,account_id,platform,account_name,file,status,publish_at,remote_id) values(5,1,'facebook','Shop','out/a.mp4','posted','now','a')")
	n := func(v float64) *int64 { return ptrI(v) }
	saveStats(1, "facebook", []videoStat{{MediaID: "a", Title: "Eid", Views: n(100), Likes: n(20), Retention: ptrF(50)}, {MediaID: "b", Title: "Plain", Views: n(1000), Likes: n(10)}})
	saveStats(1, "facebook", []videoStat{{MediaID: "a", Title: "Eid", Views: n(150)}}) // insights failed this time
	rows, err := statsRows(1, "", "rate", false, 10)
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows %v %v", rows, err)
	}
	a := rows[0]
	if a["media_id"] != "a" || store.Num(a["views"]) != 150 || store.Num(a["likes"]) != 20 || store.Num(a["retention"]) != 50 || store.Num(a["project_id"]) != 5 || a["project"] != "Eid promo" {
		t.Errorf("first: %v", a)
	}
	if store.Num(a["engagement_rate"]) != 13.33 || rows[1]["project_id"] != nil {
		t.Errorf("rate %v, second project %v", a["engagement_rate"], rows[1]["project_id"])
	}
}

func TestIsoSeconds(t *testing.T) {
	for in, want := range map[string]float64{"PT1M5S": 65, "PT45S": 45, "PT1H2M": 3720, "P1DT1S": 86401, "": 0} {
		if got := isoSeconds(in); got != want {
			t.Errorf("%s: got %v, want %v", in, got, want)
		}
	}
}
