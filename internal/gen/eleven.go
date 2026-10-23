// Package gen calls the media generators: Gemini for images and video, ElevenLabs for voice, sound effects and music.
package gen

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/Shaitrish/37studio/internal/costs"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/textutil"
)

// ElevenLabs: voiceover (TTS), sound effects and music. All three return MP3 bytes.
const elevenURL = "https://api.elevenlabs.io"

func eleven(method, path string, body any) ([]byte, error) {
	key := os.Getenv("ELEVENLABS_API_KEY")
	if key == "" {
		return nil, errors.New("ELEVENLABS_API_KEY is not set")
	}
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, elevenURL+path, rd)
	req.Header.Set("xi-api-key", key)
	req.Header.Set("Content-Type", "application/json")
	res, err := geminiClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err == nil && res.StatusCode >= 300 {
		err = fmt.Errorf("elevenlabs %d: %s", res.StatusCode, textutil.Tail(string(raw), 400))
	}
	return raw, err
}

// VoiceFor picks the configured voice for a language: Bangla gets its own, everything else the default.
func VoiceFor(lang string) string {
	if lang == "bn" && store.Setting("voice_bn") != "" {
		return store.Setting("voice_bn")
	}
	return store.Setting("voice_default")
}

func Voice(pid, tenant int64, text, voice, out string) error {
	if voice == "" {
		return errors.New("no voice configured: pick one in Settings → Voice")
	}
	audio, err := eleven("POST", "/v1/text-to-speech/"+voice+"?output_format=mp3_44100_128",
		map[string]any{"text": text, "model_id": store.Setting("voice_model")})
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, audio, 0o644); err != nil {
		return err
	}
	chars := float64(len([]rune(text)))
	costs.Record("elevenlabs", pid, tenant, "voice "+store.Setting("voice_model"), chars, chars/1000*store.SettingF("voice_price_per_1k"), out)
	return nil
}

func SFX(pid, tenant int64, prompt string, seconds float64, out string) error {
	body := map[string]any{"text": prompt}
	if seconds > 0 {
		body["duration_seconds"] = seconds
	}
	audio, err := eleven("POST", "/v1/sound-generation", body)
	if err == nil {
		err = os.WriteFile(out, audio, 0o644)
	}
	if err != nil {
		return err
	}
	sec := files.MediaSeconds(out)
	costs.Record("elevenlabs", pid, tenant, "sound effect", sec, sec*store.SettingF("sfx_price_per_sec"), out)
	return nil
}

func Music(pid, tenant int64, prompt string, seconds float64, out string) error {
	ms := int(max(seconds, 3) * 1000) // the API's minimum is 3 s
	audio, err := eleven("POST", "/v1/music", map[string]any{"prompt": prompt, "music_length_ms": ms})
	if err == nil {
		err = os.WriteFile(out, audio, 0o644)
	}
	if err != nil {
		return err
	}
	sec := files.MediaSeconds(out)
	costs.Record("elevenlabs", pid, tenant, "music", sec, sec/60*store.SettingF("music_price_per_min"), out)
	return nil
}

// ListVoices feeds the voice pickers in Settings.
func ListVoices(r *http.Request) (any, error) {
	raw, err := eleven("GET", "/v2/voices?page_size=100", nil)
	if err != nil {
		return nil, httpapi.Bad(err.Error())
	}
	var res struct {
		Voices []struct {
			ID      string            `json:"voice_id"`
			Name    string            `json:"name"`
			Labels  map[string]string `json:"labels"`
			Preview string            `json:"preview_url"`
		} `json:"voices"`
	}
	json.Unmarshal(raw, &res)
	out := []map[string]any{}
	for _, v := range res.Voices {
		out = append(out, map[string]any{"id": v.ID, "name": v.Name, "language": v.Labels["language"],
			"gender": v.Labels["gender"], "use": v.Labels["use_case"], "preview": v.Preview})
	}
	return out, nil
}
