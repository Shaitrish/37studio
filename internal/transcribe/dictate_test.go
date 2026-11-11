package transcribe

import (
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Shaitrish/37studio/internal/httpapi"
)

// A dictation upload streams to disk with its language, and a missing, oversized or oddly labelled one is the
// user's error (400), not the server's.
func TestDictationUpload(t *testing.T) {
	upload := func(lang string, audio io.Reader) (string, []byte, error) {
		pr, pw := io.Pipe()
		mw := multipart.NewWriter(pw)
		go func() {
			if lang != "" {
				mw.WriteField("lang", lang)
			}
			if audio != nil {
				w, _ := mw.CreateFormFile("audio", "clip.webm")
				io.Copy(w, audio)
			}
			pw.CloseWithError(mw.Close())
		}()
		r := httptest.NewRequest("POST", "/api/dictate", pr)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		path := filepath.Join(t.TempDir(), "clip")
		got, err := saveClip(r, path)
		pr.CloseWithError(io.ErrClosedPipe) // let the writer finish when saveClip stopped early
		b, _ := os.ReadFile(path)
		return got, b, err
	}
	bad := func(err error) bool { _, ok := err.(httpapi.Error); return ok }

	if lang, b, err := upload(" BN ", strings.NewReader("opus")); err != nil || lang != "bn" || string(b) != "opus" {
		t.Fatalf("got %q %q %v", lang, b, err)
	}
	if _, _, err := upload("en", nil); !bad(err) {
		t.Fatalf("no audio: %v", err)
	}
	if _, _, err := upload("bangla; rm", strings.NewReader("opus")); !bad(err) {
		t.Fatalf("odd lang: %v", err)
	}
	if _, _, err := upload("", io.LimitReader(zeros{}, maxClip+1)); !bad(err) || !strings.Contains(err.Error(), "too long") {
		t.Fatalf("too long: %v", err)
	}
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) { clear(p); return len(p), nil }

// English takes the fast model; Bangla and detection keep the script's, since small and medium write Bangla in
// Devanagari. The text comes out as one line.
func TestDictationModelAndText(t *testing.T) {
	if a := dictationArgs("en"); !slices.Contains(a, "small") || !slices.Contains(a, "--dictation") {
		t.Fatalf("en: %v", a)
	}
	if a := dictationArgs("bn"); slices.Contains(a, "--model") || !slices.Contains(a, "bn") {
		t.Fatalf("bn: %v", a)
	}
	if a := dictationArgs(""); slices.Contains(a, "--language") || slices.Contains(a, "--model") {
		t.Fatalf("detect: %v", a)
	}
	path := filepath.Join(t.TempDir(), "transcript.json")
	os.WriteFile(path, []byte(`{"segments":[{"text":" আজকের ক্লাসে "},{"text":""},{"text":"Newton's laws."}]}`), 0o644)
	if s, err := transcriptText(path); err != nil || s != "আজকের ক্লাসে Newton's laws." {
		t.Fatalf("text = %q %v", s, err)
	}
}
