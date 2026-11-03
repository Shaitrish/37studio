//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// attachParentProcess asks AttachConsole for the console of whatever started this program.
const attachParentProcess = ^uintptr(0) // (DWORD)-1

// x/sys/windows has no wrapper for this one.
var attachConsoleProc = windows.NewLazySystemDLL("kernel32.dll").NewProc("AttachConsole")

// attachConsole gives `37studio gen|clip|transcribe` somewhere to print. The app is built for the GUI subsystem, so
// that no console window appears behind it when it opens from the Start menu -- which also leaves a run from a
// terminal with nowhere to write. Joining the terminal's own console fixes that.
func attachConsole() {
	if ok, _, _ := attachConsoleProc.Call(attachParentProcess); ok == 0 {
		return // opened from the Start menu: there is no console to join
	}
	toConsole(windows.STD_OUTPUT_HANDLE, &os.Stdout)
	toConsole(windows.STD_ERROR_HANDLE, &os.Stderr)
}

// toConsole points one standard stream at the console just joined, unless it already goes somewhere. The agent
// runs these commands with pipes and reads what they print; overwriting those would throw its output away.
func toConsole(stream uint32, f **os.File) {
	if h, err := windows.GetStdHandle(stream); err == nil && h != 0 && h != windows.InvalidHandle {
		return
	}
	con, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		return
	}
	windows.SetStdHandle(stream, windows.Handle(con.Fd()))
	*f = con
}
