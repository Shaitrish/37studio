package proc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// helperEnv makes a run of this test binary act as the child process below instead of running the tests.
const helperEnv = "STUDIO37_PROC_HELPER"

// TestMain lets the test binary stand in for a long-running command, on any platform. With helperEnv set to
// "parent" it starts a second copy of itself, writes that copy's pid to the file named in the argument and waits;
// with "child" it only waits. Killing the parent must take the child with it.
func TestMain(m *testing.M) {
	switch os.Getenv(helperEnv) {
	case "parent":
		child := exec.Command(os.Args[0])
		child.Env = append(os.Environ(), helperEnv+"=child")
		if err := child.Start(); err != nil {
			os.Exit(1)
		}
		os.WriteFile(os.Args[len(os.Args)-1], []byte(strconv.Itoa(child.Process.Pid)), 0o644)
		time.Sleep(2 * time.Minute)
		os.Exit(0)
	case "child":
		time.Sleep(2 * time.Minute)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// Stop has to end the whole run, not just the command the app happens to hold: a render leaves ffmpeg and Chrome
// behind otherwise. This is what a process group does on Unix and a job object on Windows.
func TestKillEndsChildren(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	cmd := helper(t, "parent", pidFile)
	if err := Start(cmd); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { Kill(cmd); Done(cmd) })
	child := waitForPID(t, pidFile)

	if !Alive(cmd.Process.Pid) || !Alive(child) {
		t.Fatalf("not both up: parent %d, child %d", cmd.Process.Pid, child)
	}
	if err := Kill(cmd); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	cmd.Wait()
	Done(cmd)
	if gone := waitGone(cmd.Process.Pid) && waitGone(child); !gone {
		t.Errorf("still running after Kill: parent %v, child %v", Alive(cmd.Process.Pid), Alive(child))
	}
}

// Transcription keeps going after the agent's turn ends, so it is started detached and reached by pid afterwards.
func TestDetachedRunIsReachableByPID(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	cmd := helper(t, "parent", pidFile)
	if err := StartDetached(cmd); err != nil {
		t.Fatalf("StartDetached: %v", err)
	}
	pid := cmd.Process.Pid
	t.Cleanup(func() { TerminatePID(pid) })
	waitForPID(t, pidFile)

	if !Alive(pid) {
		t.Fatal("detached run isn't alive")
	}
	if err := TerminatePID(pid); err != nil {
		t.Fatalf("TerminatePID: %v", err)
	}
	cmd.Wait()
	if !waitGone(pid) {
		t.Error("still running after TerminatePID")
	}
}

// A pid nothing is using must not look alive, or a finished transcription would never be retried.
func TestAliveOnDeadAndImpossiblePIDs(t *testing.T) {
	cmd := helper(t, "child")
	if err := Start(cmd); err != nil {
		t.Fatalf("Start: %v", err)
	}
	pid := cmd.Process.Pid
	Kill(cmd)
	cmd.Wait()
	Done(cmd)
	if !waitGone(pid) {
		t.Error("a killed and waited-for process still reads as alive")
	}
	for _, bad := range []int{0, -1} {
		if Alive(bad) {
			t.Errorf("pid %d reads as alive", bad)
		}
	}
}

func helper(t *testing.T, role string, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), helperEnv+"="+role)
	return cmd
}

func waitForPID(t *testing.T, path string) int {
	t.Helper()
	for range 100 {
		if b, err := os.ReadFile(path); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("the helper never wrote its child's pid to %s", path)
	return 0
}

// waitGone gives the system a moment to reap the process, which neither signal nor job object does synchronously.
func waitGone(pid int) bool {
	for range 100 {
		if !Alive(pid) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}
