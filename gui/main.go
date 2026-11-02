// Command soundsync-gui is Soundsync's desktop app: a window over the same
// engine as the CLI (internal/desktop). It holds no logic of its own: it
// wires the backend's services, events and dialogs to Wails.
package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/CharlyCeballos/soundsync/internal/config"
	"github.com/CharlyCeballos/soundsync/internal/desktop"
	"github.com/CharlyCeballos/soundsync/internal/i18n"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// version is set at build time: -ldflags "-X main.version=v0.2.0".
var version = "dev"

// appID names the app to the OS: single-instance lock and Linux desktop entry.
const appID = "io.github.charlyceballos.soundsync"

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		// Used by the package smoke test: runs without a display.
		fmt.Printf("soundsync-gui %s (%s, %s/%s)\n", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return
	}
	_ = i18n.Set(i18n.Default()) // cannot fail: Default only returns es or en
	if !webView2Ready() {
		os.Exit(1)
	}

	host := newHost()
	backend := desktop.New(host, backendOptions())
	app := application.New(appOptions(backend, host))
	host.attach(app, app.Window.NewWithOptions(windowOptions()))
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func appOptions(backend *desktop.App, host shell) application.Options {
	opts := application.Options{
		Name:        "Soundsync",
		Description: "Copy your Spotify playlists to YouTube",
		// Every exported method of these three becomes callable from the page.
		Services: []application.Service{
			application.NewService(backend.Settings),
			application.NewService(backend.Accounts),
			application.NewService(backend.Copy),
		},
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(assets), Middleware: assetMiddleware()},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: appID,
			// The login port is fixed, so a second window could not log in
			// anyway: bring the first one forward instead.
			OnSecondInstanceLaunch: func(application.SecondInstanceData) { host.Focus() },
		},
		// Revoke Google's grant however the app closes (Policies III.D.2).
		OnShutdown: backend.Shutdown,
		Linux:      application.LinuxOptions{ApplicationID: appID, ProgramName: "soundsync-gui"},
	}
	opts.Windows.AdditionalBrowserArgs = devBrowserArgs()
	if dir := config.Dir(); dir != "" {
		// Keep the webview's cache with the rest of Soundsync's files, so
		// removing that one folder removes everything.
		opts.Windows.WebviewUserDataPath = filepath.Join(dir, "webview")
	}
	return opts
}

func windowOptions() application.WebviewWindowOptions {
	return application.WebviewWindowOptions{
		Title:     "Soundsync",
		Width:     1180,
		Height:    800,
		MinWidth:  900,
		MinHeight: 640,
		// Matches the frontend's light background, so a dark-mode flash is the
		// only thing the user could see before the page paints.
		BackgroundColour:           application.NewRGB(247, 247, 248),
		URL:                        "/",
		DefaultContextMenuDisabled: version != "dev",
	}
}
