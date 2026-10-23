// 37Studio: multi-tenant video editing studio. Claude Code edits, HyperFrames renders.
package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

var (
	dataDir   = env("STUDIO37_DATA", "data")
	addr      = env("STUDIO37_ADDR", "127.0.0.1:8080") // this computer only; Docker sets :8080 and publishes it on 127.0.0.1
	webDir    = env("STUDIO37_WEB", "web/dist")
	hfVersion = env("HF_VERSION", "0.8.186")
	claudeBin = env("STUDIO37_CLAUDE", "claude")
)

var dotenv sync.Once

// env reads a setting. A .env in the working directory fills in variables the environment lacks, so keys added
// there apply on the next server restart (air restarts on every .go change) without restarting dev.sh.
func env(k, def string) string {
	dotenv.Do(func() {
		b, _ := os.ReadFile(".env")
		for _, line := range strings.Split(string(b), "\n") {
			if key, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok && !strings.HasPrefix(key, "#") && os.Getenv(key) == "" {
				os.Setenv(key, strings.Trim(v, `"'`))
			}
		}
	})
	if v := os.Getenv(k); v != "" {
		return v
	}
	if rest, ok := strings.CutPrefix(k, "STUDIO37_"); ok { // pre-rename name, so older .env files and launchers keep working
		return env("VIDBIT_"+rest, def)
	}
	return def
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "gen" {
		os.Exit(genCLI(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "transcribe" {
		os.Exit(transcribeCLI(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "license" {
		os.Exit(licenseCLI(os.Args[2:]))
	}
	var err error
	if dataDir, err = filepath.Abs(dataDir); err != nil {
		log.Fatal(err)
	}
	openDB()
	loadKeys()
	// A restart kills child agents; don't leave projects stuck in "running".
	db.Exec(`update projects set status='review' where status='running'`)
	db.Exec(`update tenants set scan_status='failed', scan_note='Interrupted by a server restart.' where scan_status='running'`)
	go postLoop()
	go autopilotLoop()
	go inboxLoop()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/meta", api(getMeta))
	mux.HandleFunc("GET /api/settings", api(getSettings))
	mux.HandleFunc("GET /api/voices", api(listVoices))
	mux.HandleFunc("PUT /api/settings", api(putSettings))

	mux.HandleFunc("GET /api/tenants", api(listTenants))
	mux.HandleFunc("POST /api/tenants", api(saveTenant))
	mux.HandleFunc("POST /api/tenants/scan", api(scanNewTenant))
	mux.HandleFunc("POST /api/tenants/{id}/scan", api(scanTenant))
	mux.HandleFunc("GET /api/tenants/{id}", api(getTenant))
	mux.HandleFunc("PUT /api/tenants/{id}", api(saveTenant))
	mux.HandleFunc("DELETE /api/tenants/{id}", api(deleteTenant))
	mux.HandleFunc("POST /api/tenants/{id}/assets", api(uploadAssets))
	mux.HandleFunc("DELETE /api/tenants/{id}/assets", api(deleteAsset))
	mux.HandleFunc("POST /api/tenants/{id}/source", api(uploadSource))
	mux.HandleFunc("PUT /api/tenants/{id}/assets", api(updateAsset))

	mux.HandleFunc("GET /api/campaigns", api(listCampaigns))
	mux.HandleFunc("POST /api/campaigns", api(saveCampaign))
	mux.HandleFunc("GET /api/campaigns/{id}", api(getCampaign))
	mux.HandleFunc("PUT /api/campaigns/{id}", api(saveCampaign))
	mux.HandleFunc("DELETE /api/campaigns/{id}", api(deleteCampaign))

	mux.HandleFunc("GET /api/projects", api(listProjects))
	mux.HandleFunc("POST /api/projects", api(saveProject))
	mux.HandleFunc("GET /api/projects/{id}", api(getProject))
	mux.HandleFunc("PUT /api/projects/{id}", api(saveProject))
	mux.HandleFunc("DELETE /api/projects/{id}", api(deleteProject))
	mux.HandleFunc("POST /api/projects/{id}/inputs", api(uploadInputs))
	mux.HandleFunc("DELETE /api/projects/{id}/files", api(deleteProjectFile))
	mux.HandleFunc("GET /api/projects/{id}/events", api(listEvents))
	mux.HandleFunc("POST /api/projects/{id}/messages", api(postMessage))
	mux.HandleFunc("POST /api/projects/{id}/attachments", api(uploadAttachments))
	mux.HandleFunc("POST /api/projects/{id}/stop", api(stopRun))
	mux.HandleFunc("POST /api/projects/{id}/render", api(renderFinal))
	mux.HandleFunc("POST /api/projects/{id}/generate", api(generateMedia))
	mux.HandleFunc("POST /api/projects/{id}/studio", api(openStudio))
	mux.HandleFunc("DELETE /api/projects/{id}/studio", api(closeStudio))

	mux.HandleFunc("GET /api/costs", api(getCosts))
	mux.HandleFunc("GET /api/keys", api(listKeys))
	mux.HandleFunc("PUT /api/keys", api(putKeys))
	mux.HandleFunc("GET /api/license", api(getLicense))
	mux.HandleFunc("PUT /api/license", api(putLicense))
	mux.HandleFunc("GET /api/update", api(checkUpdate))
	mux.HandleFunc("GET /api/autopilots", api(listAutopilots))
	mux.HandleFunc("POST /api/autopilots", api(saveAutopilot))
	mux.HandleFunc("PUT /api/autopilots/{id}", api(saveAutopilot))
	mux.HandleFunc("DELETE /api/autopilots/{id}", api(deleteAutopilot))
	mux.HandleFunc("POST /api/autopilots/{id}/run", api(runAutopilot))
	mux.HandleFunc("POST /api/projects/{id}/approve", api(approveProject))
	mux.HandleFunc("POST /api/projects/{id}/skip", api(skipProject))

	mux.HandleFunc("GET /api/oauth/{provider}/start", oauthStart)
	mux.HandleFunc("GET /api/oauth/{provider}/callback", oauthCallback)
	mux.HandleFunc("GET /api/tenants/{id}/social", api(listSocial))
	mux.HandleFunc("GET /api/social/pending/{key}", api(getPending))
	mux.HandleFunc("POST /api/social/pending/{key}", api(savePending))
	mux.HandleFunc("DELETE /api/social/{id}", api(disconnectSocial))
	mux.HandleFunc("POST /api/tenants/{id}/whatsapp", api(connectWhatsApp))
	mux.HandleFunc("POST /api/projects/{id}/captions", api(writeCaptions))
	mux.HandleFunc("POST /api/projects/{id}/posts", api(createPosts))
	mux.HandleFunc("POST /api/posts/{id}/retry", api(retryPost))
	mux.HandleFunc("DELETE /api/posts/{id}", api(deletePost))
	mux.HandleFunc("GET /api/inbox", api(listInbox))
	mux.HandleFunc("GET /api/inbox/thread", api(getThread))
	mux.HandleFunc("POST /api/inbox/draft", api(draftReply))
	mux.HandleFunc("POST /api/inbox/reply", api(sendReply))

	mux.Handle("GET /files/", http.StripPrefix("/files", filesHandler()))
	web := http.FileServer(http.Dir(webDir))
	mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			w.Header().Set("Cache-Control", "no-cache") // hashed bundles change name on rebuild; index.html must not go stale
		}
		web.ServeHTTP(w, r)
	}))

	log.Printf("37studio listening on %s, data in %s", addr, dataDir)
	log.Fatal(http.ListenAndServe(addr, basicAuth(mux)))
}

// basicAuth guards the whole app when STUDIO37_PASSWORD is set (any username).
func basicAuth(h http.Handler) http.Handler {
	pw := env("STUDIO37_PASSWORD", "")
	if pw == "" {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, p, ok := r.BasicAuth(); !ok || subtle.ConstantTimeCompare([]byte(p), []byte(pw)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="37studio"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

type apiErr struct {
	code int
	msg  string
}

func (e apiErr) Error() string { return e.msg }
func bad(msg string) error     { return apiErr{http.StatusBadRequest, msg} }

func api(fn func(r *http.Request) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, err := fn(r)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			code := http.StatusInternalServerError
			var ae apiErr
			if errors.As(err, &ae) {
				code = ae.code
			} else if errors.Is(err, sql.ErrNoRows) {
				code = http.StatusNotFound
			}
			w.WriteHeader(code)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(v)
	}
}

func pathID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id
}

func body(r *http.Request) (map[string]any, error) {
	m := map[string]any{}
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		return nil, bad("invalid JSON body")
	}
	return m, nil
}

func getMeta(r *http.Request) (any, error) {
	_, claudeErr := exec.LookPath(claudeBin)
	_, ffErr := exec.LookPath("ffmpeg")
	return map[string]any{
		"presets":    presets,
		"qualities":  qualities,
		"hf_version": hfVersion,
		"version":    version,
		"data_dir":   dataDir,
		"env": map[string]bool{
			"claude":    claudeErr == nil,
			"ffmpeg":    ffErr == nil,
			"gemini":    os.Getenv("GEMINI_API_KEY") != "",
			"eleven":    os.Getenv("ELEVENLABS_API_KEY") != "",
			"anthropic": os.Getenv("ANTHROPIC_API_KEY") != "" || os.Getenv("CLAUDE_CODE_OAUTH_TOKEN") != "",
			"bypass":    env("STUDIO37_AGENT_PERMISSIONS", "") == "bypass",
			"password":  env("STUDIO37_PASSWORD", "") != "",
		},
	}, nil
}
