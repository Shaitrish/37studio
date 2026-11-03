//go:build !windows

package proc

import (
	"context"
	"os/exec"
	"syscall"
)

// Command and CommandContext build a command the way 37Studio wants every tool it runs to be built. There is
// nothing to add on Unix; on Windows they keep a console window from opening.
func Command(name string, args ...string) *exec.Cmd { return exec.Command(name, args...) }

func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

// Start runs cmd as the leader of its own process group, so Kill and Terminate reach its children too.
func Start(cmd *exec.Cmd) error {
	attr(cmd).Setpgid = true
	return cmd.Start()
}

// StartDetached runs cmd in its own session, so it survives this process and its process group.
func StartDetached(cmd *exec.Cmd) error {
	attr(cmd).Setsid = true
	return cmd.Start()
}

// Done releases what Start held for cmd. Nothing to release here; Windows closes the job handle.
func Done(*exec.Cmd) {}

// Kill ends cmd and its children at once.
func Kill(cmd *exec.Cmd) error { return send(cmd, syscall.SIGKILL) }

// Terminate asks cmd and its children to stop and clean up after themselves.
func Terminate(cmd *exec.Cmd) error { return send(cmd, syscall.SIGTERM) }

// TerminatePID asks the group led by pid to stop. For a detached run started by an earlier command.
func TerminatePID(pid int) error {
	if pid <= 0 {
		return nil
	}
	return syscall.Kill(-pid, syscall.SIGTERM)
}

// Alive reports whether pid is a live process (not a zombie).
func Alive(pid int) bool { return pid > 0 && syscall.Kill(pid, 0) == nil }

func send(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, sig)
}

func attr(cmd *exec.Cmd) *syscall.SysProcAttr {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	return cmd.SysProcAttr
}
