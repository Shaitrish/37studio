package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // schedules are in the user's own time zone, also inside Docker
)

// Autopilot: per company content plans ("3 reels a week about X, 8pm, to these accounts"). Ahead of each slot it starts a
// project, lets the agent pick a fresh topic and build the video without questions, renders it, writes captions, and
// queues the posts: waiting for one click when approval is on, else straight onto the schedule.
//
//	building (agent) -> rendering -> captions -> approve | scheduled        or failed / skipped
//
// It runs while 37Studio runs; a slot that passed while the computer was off is skipped, not posted late.

const autopilotLead = 3 * time.Hour // start this long before the slot so the video is ready to approve in time

var autopilotCols = []string{"tenant_id", "name", "enabled", "brief", "kind", "aspect", "seconds", "language", "quality", "days", "at", "accounts", "approval"}

func userLoc() *time.Location {
	if l, err := time.LoadLocation(setting("timezone")); err == nil {
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
		if d.After(after) && contains(strings.Split(days, ","), strconv.Itoa(int(d.Weekday()))) {
			return d, true
		}
	}
	return time.Time{}, false
}

// ---- API

func listAutopilots(r *http.Request) (any, error) {
	aps, err := rows("select * from autopilots where tenant_id=? order by id", r.URL.Query().Get("tenant"))
	for _, ap := range aps {
		if s, ok := nextSlot(str(ap["days"]), str(ap["at"]), time.Now()); ok && num(ap["enabled"]) == 1 {
			ap["next"] = s.UTC().Format(time.RFC3339)
		}
		var ids []int64
		json.Unmarshal([]byte(str(ap["accounts"])), &ids)
		ap["accounts"] = ids
		ap["videos"], _ = rows("select id, title, autopilot_slot, autopilot_stage, created_at from projects where autopilot_id=? order by id desc limit 12", ap["id"])
	}
	return aps, err
}

func saveAutopilot(r *http.Request) (any, error) {
	b, err := body(r)
	if err != nil {
		return nil, err
	}
	if _, ok := b["brief"]; (ok || pathID(r) == 0) && strings.TrimSpace(str(b["brief"])) == "" {
		return nil, bad("say what the videos should be about")
	}
	if tz := str(b["tz"]); tz != "" { // the browser's zone becomes the schedule's zone
		if _, err := time.LoadLocation(tz); err == nil {
			db.Exec("insert into settings(key,value) values('timezone',?) on conflict(key) do update set value=excluded.value", tz)
		}
	}
	if a, ok := b["accounts"].([]any); ok {
		j, _ := json.Marshal(a)
		b["accounts"] = string(j)
	}
	if d, ok := b["days"].([]any); ok {
		var ds []string
		for _, x := range d {
			ds = append(ds, str(x))
		}
		b["days"] = strings.Join(ds, ",")
	}
	for _, k := range []string{"enabled", "approval"} {
		if v, ok := b[k].(bool); ok {
			b[k] = map[bool]int{true: 1, false: 0}[v]
		}
	}
	id := pathID(r)
	if id != 0 {
		delete(b, "tenant_id")
	}
	id, err = save("autopilots", autopilotCols, id, b)
	if err != nil {
		return nil, err
	}
	return row("select * from autopilots where id=?", id)
}

func deleteAutopilot(r *http.Request) (any, error) {
	db.Exec("update projects set autopilot_id=null where autopilot_id=?", pathID(r)) // its videos stay
	_, err := db.Exec("delete from autopilots where id=?", pathID(r))
	return nil, err
}

// runAutopilot makes one video now; its posts go out as soon as they are approved (or at once without approval).
func runAutopilot(r *http.Request) (any, error) {
	ap, err := row("select * from autopilots where id=?", pathID(r))
	if err != nil {
		return nil, err
	}
	if autopilotBusy() {
		return nil, bad("Autopilot is already making a video; one at a time")
	}
	id, err := autopilotStart(ap, time.Now())
	return map[string]int64{"project": id}, err
}

