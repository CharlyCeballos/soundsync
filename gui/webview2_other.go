//go:build !windows

package main

// webView2Ready is Windows-only: Linux links WebKitGTK through the package's
// dependencies.
func webView2Ready() bool { return true }
