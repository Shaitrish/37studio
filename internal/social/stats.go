package social

import (
	"cmp"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"maps"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/keys"
	"github.com/Shaitrish/37studio/internal/remote"
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/textutil"
)

// Video performance. Every 6 hours, and when someone asks, 37Studio reads the recent videos on each connected Facebook
// Page, Instagram, YouTube and TikTok account with the platform's own numbers for them into video_stats. The
// Performance page, the MCP tool video_stats and the editor agent (37studio stats) all read that table.

var (
	youtubeAPI  = "https://www.googleapis.com/youtube/v3" // or a stand-in, for tests
	ytAnalytics = "https://youtubeanalytics.googleapis.com/v2"
)

const statsMax = 50 // recent videos per account: enough to see what works, and a refresh stays at a few hundred calls

var statsPlatforms = []string{"facebook", "instagram", "youtube", "tiktok"}

type videoStat struct {
	MediaID, Title, URL, Thumb, Published string
	Duration                              *float64 // seconds
	Views, Reach, Likes, Comments, Shares *int64
	Saves                                 *int64
	WatchAvg, Retention, SkipRate         *float64
	Curve                                 []float64 // share of viewers still watching, 0-100, at evenly spaced points
}

func ptrI(v float64) *int64   { n := int64(math.Round(v)); return &n }
func ptrF(v float64) *float64 { v = math.Round(v*10) / 10; return &v }

