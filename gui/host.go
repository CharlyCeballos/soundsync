package main

import (
	"path/filepath"
	"strings"

	"github.com/CharlyCeballos/soundsync/internal/desktop"
	"github.com/CharlyCeballos/soundsync/internal/oauth"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// shell is the backend's Host plus the wiring main needs once Wails exists.
type shell interface {
	desktop.Host
	attach(app *application.App, window *application.WebviewWindow)
}

// wailsHost is what the backend needs from Wails: events, the save dialog,
// opening links and folders, and the window's focus.
type wailsHost struct {
	app    *application.App
	window *application.WebviewWindow
}

func (h *wailsHost) attach(app *application.App, window *application.WebviewWindow) {
	h.app, h.window = app, window
}

func (h *wailsHost) Emit(event string, data any) { h.app.Event.Emit(event, data) }

// OpenExternal uses the same opener as the CLI (xdg-open, rundll32): it
// takes URLs and folders alike. The backend decides what may be opened.
func (h *wailsHost) OpenExternal(target string) error { return oauth.OpenBrowser(target) }

// SaveFile offers the suggested name, filtered to its extension (Windows
// then adds the extension when the user types a name without one).
func (h *wailsHost) SaveFile(title, defaultName string) (string, error) {
	ext := strings.TrimPrefix(filepath.Ext(defaultName), ".")
	return h.app.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:                title,
		Filename:             defaultName,
		CanCreateDirectories: true,
		Filters:              []application.FileFilter{{DisplayName: strings.ToUpper(ext), Pattern: "*." + ext}},
		Window:               h.window,
	}).PromptForSingleSelection()
}

func (h *wailsHost) Focus() {
	if h.window == nil {
		return
	}
	h.window.Restore()
	h.window.Show()
	h.window.Focus()
}
