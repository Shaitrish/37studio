package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Several videos from one source. The agent in the source project (say, a 2-hour class with 10 solved questions) plans
// the videos and queues each with `37studio clip`. Each becomes its own project, with the same company, campaign,
// format and quality, a fresh agent, and its own preview, Export and Publish. clipLoop builds and renders them one at
// a time, and every finished video lands in the Library.

const clipRules = `

## One of several videos from the same source
Made from the project %q (#%d). Use %s; the full recording is in input/. It is already transcribed: work/transcript/ (transcript.json has word timestamps in the full recording's time), so don't transcribe again.
Nobody is watching while this is built, and the videos are made one after another: decide everything yourself from this brief, the knowledge base and the brand library, without asking questions. Build the video, check it visually and finish in this one turn. Don't render the final; the app does that when you stop.`

func clipCLI(args []string) int {
	fs := flag.NewFlagSet("clip", flag.ContinueOnError)
	title := fs.String("title", "", "the video's title")
	brief := fs.String("brief", "", "what this video covers: the moment, the hook, the point, the call to action")
	from := fs.String("from", "", "where it starts in the source, HH:MM:SS")
	to := fs.String("to", "", "where it ends in the source, HH:MM:SS")
	seconds := fs.Int("seconds", 0, "target length in seconds (default: the source project's)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *title == "" || *brief == "" {
		fmt.Fprintln(os.Stderr, `usage: 37studio clip --title "Q3: ..." --brief "..." [--from 00:12:30 --to 00:19:10] [--seconds 60]`)
		return 2
	}
	dataDir, _ = filepath.Abs(dataDir)
	openDB()
	pid, _ := strconv.ParseInt(env("STUDIO37_PROJECT", ""), 10, 64)
	id, err := queueClip(pid, *title, *brief, *from, *to, *seconds)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Printf("Queued video #%d %q. The app builds and renders queued videos one at a time; each appears in the Library when it's done.\n", id, *title)
	return 0
}

func queueClip(parent int64, title, brief, from, to string, seconds int) (int64, error) {
	p, err := row("select * from projects where id=?", parent)
	if err != nil {
		return 0, errors.New("run this inside a project (STUDIO37_PROJECT is not set)")
	}
	if !licensed() && trialLeft() == 0 {
		return 0, fmt.Errorf("the free trial covers %d videos and they're used up; a license (Settings → License) makes more", trialProjects)
	}
	span := "the parts of the recording this brief describes"
	if from != "" || to != "" {
		span = fmt.Sprintf("%s to %s of the recording", or(from, "the start"), or(to, "the end"))
	}
	if seconds <= 0 {
		seconds = int(num(p["seconds"]))
	}
	id, err := save("projects", projectCols, 0, map[string]any{
		"tenant_id": p["tenant_id"], "campaign_id": p["campaign_id"], "title": title, "kind": p["kind"], "aspect": p["aspect"],
		"seconds": seconds, "quality": p["quality"], "language": p["language"], "focus": p["focus"],
		"brief": brief + fmt.Sprintf(clipRules, str(p["title"]), parent, span),
	})
	if err != nil {
		return 0, err
	}
	db.Exec("update projects set parent_id=?, clip_stage='queued' where id=?", parent, id)
	db.Exec("insert into settings(key,value) values('projects_made','1') on conflict(key) do update set value=cast(value as integer)+1")
	dir, src := projectDir(id), projectDir(parent)
	for _, d := range []string{"input", "work/transcript", "out/drafts"} {
		os.MkdirAll(filepath.Join(dir, d), 0o755)
	}
	// The recording is shared, not copied: a 2-hour class is gigabytes. A hard link survives deleting either project.
	for _, f := range listFiles(filepath.Join(src, "input"), "", true) {
		from, to := filepath.Join(src, "input", f.Path), filepath.Join(dir, "input", f.Path)
		os.MkdirAll(filepath.Dir(to), 0o755)
		if os.Link(from, to) != nil {
			os.Symlink(from, to)
		}
	}
	for _, name := range []string{"transcript.json", "transcript.srt", "transcript.txt", "meta.json"} {
		copyFile(filepath.Join(src, "work", "transcript", name), filepath.Join(dir, "work", "transcript", name))
	}
	return id, nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}

// ---- queue

var clipMu sync.Mutex

func clipLoop() {
	db.Exec(`update projects set clip_stage='failed' where clip_stage in ('building','rendering')`) // a restart killed them
	for {
		clipTick()
		time.Sleep(30 * time.Second)
	}
}

// clipTick starts the oldest queued video. ponytail: one at a time across all projects, and after any autopilot video,
// because a PC renders one well, not three badly.
func clipTick() {
	clipMu.Lock()
	defer clipMu.Unlock()
	var busy int
	db.QueryRow("select count(*) from projects where clip_stage in ('building','rendering')").Scan(&busy)
	if busy > 0 || autopilotBusy() {
		return
	}
	var id int64
	if db.QueryRow("select id from projects where clip_stage='queued' order by id limit 1").Scan(&id) != nil {
		return
	}
	db.Exec("update projects set clip_stage='building' where id=?", id)
	addEvent(id, "info", map[string]string{"text": "Queued from its source project; the agent is building it now."})
	if err := startRun(id, "", "", false); err != nil {
		clipFail(id, "could not start the agent: "+err.Error())
	}
}

func clipFail(id int64, msg string) {
	db.Exec("update projects set clip_stage='failed' where id=?", id)
	addEvent(id, "error", map[string]string{"text": msg})
}

// clipAdvance moves a queued video on after its agent turn (render it) and after its render (done), then starts the next.
func clipAdvance(id int64) {
	clipMu.Lock()
	clipStep(id)
	clipMu.Unlock()
	clipTick()
}

func clipStep(id int64) {
	if isRunning(id) {
		return
	}
	var stage string
	db.QueryRow("select clip_stage from projects where id=?", id).Scan(&stage)
	dir := projectDir(id)
	switch stage {
	case "building":
		if _, err := os.Stat(filepath.Join(dir, "comp", "index.html")); err != nil {
			clipFail(id, "The agent finished without a video. Read its last message, then ask it to continue.")
			return
		}
		db.Exec("update projects set clip_stage='rendering' where id=?", id)
		if err := startRender(id, "", "high"); err != nil {
			clipFail(id, "Export failed to start: "+err.Error())
		}
	case "rendering":
		for _, f := range listFiles(filepath.Join(dir, "out"), "", false) {
			if f.Kind == "video" {
				db.Exec("update projects set clip_stage='done' where id=?", id)
				return
			}
		}
		clipFail(id, "The export failed. Open the chat to see why, then press Export again.")
	}
}

// ---- library

// listLibrary is every finished video and image (each project's out/, not drafts) across companies, newest first.
func listLibrary(r *http.Request) (any, error) {
	ps, err := rows("select p.id, p.title, p.tenant_id, t.name as tenant_name, p.parent_id from projects p join tenants t on t.id=p.tenant_id")
	out := []map[string]any{}
	for _, p := range ps {
		id := p["id"].(int64)
		for _, f := range listFiles(filepath.Join(projectDir(id), "out"), fmt.Sprintf("/files/projects/%d/out", id), false) {
			if f.Kind == "video" || f.Kind == "image" {
				out = append(out, map[string]any{"file": f, "project_id": id, "title": p["title"], "tenant_id": p["tenant_id"], "tenant_name": p["tenant_name"]})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["file"].(fileInfo).Mod.After(out[j]["file"].(fileInfo).Mod) })
	return out, err
}