// val turns an unknown number into SQL null.
func val[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

// utc normalizes the platforms' timestamps ("2026-11-01T10:00:00+0000", RFC 3339) so they sort as text and every
// browser parses them.
func utc(s string) string {
	for _, layout := range []string{"2006-01-02T15:04:05-0700", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return s
}

// --- Meta (Facebook and Instagram) ---

type graphInsights struct {
	Data []struct {
		Name   string `json:"name"`
		Values []struct {
			Value json.RawMessage `json:"value"`
		} `json:"values"`
		Total *struct {
			Value json.RawMessage `json:"value"`
		} `json:"total_value"`
	} `json:"data"`
}

// insights asks Graph for several metrics at once. One retired or unsupported metric fails the whole request, so after
// an error it asks one at a time and notes in bad which ones fail, and the rest of this refresh leaves them out.
func insights(path, tok string, metrics []string, bad map[string]bool, kind string) (map[string]json.RawMessage, error) {
	var ms []string
	for _, m := range metrics {
		if !bad[kind+"/"+m] {
			ms = append(ms, m)
		}
	}
	get := func(ms []string) (map[string]json.RawMessage, error) {
		var r graphInsights
		if err := remote.Call(remote.Get(graph(path, url.Values{"metric": {strings.Join(ms, ",")}}), "Bearer "+tok), &r); err != nil {
			return nil, err
		}
		out := map[string]json.RawMessage{}
		for _, d := range r.Data {
			if len(d.Values) > 0 {
				out[d.Name] = d.Values[0].Value
			} else if d.Total != nil {
				out[d.Name] = d.Total.Value
			}
		}
		return out, nil
	}
	if len(ms) == 0 {
		return nil, errors.New("the platform refused every metric earlier in this refresh")
	}
	out, err := get(ms)
	if err == nil || len(ms) == 1 {
		return out, err
	}
	out = map[string]json.RawMessage{}
	for _, m := range ms {
		o, e := get([]string{m})
		if e != nil {
			bad[kind+"/"+m], err = true, e
			continue
		}
		maps.Copy(out, o)
	}
	if len(out) == 0 {
		return nil, err
	}
	return out, nil
}

// count reads a metric: a number, or counts by type ({"like": 3, "love": 1}), which it adds up.
func count(raw json.RawMessage) (float64, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, false
	}
	var n float64
	if json.Unmarshal(raw, &n) == nil {
		return n, true
	}
	var m map[string]float64
	if json.Unmarshal(raw, &m) != nil {
		return 0, false
	}
	n = 0
	for _, v := range m {
		n += v
	}
	return n, true
}

// countOf reads one type from counts by type, like the comments in {"COMMENT": 3, "SHARE": 1}; a missing type is 0.
func countOf(raw json.RawMessage, key string) (float64, bool) {
	var m map[string]float64
	if len(raw) == 0 || json.Unmarshal(raw, &m) != nil {
		return 0, false
	}
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return 0, true
}

// retentionCurve turns Meta's retention graph ({"0": 1, "1": 0.83, …}, by step) into percentages in step order.
func retentionCurve(raw json.RawMessage) []float64 {
	var m map[string]float64
	if len(raw) == 0 || json.Unmarshal(raw, &m) != nil || len(m) == 0 {
		return nil
	}
	steps := make([]int, 0, len(m))
	top := 0.0
	for k, v := range m {
		i, err := strconv.Atoi(k)
		if err != nil {
			return nil
		}
		steps = append(steps, i)
		top = math.Max(top, v)
	}
	sort.Ints(steps)
	scale := 1.0
	if top <= 1.0001 { // fractions, not percentages
		scale = 100
	}
	out := make([]float64, len(steps))
	for i, s := range steps {
		out[i] = math.Round(m[strconv.Itoa(s)]*scale*10) / 10
	}
	return out
}

// average of a retention curve: roughly the average share of the video watched.
func average(xs []float64) float64 {
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

// Reels and other Page videos have different metric names for the same numbers.
var fbMetrics = map[bool]map[string]string{
	true: {"views": "blue_reels_play_count", "reach": "post_impressions_unique", "watch": "post_video_avg_time_watched",
		"curve": "post_video_retention_graph", "actions": "post_video_social_actions", "likes": "post_video_likes_by_reaction_type"},
	false: {"views": "total_video_views", "reach": "total_video_impressions_unique", "watch": "total_video_avg_time_watched",
		"curve": "total_video_retention_graph", "actions": "total_video_stories_by_action_type", "likes": "total_video_reactions_by_type_total"},
}

type fbVideo struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Created     string  `json:"created_time"`
	Permalink   string  `json:"permalink_url"`
	Length      float64 `json:"length"`
	Picture     string  `json:"picture"`
}

// facebookStats reads a Page's recent Reels and videos (read_insights). The error is set when some numbers couldn't
// be read; the videos it could read still come back.
func facebookStats(page, tok string) ([]videoStat, error) {
	list := func(edge string) ([]fbVideo, error) {
		var r struct {
			Data []fbVideo `json:"data"`
		}
		err := remote.Call(remote.Get(graph("/"+page+"/"+edge, url.Values{"fields": {"id,title,description,created_time,permalink_url,length,picture"},
			"limit": {strconv.Itoa(statsMax)}}), "Bearer "+tok), &r)
		return r.Data, err
	}
	reels, err := list("video_reels")
	if err != nil {
		return nil, err
	}
	vids, err := list("videos")
	if err != nil {
		return nil, err
	}
	reel := map[string]bool{}
	for _, v := range reels {
		reel[v.ID] = true
	}
	all := reels
	for _, v := range vids {
		if !reel[v.ID] {
			all = append(all, v)
		}
	}
	for i := range all {
		all[i].Created = utc(all[i].Created)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Created > all[j].Created })
	all = all[:min(len(all), statsMax)]

	bad := map[string]bool{}
	var out []videoStat
	var miss error
	for _, v := range all {
		s := videoStat{MediaID: v.ID, Title: cmp.Or(v.Title, v.Description), URL: v.Permalink, Thumb: v.Picture, Published: v.Created}
		if strings.HasPrefix(s.URL, "/") {
			s.URL = "https://www.facebook.com" + s.URL
		}
		if v.Length > 0 {
			s.Duration = ptrF(v.Length)
		}
		names := fbMetrics[reel[v.ID]]
		m, err := insights("/"+v.ID+"/video_insights", tok, slicesOf(names), bad, fmt.Sprint(reel[v.ID]))
		if err != nil {
			miss = cmp.Or(miss, err)
		}
		if n, ok := count(m[names["views"]]); ok {
			s.Views = ptrI(n)
		}
		if n, ok := count(m[names["reach"]]); ok {
			s.Reach = ptrI(n)
		}
		if n, ok := count(m[names["likes"]]); ok {
			s.Likes = ptrI(n)
		}
		if n, ok := countOf(m[names["actions"]], "comment"); ok {
			s.Comments = ptrI(n)
		}
		if n, ok := countOf(m[names["actions"]], "share"); ok {
			s.Shares = ptrI(n)
		}
		if n, ok := count(m[names["watch"]]); ok {
			s.WatchAvg = ptrF(n / 1000)
		}
		// Average watch time over length, the way YouTube counts "average percentage viewed", so platforms compare.
		// The curve's average runs higher (it leaves out plays that never got going), so it's only the fallback.
		if s.Curve = retentionCurve(m[names["curve"]]); s.WatchAvg != nil && v.Length > 0 {
			s.Retention = ptrF(math.Min(100, *s.WatchAvg/v.Length*100))
		} else if len(s.Curve) > 0 {
			s.Retention = ptrF(average(s.Curve))
		}
		out = append(out, s)
	}
	return out, miss
}

func slicesOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// instagramStats reads an Instagram account's recent videos (instagram_manage_insights). Instagram gives no video
// length, so retention stays empty; average watch time and the 3-second skip rate stand in for it.
func instagramStats(ig, tok string) ([]videoStat, error) {
	var r struct {
		Data []struct {
			ID        string `json:"id"`
			Caption   string `json:"caption"`
			Type      string `json:"media_type"`
			Product   string `json:"media_product_type"`
			Permalink string `json:"permalink"`
			Timestamp string `json:"timestamp"`
			Thumb     string `json:"thumbnail_url"`
			Likes     *int64 `json:"like_count"`
			Comments  *int64 `json:"comments_count"`
		} `json:"data"`
	}
	if err := remote.Call(remote.Get(graph("/"+ig+"/media", url.Values{"fields": {"id,caption,media_type,media_product_type,permalink,timestamp,thumbnail_url,like_count,comments_count"},
		"limit": {strconv.Itoa(statsMax)}}), "Bearer "+tok), &r); err != nil {
		return nil, err
	}
	bad := map[string]bool{}
	var out []videoStat
	var miss error
	for _, v := range r.Data {
		if v.Type != "VIDEO" {
			continue
		}
		s := videoStat{MediaID: v.ID, Title: v.Caption, URL: v.Permalink, Thumb: v.Thumb, Published: utc(v.Timestamp), Likes: v.Likes, Comments: v.Comments}
		metrics := []string{"views", "reach", "likes", "comments", "shares", "saved"}
		if v.Product == "REELS" {
			metrics = append(metrics, "ig_reels_avg_watch_time", "reels_skip_rate")
		}
		m, err := insights("/"+v.ID+"/insights", tok, metrics, bad, v.Product)
		if err != nil {
			miss = cmp.Or(miss, err)
		}
		for name, dst := range map[string]**int64{"views": &s.Views, "reach": &s.Reach, "likes": &s.Likes, "comments": &s.Comments, "shares": &s.Shares, "saved": &s.Saves} {
			if n, ok := count(m[name]); ok {
				*dst = ptrI(n)
			}
		}
		if n, ok := count(m["ig_reels_avg_watch_time"]); ok {
			s.WatchAvg = ptrF(n / 1000)
		}
		if n, ok := count(m["reels_skip_rate"]); ok {
			s.SkipRate = ptrF(n)
		}
		out = append(out, s)
	}
	return out, miss
}

// --- YouTube ---

var isoDur = regexp.MustCompile(`^P(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$`)

// isoSeconds reads YouTube's ISO 8601 durations, like PT1M5S.
func isoSeconds(s string) float64 {
	m := isoDur.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	n := 0.0
	for i, unit := range []float64{86400, 3600, 60, 1} {
		v, _ := strconv.ParseFloat(m[i+1], 64)
		n += v * unit
	}
	return n
}

