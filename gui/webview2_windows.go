package main

import (
	"github.com/CharlyCeballos/soundsync/internal/oauth"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// webView2Client is the WebView2 runtime's EdgeUpdate client ID.
const webView2Client = `Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`

// webView2Ready checks for the runtime before any window exists: without it
// Wails exits silently. When it is missing the user gets a native message
// that offers the download page.
func webView2Ready() bool {
	if hasRuntime(installedWebView2()) {
		return true
	}
	title, text := missingWebView2()
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(text)
	answer, _ := windows.MessageBox(0, m, t, windows.MB_YESNO|windows.MB_ICONWARNING)
	if answer == 6 { // IDYES
		_ = oauth.OpenBrowser(webView2DownloadURL)
	}
	return false
}

// installedWebView2 reads the "pv" values Microsoft documents for the
// per-machine (64- and 32-bit views) and per-user installs.
func installedWebView2() []string {
	places := []struct {
		root registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\` + webView2Client},
		{registry.LOCAL_MACHINE, `SOFTWARE\` + webView2Client},
		{registry.CURRENT_USER, `Software\` + webView2Client},
	}
	var versions []string
	for _, p := range places {
		k, err := registry.OpenKey(p.root, p.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		if v, _, err := k.GetStringValue("pv"); err == nil {
			versions = append(versions, v)
		}
		k.Close()
	}
	return versions
}
