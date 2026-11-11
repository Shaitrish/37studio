package proc

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

// UseLoginPath gives this process the PATH a terminal would have. Opened from the app menu, the desktop app gets the
// session's bare PATH, which leaves out where Node, npx and Claude Code usually live (nvm, ~/.local/bin, Homebrew),
// so renders failed with `exec: "npx": executable file not found` although the terminal finds it. It asks the
// user's login shell for its PATH, as VS Code does, and adds the usual install folders that exist.
func UseLoginPath() {
	if runtime.GOOS == "windows" { // Windows apps get the user's full PATH from the registry
		return
	}
	os.Setenv("PATH", strings.Join(mergePaths(filepath.SplitList(os.Getenv("PATH")), shellPath(), knownDirs()), string(os.PathListSeparator)))
}

// mergePaths keeps the current order, then adds the shell's folders and the known ones it lacks, without repeats.
func mergePaths(lists ...[]string) []string {
	var out []string
	for _, l := range lists {
		for _, d := range l {
			if d != "" && !slices.Contains(out, d) {
				out = append(out, d)
			}
		}
	}
	return out
}

// shellPath is $PATH as the user's interactive login shell sets it (nvm loads in .bashrc/.zshrc, so -i matters).
// A shell that prints a banner or hangs is fine: the marker finds the line, and the timeout ends a stuck shell.
func shellPath() []string {
	sh := os.Getenv("SHELL")
	if sh == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := CommandContext(ctx, sh, "-ilc", `printf '\n__37studio_path__%s\n' "$PATH"`)
	cmd.Stdin = nil
	out, _ := cmd.Output()
	for _, line := range strings.Split(string(out), "\n") {
		if p, ok := strings.CutPrefix(line, "__37studio_path__"); ok {
			return filepath.SplitList(strings.TrimSpace(p))
		}
	}
	return nil
}

// knownDirs are where Node, npm's global tools and Claude Code are installed by the usual installers, newest Node
// first, kept only if they exist: for a shell that couldn't be asked, or one set up only in a file it didn't read.
func knownDirs() []string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return nil
	}
	nvm := os.Getenv("NVM_DIR")
	if nvm == "" {
		nvm = filepath.Join(home, ".nvm")
	}
	nodes, _ := filepath.Glob(filepath.Join(nvm, "versions", "node", "*", "bin"))
	slices.SortFunc(nodes, func(a, b string) int { return compareVersions(versionOf(b), versionOf(a)) })
	dirs := append([]string{filepath.Join(home, ".local", "bin"), filepath.Join(home, "bin")}, nodes...)
	dirs = append(dirs, filepath.Join(home, ".volta", "bin"), filepath.Join(home, ".npm-global", "bin"), filepath.Join(home, ".bun", "bin"),
		filepath.Join(home, ".local", "share", "fnm", "aliases", "default", "bin"), "/opt/homebrew/bin", "/usr/local/bin")
	var out []string
	for _, d := range dirs {
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			out = append(out, d)
		}
	}
	return out
}

// versionOf reads "v24.22.0" out of …/versions/node/v24.22.0/bin.
func versionOf(binDir string) string {
	return strings.TrimPrefix(filepath.Base(filepath.Dir(binDir)), "v")
}

// compareVersions compares dotted versions numerically: 24.2.0 is newer than 9.11.0.
func compareVersions(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := range max(len(pa), len(pb)) {
		var x, y int
		if i < len(pa) {
			x = atoi(pa[i])
		}
		if i < len(pb) {
			y = atoi(pb[i])
		}
		if x != y {
			return x - y
		}
	}
	return 0
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}
