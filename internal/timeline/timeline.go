// Package timeline runs HyperFrames Studio, the hand-editing timeline, per project behind the app's password.
package timeline

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

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/textutil"
)

var studioBase = int(store.Num(config.Env("STUDIO37_STUDIO_PORT", "3100")))

var studioMu sync.Mutex

var studioPorts = map[int64]int{} // project -> public port

var studioSrvs = map[int64]*http.Server{}

func Open(r *http.Request) (any, error) {
	id := httpapi.PathID(r)
	comp := filepath.Join(files.ProjectDir(id), "comp")
	if _, err := os.Stat(filepath.Join(comp, "index.html")); err != nil {
		return nil, httpapi.Bad("no composition yet; ask the agent to build a preview first")
	}
	studioMu.Lock()
	defer studioMu.Unlock()
	if p, ok := studioPorts[id]; ok {
		return map[string]any{"port": p, "project": "comp"}, nil
	}

	public, ln := 0, net.Listener(nil)
	host := strings.Split(config.Addr, ":")[0] // same interface as the app: this computer only by default
	for p := studioBase; p < studioBase+50 && ln == nil; p++ {
		if l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, p)); err == nil {
			public, ln = p, l
		}
	}
	if ln == nil {
		return nil, httpapi.Bad("all timeline editor ports are busy; close one first")
	}

	// --background returns once Studio is ready and reuses a session that survived a 37Studio restart.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "npx", "-y", "hyperframes@"+config.HFVersion, "preview", "--background", "--no-open", "--json",
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
		return nil, fmt.Errorf("could not start the timeline editor: %v %s", err, textutil.Tail(string(out), 300))
	}

	target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", res.Result.Port))
	proxy := &httputil.ReverseProxy{Rewrite: func(pr *httputil.ProxyRequest) { pr.SetURL(target) }}
	srv := &http.Server{Handler: httpapi.Guard(proxy)}
	go srv.Serve(ln)
	studioPorts[id], studioSrvs[id] = public, srv
	return map[string]any{"port": public, "project": "comp"}, nil
}

func Close(r *http.Request) (any, error) {
	id := httpapi.PathID(r)
	studioMu.Lock()
	defer studioMu.Unlock()
	if srv := studioSrvs[id]; srv != nil {
		srv.Close()
	}
	delete(studioPorts, id)
	delete(studioSrvs, id)
	cmd := exec.Command("npx", "-y", "hyperframes@"+config.HFVersion, "preview", "--stop")
	cmd.Dir = filepath.Join(files.ProjectDir(id), "comp")
	return nil, cmd.Run()
}

func Port(id int64) any {
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
