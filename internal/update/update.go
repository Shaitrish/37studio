// Package update checks for new 37Studio releases.
package update

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/remote"
)

const Version = "0.5.0"

// Every copy checks studio.shaitrish.com for new releases; point STUDIO37_UPDATE_URL elsewhere to follow a fork.
var updateURL = config.Env("STUDIO37_UPDATE_URL", "https://studio.shaitrish.com/version.json")

var updMu sync.Mutex

var updAt time.Time

var updInfo map[string]any

func Check(r *http.Request) (any, error) {
	updMu.Lock()
	defer updMu.Unlock()
	if updInfo == nil || time.Since(updAt) > 6*time.Hour {
		updAt = time.Now()
		var v struct {
			Version string `json:"version"`
			Notes   string `json:"notes"`
			URL     string `json:"url"`
		}
		if err := remote.Call(remote.Get(updateURL, ""), &v); err != nil {
			return map[string]any{"current": Version, "error": err.Error()}, nil // offline is fine; try again later
		}
		updInfo = map[string]any{"current": Version, "latest": v.Version, "notes": v.Notes, "url": v.URL, "newer": Newer(v.Version, Version)}
	}
	return updInfo, nil
}

// Newer reports whether a > b for dotted versions like 0.3.10.
func Newer(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < max(len(pa), len(pb)); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x > y
		}
	}
	return false
}
