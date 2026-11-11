package agent

import "testing"

func TestResizedName(t *testing.T) {
	for _, c := range [][3]string{
		{"out/promo.mp4", "9:16", "promo-9x16.mp4"},
		{"out/promo-9x16.mp4", "16:9", "promo-16x9.mp4"}, // a copy of a copy doesn't stack suffixes
		{"out/reel.mov", "1:1", "reel-1x1.mp4"},
	} {
		if got := resizedName(c[0], c[1]); got != c[2] {
			t.Errorf("resizedName(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}
