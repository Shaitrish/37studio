package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// HyperFrames Studio is the timeline editor: editors drag, trim and split clips, change text and audio,
// and every edit saves to comp/. Studio only binds 127.0.0.1 and serves from "/", so each one gets its own
// public port here (STUDIO37_STUDIO_PORT and up), reverse-proxied behind the app's password. That also makes it
// reachable from outside a Docker container. ponytail: one port per open Studio, 50 at most.

var (
	studioBase  = int(num(env("STUDIO37_STUDIO_PORT", "3100")))
	studioMu    sync.Mutex
	studioPorts = map[int64]int{} // project -> public port
	studioSrvs  = map[int64]*http.Server{}
)

func openStudio(r *http.Request) (any, error) {
	id := pathID(r)
	comp := filepath.Join(projectDir(id), "comp")
	if _, err := os.Stat(filepath.Join(comp, "index.html")); err != nil {
		return nil, bad("no composition yet; ask the agent to build a preview first")
	}
	studioMu.Lock()
	defer studioMu.Unlock()
	if p, ok := studioPorts[id]; ok {
		return map[string]any{"port": p, "project": "comp"}, nil
	}

	public, ln := 0, net.Listener(nil)
	host := strings.Split(addr, ":")[0] // same interface as the app: this computer only by default
	for p := studioBase; p < studioBase+50 && ln == nil; p++ {
		if l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, p)); err == nil {
			public, ln = p, l
		}
	}
	if ln == nil {
		return nil, bad("all timeline editor ports are busy; close one first")
	}

	// --background returns once Studio is ready and reuses a session that survived a 37Studio restart.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "npx", "-y", "hyperframes@"+hfVersion, "preview", "--background", "--no-open", "--json",
		"--port", fmt.Sprint(public+1000))
	cmd.Dir = comp
	out, err := cmd.Output()
	var res struct {
		OK     bool `json:"ok"`
		Result struct {
			Port int `json:"port"`
		} `json:"result"`
	}
	if err != nil || json.Unmarshal(lastLine(out), &res) != nil || !res.OK {
		ln.Close()
		return nil, fmt.Errorf("could not start the timeline editor: %v %s", err, tail(string(out), 300))
	}

	target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", res.Result.Port))
	proxy := &httputil.ReverseProxy{Rewrite: func(pr *httputil.ProxyRequest) { pr.SetURL(target) }}
	srv := &http.Server{Handler: basicAuth(proxy)}
	go srv.Serve(ln)
	studioPorts[id], studioSrvs[id] = public, srv
	return map[string]any{"port": public, "project": "comp"}, nil
}

func closeStudio(r *http.Request) (any, error) {
	id := pathID(r)
	studioMu.Lock()
	defer studioMu.Unlock()
	if srv := studioSrvs[id]; srv != nil {
		srv.Close()
	}
	delete(studioPorts, id)
	delete(studioSrvs, id)
	cmd := exec.Command("npx", "-y", "hyperframes@"+hfVersion, "preview", "--stop")
	cmd.Dir = filepath.Join(projectDir(id), "comp")
	return nil, cmd.Run()
}

func studioPort(id int64) any {
	studioMu.Lock()
	defer studioMu.Unlock()
	if p, ok := studioPorts[id]; ok {
		return p
	}
	return nil
}

func lastLine(b []byte) []byte {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	return []byte(lines[len(lines)-1])
}
