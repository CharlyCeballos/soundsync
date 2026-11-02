//go:build !e2e

package main

import (
	"github.com/CharlyCeballos/soundsync/internal/config"
	"github.com/CharlyCeballos/soundsync/internal/desktop"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// backendOptions are the real ones: browser logins, the user's config folder.
func backendOptions() desktop.Options {
	return desktop.Options{Version: version, ConfigDir: config.Dir()}
}

func newHost() shell { return &wailsHost{} }

// assetMiddleware adds nothing outside the e2e build.
func assetMiddleware() application.Middleware { return nil }