// youtubeStats reads the channel's latest uploads (youtube.readonly), then retention and shares from YouTube
// Analytics. Logins from before 37Studio asked for yt-analytics.readonly still get views, likes and comments.
func youtubeStats(tok string) ([]videoStat, error) {
	auth := "Bearer " + tok
	var ch struct {
		Items []struct {
			Content struct {
				Playlists struct {
					Uploads string `json:"uploads"`
				} `json:"relatedPlaylists"`
			} `json:"contentDetails"`
		} `json:"items"`
	}
	if err := remote.Call(remote.Get(youtubeAPI+"/channels?part=contentDetails&mine=true", auth), &ch); err != nil {
		return nil, err
	}
	if len(ch.Items) == 0 {
		return nil, errors.New("this Google login has no YouTube channel")
	}
	var pl struct {
		Items []struct {
			Content struct {
				VideoID string `json:"videoId"`
			} `json:"contentDetails"`
		} `json:"items"`
	}
	q := url.Values{"part": {"contentDetails"}, "maxResults": {strconv.Itoa(statsMax)}, "playlistId": {ch.Items[0].Content.Playlists.Uploads}}
	if err := remote.Call(remote.Get(youtubeAPI+"/playlistItems?"+q.Encode(), auth), &pl); err != nil {
		return nil, err
	}
	var ids []string
	for _, it := range pl.Items {
		ids = append(ids, it.Content.VideoID)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var vs struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title      string                          `json:"title"`
				Published  string                          `json:"publishedAt"`
				Thumbnails map[string]struct{ URL string } `json:"thumbnails"`
			} `json:"snippet"`
			Stats   map[string]string `json:"statistics"` // counts come as strings; likes are missing when hidden
			Content struct {
				Duration string `json:"duration"`
			} `json:"contentDetails"`
		} `json:"items"`
	}
	q = url.Values{"part": {"snippet,statistics,contentDetails"}, "id": {strings.Join(ids, ",")}}
	if err := remote.Call(remote.Get(youtubeAPI+"/videos?"+q.Encode(), auth), &vs); err != nil {
		return nil, err
	}
	out := make([]videoStat, 0, len(vs.Items))
	at := map[string]int{}
	for _, v := range vs.Items {
		s := videoStat{MediaID: v.ID, Title: v.Snippet.Title, URL: "https://youtu.be/" + v.ID, Published: utc(v.Snippet.Published),
			Thumb: cmp.Or(v.Snippet.Thumbnails["medium"].URL, v.Snippet.Thumbnails["default"].URL)}
		if d := isoSeconds(v.Content.Duration); d > 0 {
			s.Duration = ptrF(d)
		}
		for name, dst := range map[string]**int64{"viewCount": &s.Views, "likeCount": &s.Likes, "commentCount": &s.Comments} {
			if n, err := strconv.ParseFloat(v.Stats[name], 64); err == nil {
				*dst = ptrI(n)
			}
		}
		at[v.ID] = len(out)
		out = append(out, s)
	}
	var rep struct {
		Headers []struct {
			Name string `json:"name"`
		} `json:"columnHeaders"`
		Rows [][]any `json:"rows"`
	}
	q = url.Values{"ids": {"channel==MINE"}, "startDate": {"2005-02-14"}, "endDate": {time.Now().Format("2006-01-02")}, "dimensions": {"video"},
		"metrics": {"averageViewDuration,averageViewPercentage,shares"}, "filters": {"video==" + strings.Join(ids, ",")}, "sort": {"-averageViewPercentage"}, "maxResults": {"200"}}
	if err := remote.Call(remote.Get(ytAnalytics+"/reports?"+q.Encode(), auth), &rep); err != nil {
		return out, fmt.Errorf("retention needs YouTube Analytics: enable the YouTube Analytics API in your Google project and connect YouTube again (%v)", err)
	}
	col := map[string]int{}
	for i, h := range rep.Headers {
		col[h.Name] = i
	}
	for _, row := range rep.Rows {
		i, ok := at[fmt.Sprint(row[col["video"]])]
		if !ok {
			continue
		}
		num := func(name string) (float64, bool) {
			n, ok := row[col[name]].(float64)
			return n, ok && col[name] > 0
		}
		if n, ok := num("averageViewDuration"); ok {
			out[i].WatchAvg = ptrF(n)
		}
		if n, ok := num("averageViewPercentage"); ok {
			out[i].Retention = ptrF(math.Min(100, n))
		}
		if n, ok := num("shares"); ok {
			out[i].Shares = ptrI(n)
		}
	}
	return out, nil
}

// --- TikTok ---

