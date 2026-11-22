package files

import "testing"

func TestSafeNameKeepsEveryScript(t *testing.T) {
	for in, want := range map[string]string{
		"ইংরেজি মাস.mp4":    "ইংরেজি মাস.mp4",
		"हिन्दी-वीडियो.mp4": "हिन्दी-वीडियो.mp4",
		"../a/b:c?.mp4": "b_c_.mp4",
		"con.mp4":       "_con.mp4",
	} {
		if got := SafeName(in); got != want {
			t.Errorf("SafeName(%q) = %q, want %q", in, got, want)
		}
	}
}
