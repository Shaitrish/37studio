// Command 37studio-desktop is 37Studio as a native app: the same server as the Docker image, shown in a Wails window
// instead of a browser tab. It has its own go.mod so the Docker image and CI never build Wails or need cgo.
//
// The window loads the server's own http://127.0.0.1 address rather than Wails' asset scheme, so the page's origin is
// a real loopback one: the cross-origin guard, MCP's Origin check, OAuth return addresses and the timeline editor's
// URL (built from location.hostname) all work exactly as they do in a browser.
package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/server"
	"github.com/Shaitrish/37studio/internal/social"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// web is a copy of web/dist that desktop/build.sh puts here before building (gitignored).
//
//go:embed all:web
var web embed.FS

//go:embed icon.png
var icon []byte

func main() {
	// The agent calls `37studio gen|clip|transcribe` while the app runs: answer those first, with no window and no
	// single-instance lock, or every call would just focus the open window.
	attachConsole()
	if code, ok := server.CLI(os.Args[1:]); ok {
		os.Exit(code)
	}

	var win *application.WebviewWindow
	app := application.New(application.Options{
		Name:        "37Studio",
		Description: "AI video studio and social media desk",
		Icon:        icon,
		// One copy per user: a second one would open the same database and run Autopilot and the inbox twice.
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.shaitrish.studio37", // D-Bus refuses an element that starts with a digit
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if win != nil {
					win.Restore()
					win.Show()
					win.Focus()
				}
			},
		},
		Mac:   application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true}, // ponytail: until the tray keeps it running
		Linux: application.LinuxOptions{ProgramName: "37studio"},
	})

	dir, err := dataDir()
	if err != nil {
		log.Fatal(err)
	}
	config.DataDir = dir
	ui, err := fs.Sub(web, "web")
	if err != nil {
		log.Fatal(err)
	}
	server.WebFS = ui
	if err := server.Init(); err != nil {
		log.Fatal(err)
	}
	ln, err := listen()
	if err != nil {
		log.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	config.Addr = ln.Addr().String() // the Assistant's MCP address and the timeline editor's host come from here
	// Platforms send the browser back to the address registered with them, which the docs give as localhost.
	config.PublicURL = fmt.Sprintf("http://localhost:%d", port)

	ctx, stop := context.WithCancel(context.Background())
	served := make(chan struct{})
	go func() {
		if err := server.Serve(ctx, ln); err != nil {
			log.Print(err)
		}
		close(served)
	}()
	url := fmt.Sprintf("http://127.0.0.1:%d/", port)
	if err := waitUp(url); err != nil {
		log.Fatal(err)
	}

	win = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "37Studio",
		URL:       url,
		Width:     1440,
		Height:    900,
		MinWidth:  960,
		MinHeight: 640,
		// The chat's mic records here and the server transcribes it (/api/dictate): neither WebKitGTK nor WebView2 has
		// working speech recognition. Allow the microphone without a prompt; the camera is never used, so deny it,
		// which also keeps WebView2 from its default of granting everything. WebKitGTK 6.0 has media streams and
		// MediaRecorder on by default (enable-media-stream defaults to true), so no setting is needed for those.
		Permissions: map[application.PermissionType]application.Permission{
			application.PermissionMicrophone: application.PermissionAllow,
			application.PermissionCamera:     application.PermissionDeny,
		},
	})
	server.Desktop.OpenURL = app.Browser.OpenURL
	server.Desktop.SaveAs = func(name string) (string, error) {
		return app.Dialog.SaveFile().SetFilename(name).CanCreateDirectories(true).AttachToWindow(win).PromptForSingleSelection()
	}
	social.AfterSignIn = func(target string) {
		win.SetURL(target)
		win.Restore()
		win.Show()
		win.Focus()
	}
	app.OnShutdown(func() {
		stop()
		<-served
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// dataDir is where brands, projects and keys live: STUDIO37_DATA when set (an existing Docker data folder works as
// is), otherwise the platform's usual place for app data.
func dataDir() (string, error) {
	if d := config.Env("STUDIO37_DATA", ""); d != "" {
		return d, nil
	}
	if runtime.GOOS == "linux" {
		if x := os.Getenv("XDG_DATA_HOME"); x != "" {
			return filepath.Join(x, "37studio"), nil
		}
		home, err := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", "37studio"), err
	}
	d, err := os.UserConfigDir() // macOS: ~/Library/Application Support, Windows: %AppData%
	return filepath.Join(d, "37Studio"), err
}

// listen takes port 8080 when it's free, since that's the port in the redirect URIs people have registered with each
// platform; otherwise any free port, which still works for everything but those sign-ins.
func listen() (net.Listener, error) {
	if ln, err := net.Listen("tcp", "127.0.0.1:8080"); err == nil {
		return ln, nil
	}
	log.Print("port 8080 is in use; social sign-ins need their redirect URIs registered with the port used instead")
	return net.Listen("tcp", "127.0.0.1:0")
}

func waitUp(url string) error {
	for range 50 {
		if res, err := http.Get(url + "api/meta"); err == nil {
			res.Body.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("37Studio's server didn't start at %s", url)
}
