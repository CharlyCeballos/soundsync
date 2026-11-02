//go:build !production

package main

import (
	"os"
	"strconv"
)

// devBrowserArgs opens WebView2's remote debugging port when
// SOUNDSYNC_DEBUG_PORT is set, so tests can drive the real window over CDP
// (WebView2 ignores its own environment variable once Wails passes flags).
// Development builds only: release builds use the production tag.
func devBrowserArgs() []string {
	port, err := strconv.Atoi(os.Getenv("SOUNDSYNC_DEBUG_PORT"))
	if err != nil || port < 1024 || port > 65535 {
		return nil
	}
	return []string{"--remote-debugging-port=" + strconv.Itoa(port)}
}
