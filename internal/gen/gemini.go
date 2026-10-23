package gen

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Shaitrish/37studio/internal/costs"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/textutil"
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
		return nil, fmt.Errorf("gemini %d: %s", res.StatusCode, textutil.Tail(string(raw), 500))
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
	return nil, fmt.Errorf("gemini returned no %s: %s", kind, textutil.Tail(string(raw), 500))
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

func Image(pid, tenant int64, prompt, aspect, size string, refs []string, out string) error {
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
	model := store.Setting("image_model")
	img, err := gemini(map[string]any{"model": model, "input": input, "response_format": format}, "image")
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, img, 0o644); err != nil {
		return err
	}
	costs.Record("gemini", pid, tenant, model, 1, store.SettingF("image_price"), out)
	return nil
}

func Video(pid, tenant int64, prompt, aspect, firstFrame, out string) error {
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
	model := store.Setting("video_model")
	vid, err := gemini(map[string]any{"model": model, "input": input, "response_format": format}, "video")
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, vid, 0o644); err != nil {
		return err
	}
	sec := files.MediaSeconds(out)
	costs.Record("gemini", pid, tenant, model, sec, sec*store.SettingF("video_price_per_sec"), out)
	return nil
}
