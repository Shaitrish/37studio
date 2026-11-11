//go:build !windows

package server

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Shaitrish/37studio/internal/proc"
)

// showInFolder opens the folder holding p in the file manager, with p selected where the file manager can do that.
func showInFolder(p string) error {
	if runtime.GOOS == "darwin" {
		return proc.Command("open", "-R", p).Run()
	}
	// Nautilus, Dolphin, Nemo, Caja and Thunar answer this D-Bus call by opening the folder with the file selected;
	// any other file manager gets the folder from xdg-open. dbus-send splits an array at commas, so a name with one
	// goes straight to xdg-open.
	if !strings.Contains(p, ",") {
		uri := (&url.URL{Scheme: "file", Path: p}).String()
		if proc.Command("dbus-send", "--session", "--print-reply", "--reply-timeout=5000", "--dest=org.freedesktop.FileManager1",
			"--type=method_call", "/org/freedesktop/FileManager1", "org.freedesktop.FileManager1.ShowItems",
			"array:string:"+uri, "string:").Run() == nil {
			return nil
		}
	}
	return proc.Command("xdg-open", filepath.Dir(p)).Run()
}