// tiktokStats reads the account's latest public videos (video.list). TikTok's Display API has no retention.
func tiktokStats(tok string) ([]videoStat, error) {
	var out []videoStat
	cursor := json.Number("")
	for len(out) < statsMax {
		body := map[string]any{"max_count": 20}
		if cursor != "" {
			body["cursor"] = cursor
		}
		var r struct {
			Data struct {
				Videos []struct {
					ID       json.Number `json:"id"`
					Title    string      `json:"title"`
					Desc     string      `json:"video_description"`
					Created  int64       `json:"create_time"`
					Cover    string      `json:"cover_image_url"`
					Share    string      `json:"share_url"`
					Duration float64     `json:"duration"`
					Views    *float64    `json:"view_count"`
					Likes    *float64    `json:"like_count"`
					Comments *float64    `json:"comment_count"`
					Shares   *float64    `json:"share_count"`
				} `json:"videos"`
				Cursor  json.Number `json:"cursor"`
				HasMore bool        `json:"has_more"`
			} `json:"data"`
		}
		fields := "id,title,video_description,create_time,cover_image_url,share_url,duration,view_count,like_count,comment_count,share_count"
		if err := remote.Call(remote.JSON("POST", tiktokAPI+"/v2/video/list/?fields="+fields, body, "Bearer "+tok), &r); err != nil {
			return out, err
		}
		for _, v := range r.Data.Videos {
			s := videoStat{MediaID: v.ID.String(), Title: cmp.Or(v.Title, v.Desc), URL: v.Share, Thumb: v.Cover,
				Published: time.Unix(v.Created, 0).UTC().Format(time.RFC3339)}
			if v.Duration > 0 {
				s.Duration = ptrF(v.Duration)
			}
			for _, c := range []struct {
				src *float64
				dst **int64
			}{{v.Views, &s.Views}, {v.Likes, &s.Likes}, {v.Comments, &s.Comments}, {v.Shares, &s.Shares}} {
				if c.src != nil {
					*c.dst = ptrI(*c.src)
				}
			}
			out = append(out, s)
		}
		if !r.Data.HasMore || len(r.Data.Videos) == 0 {
			break
		}
		cursor = r.Data.Cursor
	}
	return out[:min(len(out), statsMax)], nil
}

// --- refresh ---

var (
	statsBusy atomic.Bool
	statsMu   sync.Mutex
	statsErr  = map[int64]string{} // social_accounts.id -> what the last refresh couldn't read. ponytail: in memory, so a restart forgets it until the next refresh
)

func accountStats(a map[string]any) ([]videoStat, error) {
	a["token"], a["refresh"] = keys.OpenText(store.Str(a["token"])), keys.OpenText(store.Str(a["refresh"]))
	tok, err := accessToken(a)
	if err != nil {
		return nil, fmt.Errorf("could not refresh the login (%v); connect the account again", err)
	}
	id := store.Str(a["account_id"])
	switch store.Str(a["platform"]) {
	case "facebook":
		return facebookStats(id, tok)
	case "instagram":
		return instagramStats(id, tok)
	case "youtube":
		return youtubeStats(tok)
	case "tiktok":
		return tiktokStats(tok)
	}
	return nil, nil
}

// saveStats upserts what was read. A number this read couldn't get keeps its last known value.
func saveStats(account int64, platform string, vs []videoStat) {
	for _, v := range vs {
		curve := ""
		if len(v.Curve) > 0 {
			b, _ := json.Marshal(v.Curve)
			curve = string(b)
		}
		store.DB.Exec(`insert into video_stats(account_id, media_id, platform, title, url, thumb, published_at, duration, views, reach, likes, comments,
			  shares, saves, watch_avg, retention, skip_rate, curve, fetched_at) values(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,current_timestamp)
			on conflict(account_id, media_id) do update set title=excluded.title, url=excluded.url, thumb=excluded.thumb,
			  published_at=excluded.published_at, duration=coalesce(excluded.duration, duration), views=coalesce(excluded.views, views),
			  reach=coalesce(excluded.reach, reach), likes=coalesce(excluded.likes, likes), comments=coalesce(excluded.comments, comments),
			  shares=coalesce(excluded.shares, shares), saves=coalesce(excluded.saves, saves), watch_avg=coalesce(excluded.watch_avg, watch_avg),
			  retention=coalesce(excluded.retention, retention), skip_rate=coalesce(excluded.skip_rate, skip_rate),
			  curve=coalesce(nullif(excluded.curve, ''), curve), fetched_at=excluded.fetched_at`,
			account, v.MediaID, platform, textutil.Cut(strings.TrimSpace(v.Title), 300), v.URL, v.Thumb, v.Published, val(v.Duration), val(v.Views),
			val(v.Reach), val(v.Likes), val(v.Comments), val(v.Shares), val(v.Saves), val(v.WatchAvg), val(v.Retention), val(v.SkipRate), curve)
	}
}

