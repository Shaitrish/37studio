package gen

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/store"
)

// A brand voice is cloned once from a recording and then reused by name; the brand's default for a language beats its
// any-language default, which beats Settings; one default per language; deleting a clone deletes it at ElevenLabs.
func TestBrandVoices(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("needs ffmpeg")
	}
	config.DataDir = t.TempDir()
	store.Open()
	store.DB.Exec("insert into tenants(id, name) values(1, 'Course37')")
	t.Setenv("ELEVENLABS_API_KEY", "k")
	const clone = "C1onedVoiceAAAAAAAAA"
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		if r.Header.Get("xi-api-key") != "k" {
			t.Errorf("key %q", r.Header.Get("xi-api-key"))
		}
		switch {
		case r.URL.Path == "/v1/voices/add":
			if r.ParseMultipartForm(1<<20) != nil || r.FormValue("name") != "Rahim" || len(r.MultipartForm.File["files"]) != 1 {
				t.Errorf("clone form = %v", r.MultipartForm)
			}
			w.Write([]byte(`{"voice_id":"` + clone + `","requires_verification":false}`))
		case r.Method == "GET":
			w.Write([]byte(`{"name":"Alice"}`))
		case strings.HasPrefix(r.URL.Path, "/v1/voice-changer/"+clone):
			if f, _, err := r.FormFile("audio"); err != nil {
				t.Errorf("revoice without audio: %v", err)
			} else {
				f.Close()
			}
			w.Write([]byte("MP3"))
		}
	}))
	defer srv.Close()
	defer func(u string) { elevenURL = u }(elevenURL)
	elevenURL = srv.URL

	// Clone from a one-second silent WAV.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("file", "rahim.wav")
	part.Write(wav())
	mw.Close()
	r := httptest.NewRequest("POST", "/api/tenants/1/voices/clone?name=Rahim&language=bn&default=1", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.SetPathValue("id", "1")
	if _, err := CloneBrandVoice(r); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(voiceDir(1, clone), "sample-1.mp3")); err != nil {
		t.Fatalf("sample not kept: %v", err)
	}

	add := func(b map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(b)
		r := httptest.NewRequest("POST", "/api/tenants/1/voices", bytes.NewReader(raw))
		r.SetPathValue("id", "1")
		if _, err := AddBrandVoice(r); err != nil {
			t.Fatal(err)
		}
	}
	add(map[string]any{"voice_id": "Xb7hH8MSUJpSbSDYk0k2", "is_default": true}) // any language, named after ElevenLabs
	if v := VoiceFor(1, "bn"); v != clone {
		t.Errorf("Bangla voice = %s, want the Bangla default", v)
	}
	if v := VoiceFor(1, "en"); v != "Xb7hH8MSUJpSbSDYk0k2" {
		t.Errorf("English voice = %s, want the any-language default", v)
	}
	if v := VoiceFor(2, "bn"); v != store.Setting("voice_bn") {
		t.Errorf("another brand's voice = %s, want Settings", v)
	}
	if v, err := ResolveVoice(1, "en", "rahim"); err != nil || v != clone {
		t.Errorf("by name = %s, %v", v, err)
	}
	if _, err := ResolveVoice(1, "en", "Karim"); err == nil || !strings.Contains(err.Error(), `"Alice"`) {
		t.Errorf("unknown name = %v, want the brand's voices listed", err)
	}

	add(map[string]any{"voice_id": "OtherBanglaVoiceAAAA", "name": "Nadia", "language": "bn", "is_default": true})
	if v := VoiceFor(1, "bn"); v != "OtherBanglaVoiceAAAA" {
		t.Errorf("Bangla voice = %s, want the newest default", v)
	}
	var defaults int
	store.DB.QueryRow("select count(*) from brand_voices where tenant_id=1 and language='bn' and is_default=1").Scan(&defaults)
	if defaults != 1 {
		t.Errorf("%d Bangla defaults, want 1", defaults)
	}

	vo := filepath.Join(t.TempDir(), "vo.mp3")
	os.WriteFile(vo, []byte("old"), 0o644)
	if err := Revoice(1, 1, vo, clone, vo); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(vo); string(b) != "MP3" {
		t.Errorf("revoiced file = %q", b)
	}

	r = httptest.NewRequest("DELETE", "/api/brand-voices/1", nil)
	r.SetPathValue("id", "1")
	if _, err := DeleteBrandVoice(r); err != nil {
		t.Fatal(err)
	}
	if calls[len(calls)-1] != "DELETE /v1/voices/"+clone {
		t.Errorf("last call = %s, want the clone deleted at ElevenLabs", calls[len(calls)-1])
	}
	if _, err := os.Stat(voiceDir(1, clone)); !os.IsNotExist(err) {
		t.Errorf("sample folder still there: %v", err)
	}
}

// wav is one second of 16 kHz mono silence.
func wav() []byte {
	var b bytes.Buffer
	data := make([]byte, 32000)
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+len(data)))
	b.WriteString("WAVEfmt ")
	for _, v := range []any{uint32(16), uint16(1), uint16(1), uint32(16000), uint32(32000), uint16(2), uint16(16)} {
		binary.Write(&b, binary.LittleEndian, v)
	}
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(len(data)))
	b.Write(data)
	return b.Bytes()
}
