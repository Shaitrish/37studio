// Package autopilot makes and posts videos on a schedule from each brand's content plans.
package autopilot

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"

	"github.com/Shaitrish/37studio/internal/agent"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/social"
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/textutil"
)

const autopilotLead = 3 * time.Hour // start this long before the slot so the video is ready to approve in time

var autopilotCols = []string{"tenant_id", "name", "enabled", "brief", "kind", "aspect", "seconds", "language", "quality", "days", "at", "accounts", "approval"}

func userLoc() *time.Location {
	if l, err := time.LoadLocation(store.Setting("timezone")); err == nil {
		return l
	}
	return time.UTC
}

// nextSlot is the first posting time after `after` on one of `days` (0 = Sunday) at "HH:MM" local time.
func nextSlot(days string, at string, after time.Time) (time.Time, bool) {
	var h, m int
	if _, err := fmt.Sscanf(at, "%d:%d", &h, &m); err != nil {
		return time.Time{}, false
	}
	loc := userLoc()
	t := after.In(loc)
	for i := 0; i < 8; i++ {
		d := time.Date(t.Year(), t.Month(), t.Day()+i, h, m, 0, 0, loc)
		if d.After(after) && slices.Contains(strings.Split(days, ","), strconv.Itoa(int(d.Weekday()))) {
			return d, true
		}
	}
	return time.Time{}, false
}

func List(r *http.Request) (any, error) {
	aps, err := store.Rows("select * from autopilots where tenant_id=? order by id", r.URL.Query().Get("tenant"))
	for _, ap := range aps {
		if s, ok := nextSlot(store.Str(ap["days"]), store.Str(ap["at"]), time.Now()); ok && store.Num(ap["enabled"]) == 1 {
			ap["next"] = s.UTC().Format(time.RFC3339)
		}
		var ids []int64
		json.Unmarshal([]byte(store.Str(ap["accounts"])), &ids)
		ap["accounts"] = ids
		ap["videos"], _ = store.Rows("select id, title, autopilot_slot, autopilot_stage, created_at from projects where autopilot_id=? order by id desc limit 12", ap["id"])
	}
	return aps, err
}

func Save(r *http.Request) (any, error) {
	b, err := httpapi.Body(r)
	if err != nil {
		return nil, err
	}
	if _, ok := b["brief"]; (ok || httpapi.PathID(r) == 0) && strings.TrimSpace(store.Str(b["brief"])) == "" {
		return nil, httpapi.Bad("say what the videos should be about")
	}
	if tz := store.Str(b["tz"]); tz != "" { // the browser's zone becomes the schedule's zone
		if _, err := time.LoadLocation(tz); err == nil {
			store.DB.Exec("insert into settings(key,value) values('timezone',?) on conflict(key) do update set value=excluded.value", tz)
		}
	}
	if a, ok := b["accounts"].([]any); ok {
		j, _ := json.Marshal(a)
		b["accounts"] = string(j)
	}
	if d, ok := b["days"].([]any); ok {
		var ds []string
		for _, x := range d {
			ds = append(ds, store.Str(x))
		}
		b["days"] = strings.Join(ds, ",")
	}
	for _, k := range []string{"enabled", "approval"} {
		if v, ok := b[k].(bool); ok {
			b[k] = map[bool]int{true: 1, false: 0}[v]
		}
	}
	id := httpapi.PathID(r)
	if id != 0 {
		delete(b, "tenant_id")
	}
	id, err = store.Save("autopilots", autopilotCols, id, b)
	if err != nil {
		return nil, err
	}
	return store.Row("select * from autopilots where id=?", id)
}

func Delete(r *http.Request) (any, error) {
	store.DB.Exec("update projects set autopilot_id=null where autopilot_id=?", httpapi.PathID(r)) // its videos stay
	_, err := store.DB.Exec("delete from autopilots where id=?", httpapi.PathID(r))
	return nil, err
}

// Run makes one video now; its posts go out as soon as they are approved (or at once without approval).
func Run(r *http.Request) (any, error) {
	ap, err := store.Row("select * from autopilots where id=?", httpapi.PathID(r))
	if err != nil {
		return nil, err
	}
	if Busy() {
		return nil, httpapi.Bad("Autopilot is already making a video; one at a time")
	}
	id, err := autopilotStart(ap, time.Now())
	return map[string]int64{"project": id}, err
}

