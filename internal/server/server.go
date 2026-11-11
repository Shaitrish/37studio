// Package server is the whole app behind one HTTP surface: the database, keys, startup recovery, the background
// loops and the routes. cmd/37studio serves it as the Docker and cloud binary; cmd/37studio-desktop serves it to a
// native window. Nothing below cmd imports it.
package server

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Shaitrish/37studio/internal/agent"
	"github.com/Shaitrish/37studio/internal/assistant"
	"github.com/Shaitrish/37studio/internal/autopilot"
	"github.com/Shaitrish/37studio/internal/company"
	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/costs"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/gen"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/keys"
	"github.com/Shaitrish/37studio/internal/mcp"
	"github.com/Shaitrish/37studio/internal/project"
	"github.com/Shaitrish/37studio/internal/social"
	"github.com/Shaitrish/37studio/internal/source"
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/templates"
	"github.com/Shaitrish/37studio/internal/timeline"
	"github.com/Shaitrish/37studio/internal/transcribe"
	"github.com/Shaitrish/37studio/internal/update"
)

// CLI runs the agent's helper commands (gen, clip, transcribe, sql, stats, strategy) when args name one. The agent calls them as
// `37studio …`, so every build of the app, desktop included, has to answer to them.
func CLI(args []string) (code int, ok bool) {
	if len(args) == 0 {
		return 0, false
	}
	switch args[0] {
	case "gen":
		return agent.GenCLI(args[1:]), true
	case "clip":
		return project.ClipCLI(args[1:]), true
	case "transcribe":
		return transcribe.CLI(args[1:]), true
	case "sql":
		return source.SQLCLI(args[1:]), true
	case "stats":
		return social.StatsCLI(args[1:]), true
	case "strategy":
		return social.StrategyCLI(args[1:]), true
	}
	return 0, false
}

// WebFS is the built UI. Nil serves config.WebDir from disk; the desktop build embeds it instead.
var WebFS fs.FS

// Init opens the data folder and starts the background work. Set config.DataDir before calling it.
func Init() error {
	var err error
	if config.DataDir, err = filepath.Abs(config.DataDir); err != nil {
		return err
	}
	store.Open()
	agent.UseToolsPath()
	keys.Load()
	social.SealTokens()
	// A restart kills child agents; don't leave projects stuck in "running".
	store.DB.Exec(`update projects set status='review' where status='running'`)
	store.DB.Exec(`update tenants set scan_status='failed', scan_note='Interrupted by a server restart.' where scan_status='running'`)
	agent.AfterTurn = append(agent.AfterTurn, autopilot.Advance, project.ClipAdvance)
	agent.ResumeQueued()
	go social.PostLoop()
	go autopilot.Loop()
	go project.ClipLoop()
	go social.InboxLoop()
	go social.StatsLoop()
	return nil
}

// Serve answers on ln until ctx ends, then stops taking requests and ends the agent runs and renders still going,
// which would otherwise outlive the app in their own process groups. The next Init marks their projects for review.
func Serve(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{Handler: httpapi.Guard(Routes())}
	log.Printf("37studio listening on %s, data in %s", ln.Addr(), config.DataDir)
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := srv.Shutdown(shut)
	agent.KillAll()
	log.Print("37studio stopped")
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	return err
}

