module github.com/CharlyCeballos/soundsync/gui

go 1.26.0

require (
	github.com/CharlyCeballos/soundsync v0.0.0
	github.com/wailsapp/wails/v3 v3.1.0
	golang.org/x/oauth2 v0.37.0
	golang.org/x/sys v0.48.0
)

require (
	github.com/coder/websocket v1.8.14 // indirect
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	golang.org/x/term v0.46.0 // indirect
)

// The GUI ships from this repository only: it always builds against the
// engine next to it.
replace github.com/CharlyCeballos/soundsync => ../
