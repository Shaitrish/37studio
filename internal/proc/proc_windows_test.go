//go:build windows

package proc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// npm installs Claude Code, Codex and OpenCode as a .cmd shim around a real program. Go would run the shim
// through cmd.exe, where the command line stops at 8191 characters, so proc follows it to the program itself.
func TestRealProgramFollowsAnNpmShim(t *testing.T) {
	dir := t.TempDir()
	// A stand-in for the program the shim wraps; only its path matters here.
	target := filepath.Join(dir, "node_modules", "@scope", "tool", "bin", "tool.exe")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	// An npm shim, written the way npm writes one.
	shim := "@ECHO off\r\nSETLOCAL\r\nCALL :find_dp0\r\n" +
		`"%dp0%\node_modules\@scope\tool\bin\tool.exe"   %*` + "\r\n"
	if err := os.WriteFile(filepath.Join(dir, "studio37shim.cmd"), []byte(shim), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if got := realProgram("studio37shim"); got != target {
		t.Errorf("got %q, want the program the shim names, %q", got, target)
	}
}

// npx's shim runs node on npx-cli.js. Following it to node.exe would drop the script and hand npx's arguments
// to node, which fails with `node.exe: bad option: -y`, so a shim shaped like that has to be left alone.
func TestRealProgramLeavesANodeShimAlone(t *testing.T) {
	dir := t.TempDir()
	node := filepath.Join(dir, "node.exe")
	if err := os.WriteFile(node, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(dir, "studio37npx.cmd")
	// npm's npx shim, with the program and the script it runs on the same line.
	body := "@ECHO off\r\nSETLOCAL\r\nCALL :find_dp0\r\n" +
		`IF EXIST "%dp0%\node.exe" ( SET "_prog=%dp0%\node.exe" ) ELSE ( SET "_prog=node" )` + "\r\n" +
		`endLocal & goto #_undefined_# 2>NUL || title %COMSPEC% & "%_prog%"  "%dp0%\node_modules\npm\bin\npx-cli.js" %*` + "\r\n"
	if err := os.WriteFile(shim, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if got := realProgram("studio37npx"); got != shim {
		t.Errorf("got %q, want the shim itself (%q): node would have been run without its script", got, shim)
	}
}

// A shim pointing nowhere, and a program that is no shim at all, must both be left as they are.
func TestRealProgramLeavesOtherThingsAlone(t *testing.T) {
	dir := t.TempDir()
	shim := filepath.Join(dir, "studio37broken.cmd")
	broken := "@ECHO off\r\n" + `"%dp0%\gone\missing.exe" %*` + "\r\n"
	if err := os.WriteFile(shim, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if got := realProgram("studio37broken"); got != shim {
		t.Errorf("a shim naming a program that isn't there should stay the shim: got %q", got)
	}
	if got := realProgram("studio37nosuchprogram"); got != "studio37nosuchprogram" {
		t.Errorf("an unknown program should be handed to exec unchanged: got %q", got)
	}
	// The test binary itself is a plain .exe: it should come back as its full path, not a shim target.
	if got := realProgram(os.Args[0]); !strings.EqualFold(got, os.Args[0]) {
		t.Errorf("a real program should resolve to itself: got %q, want %q", got, os.Args[0])
	}
}

// Finish setup exists so people can install a missing tool with 37Studio open and press Check again. A program
// that wasn't there the first time has to be found the next time, so a miss must not be remembered.
func TestRealProgramFindsAToolInstalledLater(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if got := realProgram("studio37later"); got != "studio37later" {
		t.Fatalf("before it exists: got %q, want the bare name", got)
	}
	later := filepath.Join(dir, "studio37later.exe")
	if err := os.WriteFile(later, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := realProgram("studio37later"); got != later {
		t.Errorf("after installing it: got %q, want %q", got, later)
	}
}
