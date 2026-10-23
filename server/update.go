package main

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const version = "0.5.0"

// Every copy checks studio.shaitrish.com for new releases; point STUDIO37_UPDATE_URL elsewhere to follow a fork.
var updateURL = env("STUDIO37_UPDATE_URL", "https://studio.shaitrish.com/version.json")

// ---- update check: version.json, fetched at most every 6 hours

var (
	updMu   sync.Mutex
	updAt   time.Time
	updInfo map[string]any
)

func checkUpdate(r *http.Request) (any, error) {
	updMu.Lock()
	defer updMu.Unlock()
	if updInfo == nil || time.Since(updAt) > 6*time.Hour {
		updAt = time.Now()
		var v struct {
			Version string `json:"version"`
			Notes   string `json:"notes"`
			URL     string `json:"url"`
		}
		if err := call(get(updateURL, ""), &v); err != nil {
			return map[string]any{"current": version, "error": err.Error()}, nil // offline is fine; try again later
		}
		updInfo = map[string]any{"current": version, "latest": v.Version, "notes": v.Notes, "url": v.URL, "newer": newer(v.Version, version)}
	}
	return updInfo, nil
}

// newer reports whether a > b for dotted versions like 0.3.10.
func newer(a, b string) bool {
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
