package gen

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Shaitrish/37studio/internal/costs"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/remote"
	"github.com/Shaitrish/37studio/internal/store"
)

// fal.ai runs many video models (Seedance, Kling, Veo, Wan, Hailuo) behind one key and one queue API. A video model
// setting with a slash, like bytedance/seedance-2.0/fast/text-to-video, is a fal endpoint; Gemini's have none.
var falQueue = "https://queue.fal.run"

var falPoll = 3 * time.Second

// IsFal reports whether a model setting names a fal endpoint.
func IsFal(model string) bool { return strings.Contains(model, "/") }

func fal(endpoint string, input map[string]any) (map[string]any, error) {
	key := os.Getenv("FAL_KEY")
	if key == "" {
		return nil, errors.New("no fal.ai key: add one in Settings → AI services, or pick a Gemini video model")
	}
	auth := "Key " + key
	var sub struct {
		StatusURL   string `json:"status_url"`
		ResponseURL string `json:"response_url"`
	}
	if err := remote.Call(remote.JSON("POST", falQueue+"/"+endpoint, input, auth), &sub); err != nil {
		return nil, err
	}
	for start := time.Now(); ; time.Sleep(falPoll) { // video models take one to several minutes
		var st struct{ Status string }
		if err := remote.Call(remote.Get(sub.StatusURL, auth), &st); err != nil {
			return nil, err
		}
		if st.Status == "COMPLETED" {
			break
		}
		if time.Since(start) > 20*time.Minute {
			return nil, errors.New("fal.ai didn't finish within 20 minutes")
		}
	}
	var res map[string]any
	return res, remote.Call(remote.Get(sub.ResponseURL, auth), &res) // a failed generation answers here with its error
}

// falFileURL finds the generated file in a fal result: {"video": {"url"}}, {"audio": …} or {"images": [{"url"}]}.
func falFileURL(res map[string]any) string {
	for _, k := range []string{"video", "audio", "image"} {
		if f, ok := res[k].(map[string]any); ok && f["url"] != nil {
			return fmt.Sprint(f["url"])
		}
	}
	if list, ok := res["images"].([]any); ok && len(list) > 0 {
		if f, ok := list[0].(map[string]any); ok {
			return fmt.Sprint(f["url"])
		}
	}
	return ""
}

func download(url, out string) error {
	res, err := (&http.Client{Timeout: 10 * time.Minute}).Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("downloading the result: %s", res.Status)
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	if _, err = io.Copy(f, res.Body); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// falVideo makes a clip with a fal model. With a first frame it uses the model's image-to-video endpoint, which fal
// names after the text one; the image goes inline as a data URI, so nothing is uploaded anywhere else first.
func falVideo(pid, tenant int64, model, prompt, aspect, firstFrame, out string) error {
	endpoint, input := model, map[string]any{"prompt": prompt}
	if aspect == "16:9" || aspect == "9:16" || aspect == "1:1" { // the ratios every model takes; others use the model's default
		input["aspect_ratio"] = aspect
	}
	if firstFrame != "" {
		b, err := os.ReadFile(firstFrame)
		if err != nil {
			return err
		}
		if endpoint = strings.Replace(model, "text-to-video", "image-to-video", 1); endpoint == model {
			endpoint = model + "/image-to-video" // fal-ai/veo3.1 → fal-ai/veo3.1/image-to-video
		}
		input["image_url"] = "data:" + http.DetectContentType(b) + ";base64," + base64.StdEncoding.EncodeToString(b)
	}
	res, err := fal(endpoint, input)
	if err != nil {
		return err
	}
	url := falFileURL(res)
	if url == "" {
		return errors.New("fal.ai returned no video")
	}
	if err := download(url, out); err != nil {
		return err
	}
	sec := files.MediaSeconds(out)
	costs.Record("fal", pid, tenant, model, sec, sec*store.SettingF("video_price_per_sec"), out)
	return nil
}
