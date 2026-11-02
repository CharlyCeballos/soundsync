# Soundsync GUI (developers)

The desktop app: a [Wails v3](https://v3.wails.io) window (Go + React) over the same engine as the CLI.
Users read the main [README](../README.md); this page is for building it.

- `main.go`, `host.go`, `webview2*.go`: the shell. All logic lives in `../internal/desktop`.
- `frontend/`: React + TypeScript + Vite. UI strings in `src/i18n` (English and Spanish, both required by the compiler).
- `build/`: icon, Windows manifest and version info, Linux desktop entry, MSI source, `package.sh` (release packages).
- `e2e.go`: a server-mode build with fake Spotify and YouTube APIs for the Playwright tests in `frontend/e2e`.

It is its own Go module (`replace ../`), so the CLI builds and tests without CGO, GTK or Node.

## Requirements

Go 1.26, Node 20.19+, `wails3` v3.1.0 (`go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0`).
Debian 13 also needs `build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev`.

## Tasks

```sh
wails3 task dev                 # live-reloading window
wails3 task build               # bin/soundsync-gui(.exe); VERSION=v1.2.3 stamps a version
wails3 task build:e2e           # bin/soundsync-gui-e2e for the Playwright tests
go test ./...                   # shell tests (the backend's are in ../internal/desktop)
```

The GUI reads `.env` from the user's config folder only (`%APPDATA%\soundsync` or `~/.config/soundsync`).
