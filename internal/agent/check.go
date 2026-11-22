package agent

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/Shaitrish/37studio/internal/proc"
)

// CheckCLI is `37studio check FILE`: one ffmpeg pass over a rendered video for what a frame snapshot can't show:
// loudness, clipping, dead air, frozen picture and black frames. hyperframes check covers layout and contrast before render.
func CheckCLI(args []string) int {
	if len(args) != 1 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(os.Stderr, "usage: 37studio check FILE.mp4")
		return 2
	}
	out, err := proc.Command("ffmpeg", "-hide_banner", "-nostats", "-i", args[0],
		"-vf", "freezedetect=n=0.001:d=3,blackdetect=d=0.5:pix_th=0.10",
		"-af", "ebur128=peak=true,silencedetect=n=-50dB:d=1.5", "-f", "null", "-").CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "check: ffmpeg could not read %s: %s\n", args[0], lastLine(string(out)))
		return 1
	}
	issues := checkIssues(string(out))
	if len(issues) == 0 {
		fmt.Println("OK: loudness, peaks, sound and picture all look right.")
		return 0
	}
	for _, s := range issues {
		fmt.Println("- " + s)
	}
	fmt.Println("Fix these, or tell the editor why one is on purpose (a held title, a pause for effect).")
	return 0
}

var (
	reLUFS    = regexp.MustCompile(`I:\s+(-?[\d.]+) LUFS`)
	rePeak    = regexp.MustCompile(`Peak:\s+(-?[\d.]+|-inf) dBFS`)
	reFreeze  = regexp.MustCompile(`freeze_start: ([\d.]+)[\s\S]*?freeze_duration: ([\d.]+)`)
	reBlack   = regexp.MustCompile(`black_start:([\d.]+) black_end:[\d.]+ black_duration:([\d.]+)`)
	reSilence = regexp.MustCompile(`silence_start: (-?[\d.]+)[\s\S]*?silence_duration: ([\d.]+)`)
)

// checkIssues reads ffmpeg's filter log. Social platforms normalise to about -14 LUFS, so much quieter sounds weak
// next to other posts and louder gets turned down with its peaks squashed.
func checkIssues(log string) []string {
	var out []string
	if !strings.Contains(log, "Audio:") {
		out = append(out, "No sound track at all. Reels without sound do worse; add music or a voice unless it's meant to be silent.")
	} else {
		if m := reLUFS.FindAllStringSubmatch(log, -1); m != nil { // the summary comes last, after a line per 100 ms
			l, _ := strconv.ParseFloat(m[len(m)-1][1], 64)
			switch {
			case l < -20:
				out = append(out, fmt.Sprintf("Too quiet: %.1f LUFS. Aim for about -14 (ffmpeg -af loudnorm=I=-14:TP=-1.5).", l))
			case l > -10:
				out = append(out, fmt.Sprintf("Too loud: %.1f LUFS. Aim for about -14 (ffmpeg -af loudnorm=I=-14:TP=-1.5).", l))
			}
		}
		if m := rePeak.FindAllStringSubmatch(log, -1); m != nil {
			if p, err := strconv.ParseFloat(m[len(m)-1][1], 64); err == nil && p > -1 {
				out = append(out, fmt.Sprintf("Peaks reach %.1f dBFS and may crackle on phones. Keep them under -1.", p))
			}
		}
		for _, m := range reSilence.FindAllStringSubmatch(log, -1) {
			out = append(out, fmt.Sprintf("Silence for %ss from %s: dead air.", trim(m[2]), clock(m[1])))
		}
	}
	for _, m := range reFreeze.FindAllStringSubmatch(log, -1) {
		out = append(out, fmt.Sprintf("Picture doesn't move for %ss from %s.", trim(m[2]), clock(m[1])))
	}
	for _, m := range reBlack.FindAllStringSubmatch(log, -1) {
		if m[1] == "0" {
			out = append(out, fmt.Sprintf("Opens on %ss of black: the first frame should already hook.", trim(m[2])))
		} else {
			out = append(out, fmt.Sprintf("Black for %ss from %s.", trim(m[2]), clock(m[1])))
		}
	}
	return out
}

func trim(s string) string {
	f, _ := strconv.ParseFloat(s, 64)
	return strconv.FormatFloat(f, 'f', 1, 64)
}

func clock(s string) string {
	f, _ := strconv.ParseFloat(s, 64)
	if f < 0 {
		f = 0
	}
	return fmt.Sprintf("%d:%04.1f", int(f)/60, f-float64(int(f)/60*60))
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}
