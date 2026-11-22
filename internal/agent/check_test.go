package agent

import (
	"strings"
	"testing"
)

func TestCheckIssues(t *testing.T) {
	clean := `Stream #0:1(und): Audio: aac
[Parsed_ebur128_0 @ 0x5] t: 0.1  TARGET:-23 LUFS  M: -70.0 S: -70.0  I: -70.0 LUFS  LRA: 0.0 LU  TPK: -70.0 -70.0 dBFS
  Integrated loudness:
    I:         -14.2 LUFS
  True peak:
    Peak:       -1.6 dBFS`
	if got := checkIssues(clean); len(got) != 0 {
		t.Fatalf("clean render flagged: %v", got)
	}

	bad := `Stream #0:1(und): Audio: aac
[freezedetect @ 0x1] lavfi.freezedetect.freeze_start: 12.5
[freezedetect @ 0x1] lavfi.freezedetect.freeze_duration: 3.25
[freezedetect @ 0x1] lavfi.freezedetect.freeze_end: 15.75
[blackdetect @ 0x2] black_start:0 black_end:0.8 black_duration:0.8
[silencedetect @ 0x3] silence_start: 61.2
[silencedetect @ 0x3] silence_end: 64 | silence_duration: 2.8
  Integrated loudness:
    I:         -24.0 LUFS
  True peak:
    Peak:       -0.2 dBFS`
	got := strings.Join(checkIssues(bad), "\n")
	for _, want := range []string{"Too quiet: -24.0", "Peaks reach -0.2", "Silence for 2.8s from 1:01.2", "doesn't move for 3.2s from 0:12.5", "Opens on 0.8s of black"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}

	if got := checkIssues("Stream #0:0: Video: h264"); len(got) != 1 || !strings.Contains(got[0], "No sound") {
		t.Fatalf("silent video: %v", got)
	}
}