// refreshStats reads every supported account, or one brand's when tenant > 0. Only one refresh runs at a time; it
// reports false when another was already going.
func refreshStats(tenant int64) bool {
	if !statsBusy.CompareAndSwap(false, true) {
		return false
	}
	defer statsBusy.Store(false)
	q, args := "select * from social_accounts where platform in ('"+strings.Join(statsPlatforms, "','")+"')", []any{}
	if tenant > 0 {
		q, args = q+" and tenant_id=?", append(args, tenant)
	}
	as, _ := store.Rows(q, args...)
	for _, a := range as {
		vs, err := accountStats(a)
		saveStats(a["id"].(int64), store.Str(a["platform"]), vs)
		statsMu.Lock()
		if err != nil {
			statsErr[a["id"].(int64)] = err.Error()
		} else {
			delete(statsErr, a["id"].(int64))
		}
		statsMu.Unlock()
	}
	return true
}

// StatsLoop refreshes the numbers every 6 hours while 37Studio runs; the platforms update theirs about that slowly.
func StatsLoop() {
	time.Sleep(2 * time.Minute) // after startup and the first inbox round
	for {
		refreshStats(0)
		time.Sleep(6 * time.Hour)
	}
}

// RefreshStats reads fresh numbers in the background; GET /api/stats shows running until it's done.
func RefreshStats(r *http.Request) (any, error) {
	b, _ := httpapi.Body(r)
	tenant := int64(store.Num(b["tenant_id"]))
	go refreshStats(tenant)
	return map[string]any{"running": true}, nil
}

// --- reading ---

const engagementSQL = "(coalesce(s.likes,0)+coalesce(s.comments,0)+coalesce(s.shares,0)+coalesce(s.saves,0))"

var statsOrder = map[string]string{
	"views": "s.views", "retention": "s.retention", "watch": "s.watch_avg", "reach": "s.reach", "engagement": engagementSQL,
	"rate": engagementSQL + "*100.0/nullif(s.views,0)", "recent": "s.published_at",
}

// A post's numbers: the video_stats row for the video it created on that account.
const postStatsCols = `s.views, s.reach, s.retention, s.watch_avg, s.likes, s.comments, s.shares, s.saves, s.skip_rate,
  (case when s.id is not null then ` + engagementSQL + ` end) as engagement, round(` + engagementSQL + `*100.0/nullif(s.views,0), 2) as engagement_rate`

const postStatsJoin = "left join video_stats s on s.account_id=p.account_id and s.media_id=p.remote_id and p.remote_id<>''"

// publishedVideos groups what 37Studio posted by video (project and file), newest first, with every platform's numbers
// side by side and totals, so one video's platforms compare at a glance.
func publishedVideos(tenant int64) ([]map[string]any, error) {
	rows, err := store.Rows(`select p.project_id, pr.title as project, pr.tenant_id, t.name as brand, p.file, p.platform, p.account_name as account,
	  p.url, p.publish_at, `+postStatsCols+`
	  from posts p join projects pr on pr.id=p.project_id join tenants t on t.id=pr.tenant_id `+postStatsJoin+`
	  where p.status='posted' and (?=0 or pr.tenant_id=?) order by p.publish_at desc, p.id desc`, tenant, tenant)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	at := map[string]map[string]any{}
	for _, r := range rows {
		key := fmt.Sprint(r["project_id"], "/", r["file"])
		v := at[key]
		if v == nil {
			v = map[string]any{"project_id": r["project_id"], "project": r["project"], "tenant_id": r["tenant_id"], "brand": r["brand"], "file": r["file"],
				"url": fmt.Sprintf("/files/projects/%v/%s", r["project_id"], r["file"]), "posted_at": r["publish_at"], "posts": []map[string]any{},
				"views": 0.0, "engagement": 0.0, "engagement_rate": nil, "best": nil}
			at[key] = v
			out = append(out, v)
		}
		post := map[string]any{}
		for _, k := range []string{"platform", "account", "url", "publish_at", "views", "reach", "retention", "watch_avg", "likes", "comments", "shares",
			"saves", "skip_rate", "engagement", "engagement_rate"} {
			post[k] = r[k]
		}
		v["posts"] = append(v["posts"].([]map[string]any), post)
		v["views"] = v["views"].(float64) + store.Num(r["views"])
		v["engagement"] = v["engagement"].(float64) + store.Num(r["engagement"])
		if r["views"] != nil && (v["best"] == nil || store.Num(r["views"]) > store.Num(v["best_views"])) {
			v["best"], v["best_views"] = r["platform"], r["views"]
		}
	}
	for _, v := range out {
		if views := v["views"].(float64); views > 0 {
			v["engagement_rate"] = math.Round(v["engagement"].(float64)/views*1000) / 10
		}
		delete(v, "best_views")
	}
	return out, nil
}

