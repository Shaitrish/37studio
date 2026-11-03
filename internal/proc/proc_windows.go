//go:build windows

package proc

import (
	"context"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Command and CommandContext build a command the way 37Studio wants every tool it runs to be built: with no
// console window. ffmpeg, ffprobe, npx, Python and the agent CLIs are all console programs and the desktop app is
// not, so without this Windows gives each run a console of its own and black windows flash over the app -- once
// per file while the Library lists footage.
func Command(name string, args ...string) *exec.Cmd { return noWindow(exec.Command(name, args...)) }

func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return noWindow(exec.CommandContext(ctx, name, args...))
}

func noWindow(cmd *exec.Cmd) *exec.Cmd {
	flags(cmd, windows.CREATE_NO_WINDOW)
	return cmd
}

// jobs holds the job object behind each running command until Done closes it. The handle has to stay open: closing
// it is what ends the job, so losing it would leave a run unstoppable.
var (
	jobMu sync.Mutex
	jobs  = map[*exec.Cmd]windows.Handle{}
)

// Start runs cmd in a job object of its own, so Kill and Terminate reach every process it spawns.
func Start(cmd *exec.Cmd) error {
	// Its own process group as well, so a Ctrl-C in a console 37Studio was started from doesn't reach the run.
	flags(cmd, windows.CREATE_NEW_PROCESS_GROUP|windows.CREATE_NO_WINDOW)
	job, jobErr := newJob()
	if err := cmd.Start(); err != nil {
		if jobErr == nil {
			windows.CloseHandle(job)
		}
		return err
	}
	// Assigning can only happen once the process exists, so a grandchild started in the first moments of a run
	// misses the job. Nothing in os/exec can start a process suspended, and the window is microseconds wide;
	// Kill falls back to walking Windows' own parent links when there is no job to end.
	if jobErr != nil {
		return nil // the run is up: only Kill's reach over its children is reduced
	}
	if assign(job, cmd.Process.Pid) != nil {
		windows.CloseHandle(job) // empty, so closing it ends nothing
		return nil
	}
	jobMu.Lock()
	jobs[cmd] = job
	jobMu.Unlock()
	return nil
}

// StartDetached runs cmd with no console and no job, so it survives 37Studio quitting.
func StartDetached(cmd *exec.Cmd) error {
	flags(cmd, windows.CREATE_NEW_PROCESS_GROUP|windows.DETACHED_PROCESS)
	// CreateProcess refuses the two together, and a detached process has no console window to suppress anyway.
	cmd.SysProcAttr.CreationFlags &^= windows.CREATE_NO_WINDOW
	return cmd.Start()
}

// Done closes cmd's job once it has been waited for, which also ends anything it left running.
func Done(cmd *exec.Cmd) {
	jobMu.Lock()
	defer jobMu.Unlock() // held across the close, so a Kill landing now can't terminate a reused handle
	if job, ok := jobs[cmd]; ok {
		delete(jobs, cmd)
		windows.CloseHandle(job)
	}
}

// Kill ends cmd and its children at once.
func Kill(cmd *exec.Cmd) error { return end(cmd) }

// Terminate ends cmd and its children. Windows has no SIGTERM for this, so it is the same as Kill: a job ends hard.
func Terminate(cmd *exec.Cmd) error { return end(cmd) }

// TerminatePID ends the tree under pid. For a detached run started by an earlier command, where no job handle is
// left in this process: taskkill walks Windows' own parent links instead.
func TerminatePID(pid int) error {
	if pid <= 0 {
		return nil
	}
	return Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
}

// Alive reports whether pid is a live process.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var code uint32
	if windows.GetExitCodeProcess(h, &code) != nil {
		return false
	}
	return code == stillActive
}

const stillActive = 259 // STILL_ACTIVE: the exit code Windows reports while a process is running

func end(cmd *exec.Cmd) error {
	if terminateJob(cmd) {
		return nil
	}
	if cmd.Process == nil {
		return nil
	}
	return TerminatePID(cmd.Process.Pid)
}

// terminateJob ends cmd's job if it still has one, holding the lock across the call so Done cannot close the
// handle underneath it: Windows hands the same number to the next job opened.
func terminateJob(cmd *exec.Cmd) bool {
	jobMu.Lock()
	defer jobMu.Unlock()
	job, ok := jobs[cmd]
	return ok && windows.TerminateJobObject(job, 1) == nil
}

// newJob makes a job that ends its processes when its last handle closes, so a run never outlives 37Studio.
func newJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

func assign(job windows.Handle, pid int) error {
	if job == 0 {
		return syscall.EINVAL
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.AssignProcessToJobObject(job, h)
}

func flags(cmd *exec.Cmd, f uint32) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= f
}