// Routes is the whole HTTP surface: the JSON API, the MCP endpoint (the same API as tools), files and the UI.
func Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/meta", httpapi.JSON(getMeta))
	mux.HandleFunc("GET /api/settings", httpapi.JSON(store.GetSettings))
	mux.HandleFunc("GET /api/voices", httpapi.JSON(gen.ListVoices))
	mux.HandleFunc("PUT /api/settings", httpapi.JSON(store.PutSettings))

	mux.HandleFunc("GET /api/tenants", httpapi.JSON(company.List))
	mux.HandleFunc("POST /api/tenants", httpapi.JSON(company.Save))
	mux.HandleFunc("POST /api/tenants/scan", httpapi.JSON(company.ScanNew))
	mux.HandleFunc("POST /api/tenants/{id}/scan", httpapi.JSON(company.Scan))
	mux.HandleFunc("GET /api/tenants/{id}", httpapi.JSON(company.Get))
	mux.HandleFunc("PUT /api/tenants/{id}", httpapi.JSON(company.Save))
	mux.HandleFunc("DELETE /api/tenants/{id}", httpapi.JSON(company.Delete))
	mux.HandleFunc("POST /api/tenants/{id}/assets", httpapi.JSON(company.UploadAssets))
	mux.HandleFunc("DELETE /api/tenants/{id}/assets", httpapi.JSON(company.DeleteAsset))
	mux.HandleFunc("POST /api/tenants/{id}/source", httpapi.JSON(source.Upload))
	mux.HandleFunc("PUT /api/tenants/{id}/assets", httpapi.JSON(company.UpdateAsset))
	mux.HandleFunc("GET /api/tenants/{id}/voices", httpapi.JSON(gen.ListBrandVoices))
	mux.HandleFunc("POST /api/tenants/{id}/voices", httpapi.JSON(gen.AddBrandVoice))
	mux.HandleFunc("POST /api/tenants/{id}/voices/clone", httpapi.JSON(gen.CloneBrandVoice))
	mux.HandleFunc("PUT /api/brand-voices/{id}", httpapi.JSON(gen.UpdateBrandVoice))
	mux.HandleFunc("DELETE /api/brand-voices/{id}", httpapi.JSON(gen.DeleteBrandVoice))

	mux.HandleFunc("GET /api/campaigns", httpapi.JSON(company.ListCampaigns))
	mux.HandleFunc("POST /api/campaigns", httpapi.JSON(company.SaveCampaign))
	mux.HandleFunc("GET /api/campaigns/{id}", httpapi.JSON(company.GetCampaign))
	mux.HandleFunc("PUT /api/campaigns/{id}", httpapi.JSON(company.SaveCampaign))
	mux.HandleFunc("DELETE /api/campaigns/{id}", httpapi.JSON(company.DeleteCampaign))

	mux.HandleFunc("GET /api/projects", httpapi.JSON(project.List))
	mux.HandleFunc("POST /api/projects", httpapi.JSON(project.Save))
	mux.HandleFunc("GET /api/projects/{id}", httpapi.JSON(project.Get))
	mux.HandleFunc("PUT /api/projects/{id}", httpapi.JSON(project.Save))
	mux.HandleFunc("DELETE /api/projects/{id}", httpapi.JSON(project.Delete))
	mux.HandleFunc("POST /api/projects/{id}/inputs", httpapi.JSON(project.UploadInputs))
	mux.HandleFunc("POST /api/projects/{id}/import", httpapi.JSON(project.ImportVideo))
	mux.HandleFunc("DELETE /api/projects/{id}/import", httpapi.JSON(project.CancelImport))
	mux.HandleFunc("POST /api/projects/{id}/transcript", httpapi.JSON(transcribe.Action))
	mux.HandleFunc("DELETE /api/projects/{id}/files", httpapi.JSON(project.DeleteProjectFile))
	mux.HandleFunc("GET /api/projects/{id}/events", httpapi.JSON(project.ListEvents))
	mux.HandleFunc("POST /api/projects/{id}/messages", httpapi.JSON(agent.PostMessage))
	mux.HandleFunc("POST /api/projects/{id}/attachments", httpapi.JSON(project.UploadAttachments))
	mux.HandleFunc("POST /api/projects/{id}/stop", httpapi.JSON(agent.StopRun))
	mux.HandleFunc("POST /api/projects/{id}/render", httpapi.JSON(agent.RenderFinal))
	mux.HandleFunc("POST /api/projects/{id}/generate", httpapi.JSON(agent.GenerateMedia))
	mux.HandleFunc("POST /api/projects/{id}/studio", httpapi.JSON(timeline.Open))
	mux.HandleFunc("DELETE /api/projects/{id}/studio", httpapi.JSON(timeline.Close))

	mux.HandleFunc("GET /api/costs", httpapi.JSON(costs.Report))
	mux.HandleFunc("GET /api/library", httpapi.JSON(project.ListLibrary))
	mux.HandleFunc("GET /api/storage", httpapi.JSON(project.Storage))
	mux.HandleFunc("POST /api/storage/cleanup", httpapi.JSON(project.CleanDrafts))
	mux.HandleFunc("GET /api/backup", store.Backup) // a zip download, not JSON; no MCP tool, since an agent has nowhere to keep it
	mux.HandleFunc("POST /api/desktop/open", httpapi.JSON(desktopOpen))
	mux.HandleFunc("POST /api/desktop/save", httpapi.JSON(desktopSave))
	mux.HandleFunc("POST /api/desktop/reveal", httpapi.JSON(desktopReveal))
	mux.HandleFunc("GET /api/templates", httpapi.JSON(templates.List))
	mux.HandleFunc("DELETE /api/templates/{id}", httpapi.JSON(templates.Delete))
	mux.HandleFunc("POST /api/projects/{id}/template", httpapi.JSON(templates.Save))
	mux.HandleFunc("POST /api/tools/{name}", httpapi.JSON(agent.InstallTool))
	mux.HandleFunc("GET /api/codex", httpapi.JSON(agent.CodexStatus))
	mux.HandleFunc("POST /api/codex/login", httpapi.JSON(agent.CodexLogin))
	mux.HandleFunc("DELETE /api/codex/login", httpapi.JSON(agent.CodexLogout))
	mux.HandleFunc("GET /api/keys", httpapi.JSON(keys.List))
	mux.HandleFunc("PUT /api/keys", httpapi.JSON(keys.Put))
	mux.HandleFunc("GET /api/update", httpapi.JSON(update.Check))
	mux.HandleFunc("GET /api/autopilots", httpapi.JSON(autopilot.List))
	mux.HandleFunc("POST /api/autopilots", httpapi.JSON(autopilot.Save))
	mux.HandleFunc("PUT /api/autopilots/{id}", httpapi.JSON(autopilot.Save))
	mux.HandleFunc("DELETE /api/autopilots/{id}", httpapi.JSON(autopilot.Delete))
	mux.HandleFunc("POST /api/autopilots/{id}/run", httpapi.JSON(autopilot.Run))
	mux.HandleFunc("POST /api/projects/{id}/approve", httpapi.JSON(autopilot.Approve))
	mux.HandleFunc("POST /api/projects/{id}/skip", httpapi.JSON(autopilot.Skip))

	mux.HandleFunc("GET /api/oauth/{provider}/start", social.OAuthStart)
	mux.HandleFunc("GET /api/oauth/{provider}/callback", social.OAuthCallback)
	mux.HandleFunc("GET /api/tenants/{id}/social", httpapi.JSON(social.ListAccounts))
	mux.HandleFunc("GET /api/social/pending/{key}", httpapi.JSON(social.GetPending))
	mux.HandleFunc("POST /api/social/pending/{key}", httpapi.JSON(social.SavePending))
	mux.HandleFunc("DELETE /api/social/{id}", httpapi.JSON(social.Disconnect))
	mux.HandleFunc("POST /api/tenants/{id}/whatsapp", httpapi.JSON(social.ConnectWhatsApp))
	mux.HandleFunc("POST /api/tenants/{id}/bluesky", httpapi.JSON(social.ConnectBluesky))
	mux.HandleFunc("POST /api/tenants/{id}/mastodon", httpapi.JSON(social.ConnectMastodon))
	mux.HandleFunc("POST /api/projects/{id}/captions", httpapi.JSON(social.WriteCaptions))
	mux.HandleFunc("POST /api/projects/{id}/posts", httpapi.JSON(social.CreatePosts))
	mux.HandleFunc("POST /api/posts/{id}/retry", httpapi.JSON(social.RetryPost))
	mux.HandleFunc("DELETE /api/posts/{id}", httpapi.JSON(social.DeletePost))
	mux.HandleFunc("GET /api/inbox", httpapi.JSON(social.ListInbox))
	mux.HandleFunc("GET /api/inbox/thread", httpapi.JSON(social.GetThread))
	mux.HandleFunc("POST /api/inbox/draft", httpapi.JSON(social.DraftReply))
	mux.HandleFunc("POST /api/inbox/reply", httpapi.JSON(social.SendReply))
	mux.HandleFunc("GET /api/stats", httpapi.JSON(social.ListStats))
	mux.HandleFunc("POST /api/stats/refresh", httpapi.JSON(social.RefreshStats))
	mux.HandleFunc("GET /api/stats/videos", httpapi.JSON(social.ListPublished))
	mux.HandleFunc("POST /api/stats/review", httpapi.JSON(social.ReviewStrategy))
	mux.HandleFunc("GET /webhooks/meta", social.WhatsAppWebhook) // WhatsApp straight to this copy through a tunnel
	mux.HandleFunc("POST /webhooks/meta", social.WhatsAppWebhook)

	mux.HandleFunc("GET /api/assistant", httpapi.JSON(assistant.Get))
	mux.HandleFunc("POST /api/assistant/messages", httpapi.JSON(assistant.Post))
	mux.HandleFunc("POST /api/assistant/stop", httpapi.JSON(assistant.Stop))
	mux.HandleFunc("DELETE /api/assistant", httpapi.JSON(assistant.Reset))
	mux.Handle("POST /mcp", mcp.Handler(mux))
	mux.Handle("GET /mcp", mcp.Handler(mux)) // answers 405: no event stream

	mux.Handle("GET /files/", http.StripPrefix("/files", files.Handler()))
	ui := WebFS
	if ui == nil {
		ui = os.DirFS(config.WebDir)
	}
	web := http.FileServerFS(ui)
	mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			w.Header().Set("Cache-Control", "no-cache") // hashed bundles change name on rebuild; index.html must not go stale
		}
		web.ServeHTTP(w, r)
	}))
	return mux
}

