package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
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
		err = fmt.Errorf("elevenlabs %d: %s", res.StatusCode, tail(string(raw), 400))
	}
	return raw, err
}

// voiceFor picks the configured voice for a language: Bangla gets its own, everything else the default.
func voiceFor(lang string) string {
	if lang == "bn" && setting("voice_bn") != "" {
		return setting("voice_bn")
	}
	return setting("voice_default")
}

func genVoice(pid, tenant int64, text, voice, out string) error {
	if voice == "" {
		return errors.New("no voice configured: pick one in Settings → Voice")
	}
	audio, err := eleven("POST", "/v1/text-to-speech/"+voice+"?output_format=mp3_44100_128",
		map[string]any{"text": text, "model_id": setting("voice_model")})
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, audio, 0o644); err != nil {
		return err
	}
	chars := float64(len([]rune(text)))
	recordUsage("elevenlabs", pid, tenant, "voice "+setting("voice_model"), chars, chars/1000*settingF("voice_price_per_1k"), out)
	return nil
}

func genSFX(pid, tenant int64, prompt string, seconds float64, out string) error {
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
	sec := mediaSeconds(out)
	recordUsage("elevenlabs", pid, tenant, "sound effect", sec, sec*settingF("sfx_price_per_sec"), out)
	return nil
}

func genMusic(pid, tenant int64, prompt string, seconds float64, out string) error {
	ms := int(max(seconds, 3) * 1000) // the API's minimum is 3 s
	audio, err := eleven("POST", "/v1/music", map[string]any{"prompt": prompt, "music_length_ms": ms})
	if err == nil {
		err = os.WriteFile(out, audio, 0o644)
	}
	if err != nil {
		return err
	}
	sec := mediaSeconds(out)
	recordUsage("elevenlabs", pid, tenant, "music", sec, sec/60*settingF("music_price_per_min"), out)
	return nil
}

// listVoices feeds the voice pickers in Settings.
func listVoices(r *http.Request) (any, error) {
	raw, err := eleven("GET", "/v2/voices?page_size=100", nil)
	if err != nil {
		return nil, bad(err.Error())
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
