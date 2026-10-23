package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Gemini Interactions API: one endpoint for Nano Banana images and Omni video.
const geminiURL = "https://generativelanguage.googleapis.com/v1beta/interactions"

var geminiClient = &http.Client{Timeout: 15 * time.Minute}

func gemini(req map[string]any, kind string) ([]byte, error) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		return nil, errors.New("GEMINI_API_KEY is not set")
	}
	body, _ := json.Marshal(req)
	hr, _ := http.NewRequest("POST", geminiURL, bytes.NewReader(body))
	hr.Header.Set("x-goog-api-key", key)
	hr.Header.Set("Content-Type", "application/json")
	res, err := geminiClient.Do(hr)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("gemini %d: %s", res.StatusCode, tail(string(raw), 500))
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	// The media block lives at steps[].content[] {type, data|uri}; walk instead of hard-coding the path.
	data, uri := "", ""
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if x["type"] == kind {
				if d, ok := x["data"].(string); ok && d != "" {
					data = d
				} else if u, ok := x["uri"].(string); ok && u != "" {
					uri = u
				}
			}
			for _, c := range x {
				walk(c)
			}
		case []any:
			for _, c := range x {
				walk(c)
			}
		}
	}
	root := doc // prefer model_output steps so an echoed reference image is never mistaken for the result
	if m, ok := doc.(map[string]any); ok {
		if steps, ok := m["steps"].([]any); ok {
			var outs []any
			for _, s := range steps {
				if sm, ok := s.(map[string]any); ok && sm["type"] == "model_output" {
					outs = append(outs, sm)
				}
			}
			if len(outs) > 0 {
				root = outs
			}
		}
	}
	walk(root)
	if data != "" {
		return base64.StdEncoding.DecodeString(data)
	}
	if uri != "" {
		return geminiDownload(uri, key)
	}
	return nil, fmt.Errorf("gemini returned no %s: %s", kind, tail(string(raw), 500))
}

func geminiDownload(uri, key string) ([]byte, error) {
	if strings.Contains(uri, "/files/") && !strings.Contains(uri, ":download") {
		uri += ":download?alt=media"
	}
	hr, _ := http.NewRequest("GET", uri, nil)
	hr.Header.Set("x-goog-api-key", key)
	res, err := geminiClient.Do(hr)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("gemini download %d", res.StatusCode)
	}
	return io.ReadAll(res.Body)
}

func imagePart(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return map[string]any{"type": "image", "mime_type": http.DetectContentType(b), "data": base64.StdEncoding.EncodeToString(b)}, nil
}

// recordUsage books a generation's cost on the project and remembers its file, so the UI can list what was generated.
func recordUsage(provider string, pid, tenant int64, item string, qty, cost float64, file string) {
	if pid <= 0 {
		return
	}
	if rel, err := filepath.Rel(projectDir(pid), file); err == nil && !strings.HasPrefix(rel, "..") {
		file = filepath.ToSlash(rel)
	}
	db.Exec("insert into usage(project_id,tenant_id,provider,item,quantity,cost,file) values(?,?,?,?,?,?,?)", pid, tenant, provider, item, qty, cost, file)
}

func genImage(pid, tenant int64, prompt, aspect, size string, refs []string, out string) error {
	input := []map[string]any{{"type": "text", "text": prompt}}
	for _, ref := range refs {
		part, err := imagePart(ref)
		if err != nil {
			return err
		}
		input = append(input, part)
	}
	format := map[string]any{"type": "image", "mime_type": "image/png"}
	if ext := strings.ToLower(filepath.Ext(out)); ext == ".jpg" || ext == ".jpeg" {
		format["mime_type"] = "image/jpeg"
	}
	if aspect != "" {
		format["aspect_ratio"] = aspect
	}
	if size != "" {
		format["image_size"] = size
	}
	model := setting("image_model")
	img, err := gemini(map[string]any{"model": model, "input": input, "response_format": format}, "image")
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, img, 0o644); err != nil {
		return err
	}
	recordUsage("gemini", pid, tenant, model, 1, settingF("image_price"), out)
	return nil
}

func genVideo(pid, tenant int64, prompt, aspect, firstFrame, out string) error {
	input := []map[string]any{{"type": "text", "text": prompt}}
	if firstFrame != "" {
		part, err := imagePart(firstFrame)
		if err != nil {
			return err
		}
		input = append([]map[string]any{part}, input...)
	}
	format := map[string]any{"type": "video"}
	if aspect == "9:16" || aspect == "16:9" { // the only ratios Omni supports
		format["aspect_ratio"] = aspect
	}
	model := setting("video_model")
	vid, err := gemini(map[string]any{"model": model, "input": input, "response_format": format}, "video")
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, vid, 0o644); err != nil {
		return err
	}
	sec := mediaSeconds(out)
	recordUsage("gemini", pid, tenant, model, sec, sec*settingF("video_price_per_sec"), out)
	return nil
}

type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

// genCLI is `37studio gen image|video ...`, the tool the agent calls. Cost lands on $STUDIO37_PROJECT.
func genCLI(args []string) int {
	if len(args) == 0 || !contains([]string{"image", "video", "voice", "sfx", "music"}, args[0]) {
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
	var refs multiFlag
	fs.Var(&refs, "ref", "image only: reference image (repeatable)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if *prompt == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "--prompt and --out are required")
		return 2
	}
	dataDir, _ = filepath.Abs(dataDir)
	openDB()
	pid, _ := strconv.ParseInt(env("STUDIO37_PROJECT", ""), 10, 64)
	p, _ := row("select * from projects where id=?", pid)
	var tenant int64
	if p != nil {
		tenant = p["tenant_id"].(int64)
	}
	if *voice == "" {
		*voice = voiceFor(projectLanguage(p))
	}
	*out, _ = filepath.Abs(*out)
	os.MkdirAll(filepath.Dir(*out), 0o755)
	err := genAny(args[0], pid, tenant, genOpts{prompt: *prompt, aspect: *aspect, size: *size, image: *image,
		refs: refs, seconds: *seconds, voice: *voice}, *out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Println(*out)
	return 0
}
