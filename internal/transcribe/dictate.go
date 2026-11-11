package transcribe

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/proc"
)

// maxClip caps a dictation recording: a minute of Opus is under 1 MB, so this only stops mistakes.
const maxClip = 25 << 20

var langCode = regexp.MustCompile(`^[a-z]{2,3}$`)

// dictating keeps to one dictation at a time: each run loads a speech model (large-v3-turbo is 1.6 GB) into memory.
//
// ponytail: every clip loads the model again (about 5 s of a turbo run); a resident worker that keeps it loaded for a
// few idle minutes would make dictation feel instant, worth it once people dictate a lot.
var dictating sync.Mutex

// Dictate is POST /api/dictate: a voice recording from the chat's mic button (multipart field "audio", optional "lang"
// such as bn or en) comes back as {"text": …}, transcribed on this computer with the same script as `37studio
// transcribe`. It's for the desktop app's window, whose WebKitGTK or WebView2 has no working speech recognition.
func Dictate(r *http.Request) (any, error) {
	dir, err := os.MkdirTemp("", "37studio-dictate-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	clip := filepath.Join(dir, "clip")
	lang, err := saveClip(r, clip)
	if err != nil {
		return nil, err
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, httpapi.Bad("Dictation needs ffmpeg to read the recording, and it isn't installed. Install ffmpeg, then try again.")
	}

	dictating.Lock()
	defer dictating.Unlock()
	py, err := Python() // the first time, this sets up the venv (about 200 MB)
	if err != nil {
		return nil, httpapi.Bad("Dictation turns speech into text on this computer with Python 3 and faster-whisper, and that isn't set up: " + err.Error() + ". Install Python 3 (on Linux, python3 and python3-venv), then try again.")
	}
	script := filepath.Join(dir, "37studio-transcribe.py")
	if err := os.WriteFile(script, asrScript, 0o644); err != nil {
		return nil, err
	}
	out := filepath.Join(dir, "out")
	cmd := proc.CommandContext(r.Context(), py, append([]string{script, clip, "--out", out}, dictationArgs(lang)...)...)
	cmd.Cancel = func() error { return proc.Kill(cmd) } // the page gave up: stop Python and its ffmpeg too
	var log strings.Builder
	cmd.Stderr = &log
	if err := proc.Start(cmd); err != nil {
		return nil, err
	}
	err = cmd.Wait()
	proc.Done(cmd)
	if err != nil {
		return nil, errors.New("Couldn't turn the recording into text: " + strings.TrimSpace(lastLines(log.String(), 1)))
	}
	text, err := transcriptText(filepath.Join(out, "transcript.json"))
	return map[string]string{"text": text}, err
}

// saveClip streams the "audio" part to path and returns the "lang" field.
func saveClip(r *http.Request, path string) (lang string, err error) {
	r.Body = http.MaxBytesReader(nil, r.Body, maxClip)
	mr, err := r.MultipartReader()
	if err != nil {
		return "", httpapi.Bad(`Expected a multipart upload with the recording in the field "audio".`)
	}
	var size int64
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", tooLong(err)
		}
		switch part.FormName() {
		case "lang":
			b, _ := io.ReadAll(io.LimitReader(part, 16))
			lang = strings.ToLower(strings.TrimSpace(string(b)))
		case "audio":
			f, err := os.Create(path)
			if err != nil {
				return "", err
			}
			size, err = io.Copy(f, part)
			f.Close()
			if err != nil {
				return "", tooLong(err)
			}
		}
	}
	if size == 0 {
		return "", httpapi.Bad(`The recording is empty: no sound reached the app. Check that the microphone works and isn't muted, then try again.`)
	}
	if lang != "" && !langCode.MatchString(lang) {
		return "", httpapi.Bad(`"lang" should be a language code such as bn or en, or left out to detect it.`)
	}
	return lang, nil
}

func tooLong(err error) error {
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return httpapi.Bad("The recording is too long for dictation (25 MB at most). Keep it to a minute or two.")
	}
	return err
}

// dictationArgs picks the speed/quality trade for a short clip. small is several times faster than large-v3-turbo
// and fine for English, but writes Bangla in Devanagari (medium too), so every other language, and detection, keep
// the script's own choice.
func dictationArgs(lang string) []string {
	args := []string{"--dictation"}
	if lang != "" {
		args = append(args, "--language", lang)
	}
	if lang == "en" {
		args = append(args, "--model", "small")
	}
	return args
}

// transcriptText is the plain text of transcript.json, its segments joined by spaces.
func transcriptText(path string) (string, error) {
	var t struct {
		Segments []struct {
			Text string `json:"text"`
		} `json:"segments"`
	}
	if err := json.Unmarshal([]byte(files.ReadFile(path)), &t); err != nil {
		return "", errors.New("Couldn't read the transcription of the recording: " + err.Error())
	}
	var parts []string
	for _, s := range t.Segments {
		if s := strings.TrimSpace(s.Text); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " "), nil
}
