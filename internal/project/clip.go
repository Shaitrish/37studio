// Package project holds projects: CRUD, uploads, footage imports, and several videos cut from one source.
package project

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/Shaitrish/37studio/internal/agent"
	"github.com/Shaitrish/37studio/internal/autopilot"
	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/templates"
)

const clipRules = `

## One of several videos from the same source
Made from the project %q (#%d). Use %s; the full recording is in input/. It is already transcribed: work/transcript/ (transcript.json has word timestamps in the full recording's time), so don't transcribe again.
Nobody is watching while this is built, and the videos are made one after another: decide everything yourself from this brief, the knowledge base and the brand library, without asking questions. Build the video, check it visually and finish in this one turn. Don't render the final; the app does that when you stop.`

func ClipCLI(args []string) int {
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
	config.DataDir, _ = filepath.Abs(config.DataDir)
	store.Open()
	pid, _ := strconv.ParseInt(config.Env("STUDIO37_PROJECT", ""), 10, 64)
	id, err := queueClip(pid, *title, *brief, *from, *to, *seconds)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Printf("Queued video #%d %q. The app builds and renders queued videos one at a time; each appears in the Library when it's done.\n", id, *title)
	return 0
}

func queueClip(parent int64, title, brief, from, to string, seconds int) (int64, error) {
	p, err := store.Row("select * from projects where id=?", parent)
	if err != nil {
		return 0, errors.New("run this inside a project (STUDIO37_PROJECT is not set)")
	}
	span := "the parts of the recording this brief describes"
	if from != "" || to != "" {
		span = fmt.Sprintf("%s to %s of the recording", cmp.Or(from, "the start"), cmp.Or(to, "the end"))
	}
	if seconds <= 0 {
		seconds = int(store.Num(p["seconds"]))
	}
	id, err := store.Save("projects", store.ProjectCols, 0, map[string]any{
		"tenant_id": p["tenant_id"], "campaign_id": p["campaign_id"], "title": title, "kind": p["kind"], "aspect": p["aspect"],
		"seconds": seconds, "quality": p["quality"], "language": p["language"], "focus": p["focus"], "template_id": p["template_id"],
		"brief": brief + fmt.Sprintf(clipRules, store.Str(p["title"]), parent, span),
	})
	if err != nil {
		return 0, err
	}
	store.DB.Exec("update projects set parent_id=?, clip_stage='queued' where id=?", parent, id)
	templates.Use(id, p["template_id"]) // a series cut from a templated source keeps its look
	dir, src := files.ProjectDir(id), files.ProjectDir(parent)
	for _, d := range []string{"input", "work/transcript", "out/drafts"} {
		os.MkdirAll(filepath.Join(dir, d), 0o755)
	}
	// The recording is shared, not copied: a 2-hour class is gigabytes. A hard link survives deleting either project.
	for _, f := range files.List(filepath.Join(src, "input"), "", true) {
		from, to := filepath.Join(src, "input", f.Path), filepath.Join(dir, "input", f.Path)
		os.MkdirAll(filepath.Dir(to), 0o755)
		if os.Link(from, to) != nil {
			os.Symlink(from, to)
		}
	}
	for _, name := range []string{"transcript.json", "transcript.srt", "transcript.txt", "meta.json"} {
		files.Copy(filepath.Join(src, "work", "transcript", name), filepath.Join(dir, "work", "transcript", name))
	}
	return id, nil
}

var clipMu sync.Mutex

func ClipLoop() {
	store.DB.Exec(`update projects set clip_stage='failed' where clip_stage in ('building','rendering')`) // a restart killed them
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
	store.DB.QueryRow("select count(*) from projects where clip_stage in ('building','rendering')").Scan(&busy)
	if busy > 0 || autopilot.Busy() {
		return
	}
	var id int64
	if store.DB.QueryRow("select id from projects where clip_stage='queued' order by id limit 1").Scan(&id) != nil {
		return
	}
	store.DB.Exec("update projects set clip_stage='building' where id=?", id)
	agent.AddEvent(id, "info", map[string]string{"text": "Queued from its source project; the agent is building it now."})
	if err := agent.StartRun(id, "", "", false); err != nil {
		clipFail(id, "could not start the agent: "+err.Error())
	}
}

func clipFail(id int64, msg string) {
	store.DB.Exec("update projects set clip_stage='failed' where id=?", id)
	agent.AddEvent(id, "error", map[string]string{"text": msg})
}

// ClipAdvance moves a queued video on after its agent turn (render it) and after its render (done), then starts the next.
func ClipAdvance(id int64) {
	clipMu.Lock()
	clipStep(id)
	clipMu.Unlock()
	clipTick()
}

func clipStep(id int64) {
	if agent.IsRunning(id) {
		return
	}
	var stage string
	store.DB.QueryRow("select clip_stage from projects where id=?", id).Scan(&stage)
	dir := files.ProjectDir(id)
	switch stage {
	case "building":
		if _, err := os.Stat(filepath.Join(dir, "comp", "index.html")); err != nil {
			clipFail(id, "The agent finished without a video. Read its last message, then ask it to continue.")
			return
		}
		store.DB.Exec("update projects set clip_stage='rendering' where id=?", id)
		if err := agent.StartRender(id, "", "high"); err != nil {
			clipFail(id, "Export failed to start: "+err.Error())
		}
	case "rendering":
		for _, f := range files.List(filepath.Join(dir, "out"), "", false) {
			if f.Kind == "video" {
				store.DB.Exec("update projects set clip_stage='done' where id=?", id)
				return
			}
		}
		clipFail(id, "The export failed. Open the chat to see why, then press Export again.")
	}
}

// ListLibrary is every finished video and image (each project's out/, not drafts) across companies, newest first.
func ListLibrary(r *http.Request) (any, error) {
	ps, err := store.Rows("select p.id, p.title, p.tenant_id, t.name as tenant_name, p.parent_id from projects p join tenants t on t.id=p.tenant_id")
	out := []map[string]any{}
	for _, p := range ps {
		id := p["id"].(int64)
		for _, f := range files.List(filepath.Join(files.ProjectDir(id), "out"), fmt.Sprintf("/files/projects/%d/out", id), false) {
			if f.Kind == "video" || f.Kind == "image" {
				out = append(out, map[string]any{"file": f, "project_id": id, "title": p["title"], "tenant_id": p["tenant_id"], "tenant_name": p["tenant_name"]})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["file"].(files.Info).Mod.After(out[j]["file"].(files.Info).Mod) })
	return out, err
}
