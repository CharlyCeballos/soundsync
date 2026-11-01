// Command soundsync copies Spotify playlists to YouTube.
package main

import (
	"os"

	"github.com/CharlyCeballos/soundsync/internal/i18n"
)

// version is set at build time: -ldflags "-X main.version=v0.1.0".
var version = "dev"

func main() {
	_ = i18n.Set(i18n.Default()) // cannot fail: Default only returns es or en
	a := newApp()
	err := newRootCmd(a).Execute()
	// Revoke Google access however the command ended (Policies III.D.2).
	a.close(os.Stdout)
	if err != nil {
		os.Exit(1)
	}
}