func getMeta(r *http.Request) (any, error) {
	_, claudeErr := exec.LookPath(config.ClaudeBin)
	_, ffErr := exec.LookPath("ffmpeg")
	_, npxErr := exec.LookPath("npx") // HyperFrames renders, previews and the timeline editor all run through npx
	// Transcription and link imports set up their own venv from this computer's Python on first use.
	_, pyErr := transcribe.SystemPython()
	python := pyErr == nil || config.Env("STUDIO37_ASR_PYTHON", "") != ""
	return map[string]any{
		"os":         runtime.GOOS, // so the setup list can show this computer's install commands
		"presets":    agent.Presets,
		"qualities":  agent.AllQualities(),
		"hf_version": config.HFVersion,
		"version":    update.Version,
		"data_dir":   config.DataDir,
		"env": map[string]bool{
			"claude":       claudeErr == nil,
			"shell":        agent.ShellReady(), // Windows: Claude Code runs the agent's commands through Git for Windows' bash
			"ffmpeg":       ffErr == nil,
			"node":         npxErr == nil,
			"python":       python,
			"gemini":       os.Getenv("GEMINI_API_KEY") != "",
			"eleven":       os.Getenv("ELEVENLABS_API_KEY") != "",
			"anthropic":    os.Getenv("ANTHROPIC_API_KEY") != "" || os.Getenv("CLAUDE_CODE_OAUTH_TOKEN") != "",
			"codex":        agent.CodexReady(),
			"claude_login": keys.ClaudeLoggedIn(),
			"opencode":     agent.ToolInstalled("opencode"),
			"bypass":       config.Env("STUDIO37_AGENT_PERMISSIONS", "") == "bypass",
			"password":     config.Env("STUDIO37_PASSWORD", "") != "",
			"desktop":      Desktop.OpenURL != nil,
			"reveal":       canReveal(r), // files can be shown in this computer's file manager
		},
	}, nil
}