func Approve(r *http.Request) (any, error) {
	id := httpapi.PathID(r)
	if _, err := store.DB.Exec("update posts set status='scheduled', publish_at=max(publish_at, datetime('now')) where project_id=? and status='approval'", id); err != nil {
		return nil, err
	}
	store.DB.Exec("update projects set autopilot_stage='scheduled' where id=? and autopilot_stage='approve'", id)
	go social.RunDuePosts()
	return nil, nil
}

func Skip(r *http.Request) (any, error) {
	id := httpapi.PathID(r)
	store.DB.Exec("delete from posts where project_id=? and status='approval'", id)
	_, err := store.DB.Exec("update projects set autopilot_stage='skipped' where id=?", id)
	return nil, err
}

var apMu sync.Mutex

func Busy() bool {
	var n int
	store.DB.QueryRow("select count(*) from projects where autopilot_stage in ('building','rendering','captions')").Scan(&n)
	return n > 0
}

func Loop() {
	store.DB.Exec(`update projects set autopilot_stage='failed' where autopilot_stage in ('building','rendering','captions')`) // a restart killed them
	for {
		autopilotTick()
		time.Sleep(time.Minute)
	}
}

// autopilotTick starts the next due video. ponytail: one at a time across all brands; a PC renders one well, not three badly.
func autopilotTick() {
	if Busy() {
		return
	}
	aps, _ := store.Rows("select * from autopilots where enabled=1 order by id")
	for _, ap := range aps {
		slot, ok := nextSlot(store.Str(ap["days"]), store.Str(ap["at"]), time.Now())
		key := slot.UTC().Format(time.DateTime)
		if !ok || time.Until(slot) > autopilotLead || store.Str(ap["last_slot"]) == key {
			continue
		}
		store.DB.Exec("update autopilots set last_slot=? where id=?", key, ap["id"])
		autopilotStart(ap, slot)
		return
	}
}

const autopilotRules = `

## Autopilot
This video is made on autopilot for %s. Nobody will answer questions: decide everything yourself from the brief, the knowledge base and the brand library, then build the video, check it visually and finish it in this one turn. Don't render the final; the app does that when you stop.
Pick a fresh topic or angle the brand hasn't covered yet.%s
Before you stop, write work/social.json (the post captions) and work/topic.txt (the topic in 3-8 words).`

func autopilotStart(ap map[string]any, slot time.Time) (int64, error) {
	tenant := ap["tenant_id"].(int64)
	var made []string
	prev, _ := store.Rows("select title from projects where autopilot_id=? order by id desc limit 30", ap["id"])
	for _, p := range prev {
		made = append(made, "\n- "+store.Str(p["title"]))
	}
	already := ""
	if len(made) > 0 {
		already = " Already made, don't repeat:" + strings.Join(made, "")
	}
	when := slot.In(userLoc()).Format("Monday 2 January, 15:04")
	id, err := store.Save("projects", store.ProjectCols, 0, map[string]any{
		"tenant_id": tenant, "title": cmp.Or(store.Str(ap["name"]), "Autopilot") + " · " + slot.In(userLoc()).Format("Mon 2 Jan"),
		"kind": ap["kind"], "aspect": ap["aspect"], "seconds": ap["seconds"], "quality": ap["quality"], "language": ap["language"],
		"brief": store.Str(ap["brief"]) + fmt.Sprintf(autopilotRules, when, already),
	})
	if err != nil {
		return 0, err
	}
	for _, d := range []string{"input", "work", "out/drafts"} {
		os.MkdirAll(filepath.Join(files.ProjectDir(id), d), 0o755)
	}
	store.DB.Exec("update projects set autopilot_id=?, autopilot_slot=?, autopilot_stage='building' where id=?", ap["id"], slot.UTC().Format(time.DateTime), id)
	agent.AddEvent(id, "info", map[string]string{"text": "Autopilot started this video for " + when + "."})
	if err := agent.StartRun(id, "", "", false); err != nil {
		autopilotFail(id, "could not start the agent: "+err.Error())
		return id, err
	}
	return id, nil
}