func approveProject(r *http.Request) (any, error) {
	id := pathID(r)
	if _, err := db.Exec("update posts set status='scheduled', publish_at=max(publish_at, datetime('now')) where project_id=? and status='approval'", id); err != nil {
		return nil, err
	}
	db.Exec("update projects set autopilot_stage='scheduled' where id=? and autopilot_stage='approve'", id)
	go runDuePosts()
	return nil, nil
}

func skipProject(r *http.Request) (any, error) {
	id := pathID(r)
	db.Exec("delete from posts where project_id=? and status='approval'", id)
	_, err := db.Exec("update projects set autopilot_stage='skipped' where id=?", id)
	return nil, err
}

// ---- scheduler

var apMu sync.Mutex

func autopilotBusy() bool {
	var n int
	db.QueryRow("select count(*) from projects where autopilot_stage in ('building','rendering','captions')").Scan(&n)
	return n > 0
}

func autopilotLoop() {
	db.Exec(`update projects set autopilot_stage='failed' where autopilot_stage in ('building','rendering','captions')`) // a restart killed them
	for {
		autopilotTick()
		time.Sleep(time.Minute)
	}
}

// autopilotTick starts the next due video. ponytail: one at a time across all companies; a PC renders one well, not three badly.
func autopilotTick() {
	if autopilotBusy() {
		return
	}
	aps, _ := rows("select * from autopilots where enabled=1 order by id")
	for _, ap := range aps {
		slot, ok := nextSlot(str(ap["days"]), str(ap["at"]), time.Now())
		key := slot.UTC().Format(time.DateTime)
		if !ok || time.Until(slot) > autopilotLead || str(ap["last_slot"]) == key {
			continue
		}
		db.Exec("update autopilots set last_slot=? where id=?", key, ap["id"])
		autopilotStart(ap, slot)
		return
	}
}

const autopilotRules = `

## Autopilot
This video is made on autopilot for %s. Nobody will answer questions: decide everything yourself from the brief, the knowledge base and the brand library, then build the video, check it visually and finish it in this one turn. Don't render the final; the app does that when you stop.
Pick a fresh topic or angle the company hasn't covered yet.%s
Before you stop, write work/social.json (the post captions) and work/topic.txt (the topic in 3-8 words).`

func autopilotStart(ap map[string]any, slot time.Time) (int64, error) {
	tenant := ap["tenant_id"].(int64)
	var made []string
	prev, _ := rows("select title from projects where autopilot_id=? order by id desc limit 30", ap["id"])
	for _, p := range prev {
		made = append(made, "\n- "+str(p["title"]))
	}
	already := ""
	if len(made) > 0 {
		already = " Already made, don't repeat:" + strings.Join(made, "")
	}
	when := slot.In(userLoc()).Format("Monday 2 January, 15:04")
	id, err := save("projects", projectCols, 0, map[string]any{
		"tenant_id": tenant, "title": or(str(ap["name"]), "Autopilot") + " · " + slot.In(userLoc()).Format("Mon 2 Jan"),
		"kind": ap["kind"], "aspect": ap["aspect"], "seconds": ap["seconds"], "quality": ap["quality"], "language": ap["language"],
		"brief": str(ap["brief"]) + fmt.Sprintf(autopilotRules, when, already),
	})
	if err != nil {
		return 0, err
	}
	for _, d := range []string{"input", "work", "out/drafts"} {
		os.MkdirAll(filepath.Join(projectDir(id), d), 0o755)
	}
	db.Exec("update projects set autopilot_id=?, autopilot_slot=?, autopilot_stage='building' where id=?", ap["id"], slot.UTC().Format(time.DateTime), id)
	addEvent(id, "info", map[string]string{"text": "Autopilot started this video for " + when + "."})
	if err := startRun(id, "", "", false); err != nil {
		autopilotFail(id, "could not start the agent: "+err.Error())
		return id, err
	}
	return id, nil
}

