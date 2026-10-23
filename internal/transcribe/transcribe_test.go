package transcribe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Subtitles stand in for a transcription: SRT and YouTube's rolling WebVTT both parse, the rolled-over lines don't
// repeat, and the editor's "skip" stops the command until --force.
func TestSubtitlesAndSkip(t *testing.T) {
	srt := "1\r\n00:00:01,000 --> 00:00:03,500\r\nআজকের ক্লাসে\r\n\r\n2\r\n00:00:03,500 --> 00:01:02,250\r\n<i>Newton's</i> laws &amp; more\r\n"
	if got := parseSubs(srt); len(got) != 2 || got[0].Text != "আজকের ক্লাসে" || got[1].End != 62.25 || got[1].Text != "Newton's laws & more" {
		t.Fatalf("srt: %+v", got)
	}
	vtt := "WEBVTT\nKind: captions\n\n00:00.000 --> 00:02.000 align:start\nhello<00:00:01.000><c> world</c>\n\n00:02.000 --> 00:02.010\nhello world\n\n00:02.010 --> 00:04.000\nhello world\nhow are you\n"
	if got := parseSubs(vtt); len(got) != 2 || got[0].Text != "hello world" || got[1].Text != "how are you" {
		t.Fatalf("vtt: %+v", got)
	}

	dir := t.TempDir()
	in, out := filepath.Join(dir, "input"), filepath.Join(dir, "work", "transcript")
	os.MkdirAll(in, 0o755)
	os.WriteFile(filepath.Join(in, "Class 7.mp4"), []byte("v"), 0o644)
	os.WriteFile(filepath.Join(in, "Class 7.en.srt"), []byte(srt), 0o644)
	os.WriteFile(filepath.Join(in, "Class 7.bn.srt"), []byte(srt), 0o644)
	if got := subtitlesFor(filepath.Join(in, "Class 7.mp4"), "bn"); filepath.Base(got) != "Class 7.bn.srt" {
		t.Fatalf("subtitlesFor picked %q", got)
	}
	if code := CLI([]string{filepath.Join(in, "Class 7.mp4"), "--language", "bn", "--out", out}); code != 0 {
		t.Fatalf("from subtitles: exit %d", code)
	}
	if b, _ := os.ReadFile(filepath.Join(out, "transcript.txt")); !strings.Contains(string(b), "[0:00:01] আজকের ক্লাসে") {
		t.Fatalf("transcript.txt = %q", b)
	}

	os.WriteFile(filepath.Join(out, "skip"), nil, 0o644)
	os.Remove(filepath.Join(in, "Class 7.bn.srt"))
	os.Remove(filepath.Join(in, "Class 7.en.srt"))
	if code := CLI([]string{filepath.Join(in, "Class 7.mp4"), "--out", out}); code != 0 {
		t.Fatalf("skipped: exit %d, want 0 and the note", code)
	}
	if _, err := os.Stat(filepath.Join(out, "skip")); err != nil {
		t.Fatal("skip must stay until --force")
	}
}
