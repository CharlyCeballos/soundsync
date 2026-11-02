//go:build production

package main

// devBrowserArgs adds nothing to a release build: no debugging port, ever.
func devBrowserArgs() []string { return nil }