// ListPublished is GET /api/stats/videos: publishedVideos for one brand, or all of them.
func ListPublished(r *http.Request) (any, error) {
	tenant, _ := strconv.ParseInt(r.URL.Query().Get("tenant"), 10, 64)
	return publishedVideos(tenant)
}

// statsRows lists a brand's videos (every brand's when tenant is 0) with engagement (likes + comments + shares + saves)
// and engagement rate (engagement per 100 views), and the 37Studio project a video came from.
func statsRows(tenant int64, platform, by string, asc bool, limit int) ([]map[string]any, error) {
	order, ok := statsOrder[by]
	if !ok {
		order = statsOrder["views"]
	}
	dir := "desc"
	if asc {
		dir = "asc"
	}
	q := `select s.media_id, s.platform, s.title, s.url, s.thumb, s.published_at, s.duration, s.views, s.reach, s.likes, s.comments, s.shares, s.saves,
	  s.watch_avg, s.retention, s.skip_rate, s.curve, s.fetched_at, s.account_id, a.tenant_id, a.name as account, t.name as brand,
	  ` + engagementSQL + ` as engagement, round(` + statsOrder["rate"] + `, 2) as engagement_rate, p.project_id, pr.title as project
	  from video_stats s join social_accounts a on a.id=s.account_id join tenants t on t.id=a.tenant_id
	  left join posts p on p.id=(select max(id) from posts where account_id=s.account_id and remote_id=s.media_id and remote_id<>'')
	  left join projects pr on pr.id=p.project_id where 1=1`
	var args []any
	if tenant > 0 {
		q, args = q+" and a.tenant_id=?", append(args, tenant)
	}
	if platform != "" {
		q, args = q+" and s.platform=?", append(args, platform)
	}
	q += " order by " + order + " " + dir + " nulls last, s.published_at desc limit ?"
	rows, err := store.Rows(q, append(args, cmp.Or(limit, 200))...)
	for _, r := range rows {
		var c []float64
		json.Unmarshal([]byte(store.Str(r["curve"])), &c)
		r["curve"] = c
	}
	return rows, err
}

