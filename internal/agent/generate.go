package agent

import (
	"cmp"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/gen"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/store"
)

type genOpts struct {
	prompt, aspect, size, image, voice string
	refs                               []string
	seconds                            float64
}

// genAny routes one generation to its provider; the cost lands on the project either way.
func genAny(kind string, pid, tenant int64, o genOpts, out string) error {
	switch kind {
	case "image":
		return gen.Image(pid, tenant, o.prompt, o.aspect, o.size, o.refs, out)
	case "video":
		return gen.Video(pid, tenant, o.prompt, o.aspect, o.image, out)
	case "voice":
		return gen.Voice(pid, tenant, o.prompt, o.voice, out)
	case "sfx":
		return gen.SFX(pid, tenant, o.prompt, o.seconds, out)
	case "music":
		if o.seconds == 0 {
			o.seconds = 30
		}
		return gen.Music(pid, tenant, o.prompt, o.seconds, out)
	}
	return fmt.Errorf("unknown kind %q", kind)
}

// ProjectLanguage is the video's language: set per project, else the brand's.
func ProjectLanguage(p map[string]any) string {
	if l := store.Str(p["language"]); l != "" {
		return l
	}
	var l string
	store.DB.QueryRow("select language from tenants where id=?", p["tenant_id"]).Scan(&l)
	return cmp.Or(l, "en")
}

func GenerateMedia(r *http.Request) (any, error) {
	b, err := httpapi.Body(r)
	if err != nil {
		return nil, err
	}
	id := httpapi.PathID(r)
	p, err := store.Row("select * from projects where id=?", id)
	if err != nil {
		return nil, err
	}
	kind, prompt := store.Str(b["kind"]), strings.TrimSpace(store.Str(b["prompt"]))
	ext, ok := map[string]string{"image": ".png", "video": ".mp4", "voice": ".mp3", "sfx": ".mp3", "music": ".mp3"}[kind]
	if prompt == "" || !ok {
		return nil, httpapi.Bad("kind (image|video|voice|sfx|music) and prompt are required")
	}
	out := filepath.Join(files.ProjectDir(id), "out", "drafts", fmt.Sprintf("gen-%s-%s%s", kind, uuid.NewString()[:8], ext))
	aspect := cmp.Or(store.Str(b["aspect"]), store.Str(p["aspect"]))
	AddEvent(id, "info", map[string]string{"text": fmt.Sprintf("Generating %s: %s", kind, prompt)})
	go func() {
		err := genAny(kind, id, p["tenant_id"].(int64), genOpts{prompt: prompt, aspect: aspect, size: "2K",
			seconds: store.Num(b["seconds"]), voice: gen.VoiceFor(ProjectLanguage(p))}, out)
		if err != nil {
			AddEvent(id, "error", map[string]string{"text": "Generation failed: " + err.Error()})
			return
		}
		rel, _ := filepath.Rel(files.ProjectDir(id), out)
		AddEvent(id, "render", map[string]string{"text": "Generated " + filepath.ToSlash(rel), "file": filepath.ToSlash(rel)})
	}()
	return map[string]bool{"ok": true}, nil
}

// mediaEstimate is roughly what one generation will cost, in USD, from the unit prices in Settings.
func mediaEstimate(kind string, o genOpts) float64 {
	switch kind {
	case "image":
		return store.SettingF("image_price")
	case "video":
		return 8 * store.SettingF("video_price_per_sec") // ponytail: clips run about 5-10 s; the real length is billed after
	case "voice":
		return float64(len(o.prompt)) / 1000 * store.SettingF("voice_price_per_1k")
	case "sfx":
		return max(o.seconds, 2) * store.SettingF("sfx_price_per_sec")
	case "music":
		return max(o.seconds, 30) / 60 * store.SettingF("music_price_per_min")
	}
	return 0
}

// overMediaLimit says whether this generation would take the project's generated media past the amount the agent may
// spend without asking (Settings → Spending); "" when it may go ahead.
func overMediaLimit(pid int64, kind string, o genOpts) string {
	limit := store.SettingF("gen_ask_above_usd")
	var spent float64
	store.DB.QueryRow("select coalesce(sum(cost),0) from usage where project_id=? and provider in ('gemini','elevenlabs','fal')", pid).Scan(&spent)
	if est := mediaEstimate(kind, o); spent+est > limit {
		return fmt.Sprintf("This %s would cost about $%.2f, bringing generated media on this project to about $%.2f, over the $%.2f "+
			"the editor allows without asking (Settings → Spending). Ask the editor first, saying what you want to make and roughly what it costs; "+
			"if they agree, run the same command again with --approved.", kind, est, spent+est, limit)
	}
	return ""
}

type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }

func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

// GenCLI is `37studio gen image|video ...`, the tool the agent calls. Cost lands on $STUDIO37_PROJECT.
func GenCLI(args []string) int {
	if len(args) == 0 || !slices.Contains([]string{"image", "video", "voice", "sfx", "music"}, args[0]) {
		fmt.Fprintln(os.Stderr, "usage: 37studio gen image|video|voice|sfx|music --prompt TEXT --out FILE [--aspect 9:16] [--size 1K|2K|4K] [--ref IMG ...] [--image FIRST_FRAME] [--seconds N] [--voice VOICE_ID]")
		return 2
	}
	fs := flag.NewFlagSet("gen "+args[0], flag.ContinueOnError)
	prompt := fs.String("prompt", "", "what to generate")
	out := fs.String("out", "", "output file")
	aspect := fs.String("aspect", "", "aspect ratio, e.g. 16:9 or 9:16")
	size := fs.String("size", "2K", "image size: 1K, 2K or 4K")
	image := fs.String("image", "", "video only: first-frame image")
	seconds := fs.Float64("seconds", 0, "sfx and music: length in seconds")
	voice := fs.String("voice", "", "voice only: ElevenLabs voice id (default: the configured voice for the video's language)")
	approved := fs.Bool("approved", false, "the editor agreed to go over the spending limit for generated media")
	var refs multiFlag
	fs.Var(&refs, "ref", "image only: reference image (repeatable)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if *prompt == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "--prompt and --out are required")
		return 2
	}
	config.DataDir, _ = filepath.Abs(config.DataDir)
	store.Open()
	pid, _ := strconv.ParseInt(config.Env("STUDIO37_PROJECT", ""), 10, 64)
	p, _ := store.Row("select * from projects where id=?", pid)
	var tenant int64
	if p != nil {
		tenant = p["tenant_id"].(int64)
	}
	if *voice == "" {
		*voice = gen.VoiceFor(ProjectLanguage(p))
	}
	*out, _ = filepath.Abs(*out)
	os.MkdirAll(filepath.Dir(*out), 0o755)
	o := genOpts{prompt: *prompt, aspect: *aspect, size: *size, image: *image, refs: refs, seconds: *seconds, voice: *voice}
	if msg := overMediaLimit(pid, args[0], o); msg != "" && !*approved {
		fmt.Fprintln(os.Stderr, msg)
		return 3
	}
	err := genAny(args[0], pid, tenant, o, *out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Println(*out)
	return 0
}