func autopilotFail(id int64, msg string) {
	store.DB.Exec("update projects set autopilot_stage='failed' where id=?", id)
	agent.AddEvent(id, "error", map[string]string{"text": "Autopilot stopped: " + msg})
}

// Advance moves an autopilot video to its next stage; drainQueue calls it after every agent turn and render.
func Advance(id int64) {
	apMu.Lock()
	defer apMu.Unlock()
	if agent.IsRunning(id) {
		return
	}
	p, err := store.Row("select p.*, a.accounts, a.approval from projects p join autopilots a on a.id=p.autopilot_id where p.id=?", id)
	if err != nil {
		return // not an autopilot video
	}
	dir := files.ProjectDir(id)
	switch store.Str(p["autopilot_stage"]) {
	case "building":
		if _, err := os.Stat(filepath.Join(dir, "comp", "index.html")); err != nil {
			autopilotFail(id, "the agent finished without a video. Open the chat to see why.")
			return
		}
		if t, _ := os.ReadFile(filepath.Join(dir, "work", "topic.txt")); strings.TrimSpace(string(t)) != "" {
			store.DB.Exec("update projects set title=? where id=?", textutil.Head(strings.TrimSpace(string(t)), 120), id)
		}
		store.DB.Exec("update projects set autopilot_stage='rendering' where id=?", id)
		if err := agent.StartRender(id, "", "high"); err != nil {
			autopilotFail(id, "render: "+err.Error())
		}
	case "rendering":
		var video string
		for _, f := range files.List(filepath.Join(dir, "out"), "", false) { // newest first
			if f.Kind == "video" {
				video = "out/" + f.Path
				break
			}
		}
		if video == "" {
			autopilotFail(id, "the render failed. Open the chat to see why.")
			return
		}
		store.DB.Exec("update projects set autopilot_stage='captions' where id=?", id)
		caps := social.Captions(id)
		if len(caps) == 0 {
			if caps, err = social.MakeCaptions(id); err != nil {
				autopilotFail(id, "captions: "+err.Error())
				return
			}
		}
		var accounts []int64
		json.Unmarshal([]byte(store.Str(p["accounts"])), &accounts)
		at, _ := time.Parse(time.DateTime, store.Str(p["autopilot_slot"]))
		if at.Before(time.Now().Add(2 * time.Minute)) {
			at = time.Now().UTC().Add(2 * time.Minute)
		}
		approval := store.Num(p["approval"]) == 1
		status := map[bool]string{true: "approval", false: "scheduled"}[approval]
		for _, aid := range accounts {
			a, err := store.Row("select platform, name from social_accounts where id=? and tenant_id=? and platform<>'whatsapp'", aid, p["tenant_id"])
			if err != nil {
				continue // disconnected since the plan was made
			}
			pl, title := store.Str(a["platform"]), ""
			if pl == "youtube" || pl == "pinterest" {
				title = cmp.Or(store.Str(caps[pl+"_title"]), store.Str(p["title"]))
			}
			privacy := map[string]string{"youtube": "public", "tiktok": "SELF_ONLY"}[pl] // unaudited TikTok apps may only post privately
			store.DB.Exec(`insert into posts(project_id,account_id,platform,account_name,file,title,caption,privacy,status,publish_at) values(?,?,?,?,?,?,?,?,?,?)`,
				id, aid, pl, a["name"], video, title, store.Str(caps[pl]), privacy, status, at.Format(time.DateTime))
		}
		stage := map[bool]string{true: "approve", false: "scheduled"}[approval]
		store.DB.Exec("update projects set autopilot_stage=? where id=?", stage, id)
		if approval {
			agent.AddEvent(id, "info", map[string]string{"text": "Autopilot: the video and captions are ready. Approve them on this page to post."})
		} else {
			agent.AddEvent(id, "info", map[string]string{"text": "Autopilot: the video is scheduled to post at " + at.In(userLoc()).Format("Mon 2 Jan 15:04") + "."})
			go social.RunDuePosts()
		}
	}
}