// ListStats is the Performance page and the video_stats tool: the videos, sorted, plus totals for the same filter.
func ListStats(r *http.Request) (any, error) {
	q := r.URL.Query()
	tenant, _ := strconv.ParseInt(q.Get("tenant"), 10, 64)
	limit, _ := strconv.Atoi(q.Get("limit"))
	rows, err := statsRows(tenant, q.Get("platform"), q.Get("sort"), q.Get("order") == "asc", min(cmp.Or(limit, 200), 1000))
	if err != nil {
		return nil, err
	}
	all, _ := statsRows(tenant, q.Get("platform"), "views", false, 1000)
	var views, engaged, retSum float64
	var retN int
	updated := ""
	type plat struct {
		Platform   string   `json:"platform"`
		Videos     int      `json:"videos"`
		Views      float64  `json:"views"`
		Engagement float64  `json:"engagement"`
		Rate       *float64 `json:"engagement_rate"` // per 100 views
		Retention  *float64 `json:"retention"`       // average of the videos that report it
		retSum     float64
		retN       int
	}
	by := map[string]*plat{}
	for _, v := range all {
		views += store.Num(v["views"])
		engaged += store.Num(v["engagement"])
		if v["retention"] != nil {
			retSum, retN = retSum+store.Num(v["retention"]), retN+1
		}
		updated = max(updated, store.Str(v["fetched_at"]))
		p := store.Str(v["platform"])
		if by[p] == nil {
			by[p] = &plat{Platform: p}
		}
		by[p].Videos++
		by[p].Views += store.Num(v["views"])
		by[p].Engagement += store.Num(v["engagement"])
		if v["retention"] != nil {
			by[p].retSum, by[p].retN = by[p].retSum+store.Num(v["retention"]), by[p].retN+1
		}
	}
	summary := map[string]any{"videos": len(all), "views": views, "engagement": engaged, "updated_at": updated, "retention": nil, "engagement_rate": nil}
	if retN > 0 {
		summary["retention"] = math.Round(retSum/float64(retN)*10) / 10
	}
	if views > 0 {
		summary["engagement_rate"] = math.Round(engaged/views*1000) / 10
	}
	platforms := []*plat{}
	for _, p := range by {
		if p.Views > 0 {
			p.Rate = ptrF(p.Engagement / p.Views * 100)
		}
		if p.retN > 0 {
			p.Retention = ptrF(p.retSum / float64(p.retN))
		}
		platforms = append(platforms, p)
	}
	sort.Slice(platforms, func(i, j int) bool { return platforms[i].Views > platforms[j].Views })
	summary["platforms"] = platforms

	// Accounts this filter covers that the last refresh couldn't (fully) read.
	accts, _ := store.Rows("select a.id, a.platform, a.name, t.name as brand from social_accounts a join tenants t on t.id=a.tenant_id where (?=0 or a.tenant_id=?)", tenant, tenant)
	errs := []map[string]any{}
	statsMu.Lock()
	for _, a := range accts {
		if e := statsErr[a["id"].(int64)]; e != "" {
			errs = append(errs, map[string]any{"account": a["name"], "platform": a["platform"], "brand": a["brand"], "error": e})
		}
	}
	statsMu.Unlock()
	out := map[string]any{"items": rows, "summary": summary, "errors": errs, "running": statsBusy.Load()}
	if tenant > 0 {
		out["reviewing"], out["review_error"] = reviewState(tenant)
	}
	return out, nil
}

// StatsCLI is `37studio stats`: the editor agent's view of how this brand's videos did, one JSON object a line.
func StatsCLI(args []string) int {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	by := fs.String("sort", "views", "views, retention, engagement, rate, watch, reach or recent")
	asc := fs.Bool("asc", false, "lowest first: the weakest videos")
	platform := fs.String("platform", "", "only facebook, instagram, youtube or tiktok")
	limit := fs.Int("limit", 15, "how many videos")
	videos := fs.Bool("videos", false, "only videos made in 37Studio, each with its numbers on every platform it was posted to")
	if fs.Parse(args) != nil {
		return 2
	}
	config.DataDir, _ = filepath.Abs(config.DataDir)
	store.Open()
	pid, _ := strconv.ParseInt(config.Env("STUDIO37_PROJECT", ""), 10, 64)
	p, err := store.Row("select tenant_id from projects where id=?", pid)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: run this inside a project (STUDIO37_PROJECT is not set)")
		return 1
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	if *videos {
		vs, err := publishedVideos(p["tenant_id"].(int64))
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		for _, v := range vs[:min(len(vs), *limit)] {
			for _, k := range []string{"tenant_id", "brand", "url"} {
				delete(v, k)
			}
			enc.Encode(v)
		}
		if len(vs) == 0 {
			fmt.Fprintln(os.Stderr, "nothing posted from 37Studio yet")
		}
		return 0
	}
	rows, err := statsRows(p["tenant_id"].(int64), *platform, *by, *asc, *limit)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if len(rows) == 0 {
		fmt.Fprintln(os.Stderr, "no numbers yet: this brand has no connected Facebook, Instagram, YouTube or TikTok account, or 37Studio hasn't read them yet")
	}
	for _, r := range rows {
		for _, k := range []string{"thumb", "account_id", "tenant_id", "brand", "fetched_at"} {
			delete(r, k)
		}
		enc.Encode(r)
	}
	return 0
}
