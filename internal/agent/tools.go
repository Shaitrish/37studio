package agent

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/proc"
	"github.com/Shaitrish/37studio/internal/textutil"
)

// Optional agent CLIs install on first use instead of shipping in the image: each is about 400 MB installed (OpenCode's
// npm package pulls all four of its Linux builds), and most people only use Claude. They go to data/tools at pinned
// versions, so they install once, survive updates, and stay as reproducible as the image. To bump one, change it here.
var tools = map[string]struct{ pkg, version string }{
	"codex":    {"@openai/codex", "0.164.0"},
	"opencode": {"opencode-ai", "1.19.4"},
}

func toolsDir() string { return filepath.Join(config.DataDir, "tools") }

// UseToolsPath puts data/tools on this process's PATH, so on-demand installs are found like built-in commands.
// main calls it once at startup.
func UseToolsPath() {
	os.Setenv("PATH", filepath.Join(toolsDir(), "node_modules", ".bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// ToolInstalled reports whether an agent CLI can run here: installed on demand, or already on the machine (from source).
func ToolInstalled(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

var toolMu sync.Mutex

// EnsureTool installs an agent CLI into data/tools if it isn't there yet. The first time takes a minute or so.
func EnsureTool(name string) error {
	t, ok := tools[name]
	if !ok {
		return fmt.Errorf("unknown tool %q", name)
	}
	toolMu.Lock() // one npm install at a time; a second caller waits and then finds it installed
	defer toolMu.Unlock()
	if ToolInstalled(name) {
		return nil
	}
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("can't install %s: npm isn't available. From source, run npm i -g %s@%s", name, t.pkg, t.version)
	}
	os.MkdirAll(toolsDir(), 0o755)
	cmd := proc.Command("npm", "install", "--prefix", toolsDir(), "--no-audit", "--no-fund", "--loglevel=error", t.pkg+"@"+t.version)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("installing %s failed: %s", name, textutil.Tail(strings.TrimSpace(string(out)), 600))
	}
	ocMu.Lock()
	ocList = nil // OpenCode's model list can be built now
	ocMu.Unlock()
	return nil
}

// InstallTool is POST /api/tools/{name}: Settings' "Set up" buttons.
func InstallTool(r *http.Request) (any, error) {
	name := r.PathValue("name")
	if _, ok := tools[name]; !ok {
		return nil, httpapi.Bad("unknown tool " + name + "; one of: codex, opencode")
	}
	if err := EnsureTool(name); err != nil {
		return nil, err
	}
	return map[string]bool{"installed": true}, nil
}

// ShellReady reports whether the agent can run shell commands at all. Claude Code's Bash tool uses Git for
// Windows' bash there -- the prompts lean on ls, cp, head and jq -- and looks for it where Git installs itself
// rather than on PATH, so a computer without Git has an editor agent that fails on its first command. The
// bash.exe in System32 is the WSL launcher, not a shell, so finding that one doesn't count.
func ShellReady() bool {
	if runtime.GOOS != "windows" {
		return true // every Unix has a shell
	}
	for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramW6432"), os.Getenv("ProgramFiles(x86)"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs")} {
		if base == "" {
			continue
		}
		if st, err := os.Stat(filepath.Join(base, "Git", "bin", "bash.exe")); err == nil && !st.IsDir() {
			return true
		}
	}
	p, err := exec.LookPath("bash")
	return err == nil && !strings.EqualFold(filepath.Dir(p), filepath.Join(os.Getenv("SystemRoot"), "System32"))
}