func autopilotFail(id int64, msg string) {
	db.Exec("update projects set autopilot_stage='failed' where id=?", id)
	addEvent(id, "error", map[string]string{"text": "Autopilot stopped: " + msg})
}

// autopilotAdvance moves an autopilot video to its next stage; drainQueue calls it after every agent turn and render.
func autopilotAdvance(id int64) {
	apMu.Lock()
	defer apMu.Unlock()
	if isRunning(id) {
		return
	}
	p, err := row("select p.*, a.accounts, a.approval from projects p join autopilots a on a.id=p.autopilot_id where p.id=?", id)
	if err != nil {
		return // not an autopilot video
	}
	dir := projectDir(id)
	switch str(p["autopilot_stage"]) {
	case "building":
		if _, err := os.Stat(filepath.Join(dir, "comp", "index.html")); err != nil {
			autopilotFail(id, "the agent finished without a video. Open the chat to see why.")
			return
		}
		if t, _ := os.ReadFile(filepath.Join(dir, "work", "topic.txt")); strings.TrimSpace(string(t)) != "" {
			db.Exec("update projects set title=? where id=?", head(strings.TrimSpace(string(t)), 120), id)
		}
		db.Exec("update projects set autopilot_stage='rendering' where id=?", id)
		if err := startRender(id, "", "high"); err != nil {
			autopilotFail(id, "render: "+err.Error())
		}
	case "rendering":
		var video string
		for _, f := range listFiles(filepath.Join(dir, "out"), "", false) { // newest first
			if f.Kind == "video" {
				video = "out/" + f.Path
				break
			}
		}
		if video == "" {
			autopilotFail(id, "the render failed. Open the chat to see why.")
			return
		}
		db.Exec("update projects set autopilot_stage='captions' where id=?", id)
		caps := socialCaptions(id)
		if len(caps) == 0 {
			if caps, err = makeCaptions(id); err != nil {
				autopilotFail(id, "captions: "+err.Error())
				return
			}
		}
		var accounts []int64
		json.Unmarshal([]byte(str(p["accounts"])), &accounts)
		at, _ := time.Parse(time.DateTime, str(p["autopilot_slot"]))
		if at.Before(time.Now().Add(2 * time.Minute)) {
			at = time.Now().UTC().Add(2 * time.Minute)
		}
		approval := num(p["approval"]) == 1
		status := map[bool]string{true: "approval", false: "scheduled"}[approval]
		for _, aid := range accounts {
			a, err := row("select platform, name from social_accounts where id=? and tenant_id=? and platform<>'whatsapp'", aid, p["tenant_id"])
			if err != nil {
				continue // disconnected since the plan was made
			}
			pl, title := str(a["platform"]), ""
			if pl == "youtube" || pl == "pinterest" {
				title = or(str(caps[pl+"_title"]), str(p["title"]))
			}
			privacy := map[string]string{"youtube": "public", "tiktok": "SELF_ONLY"}[pl] // unaudited TikTok apps may only post privately
			db.Exec(`insert into posts(project_id,account_id,platform,account_name,file,title,caption,privacy,status,publish_at) values(?,?,?,?,?,?,?,?,?,?)`,
				id, aid, pl, a["name"], video, title, str(caps[pl]), privacy, status, at.Format(time.DateTime))
		}
		stage := map[bool]string{true: "approve", false: "scheduled"}[approval]
		db.Exec("update projects set autopilot_stage=? where id=?", stage, id)
		if approval {
			addEvent(id, "info", map[string]string{"text": "Autopilot: the video and captions are ready. Approve them on this page to post."})
		} else {
			addEvent(id, "info", map[string]string{"text": "Autopilot: the video is scheduled to post at " + at.In(userLoc()).Format("Mon 2 Jan 15:04") + "."})
			go runDuePosts()
		}
	}
}
