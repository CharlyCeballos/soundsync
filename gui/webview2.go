package main

import "github.com/CharlyCeballos/soundsync/internal/i18n"

// webView2DownloadURL is Microsoft's page for the Evergreen runtime.
const webView2DownloadURL = "https://developer.microsoft.com/microsoft-edge/webview2/"

// hasRuntime reports whether any registry "pv" value names a real WebView2
// version. Microsoft documents an empty value or 0.0.0.0 as "not installed".
func hasRuntime(versions []string) bool {
	for _, v := range versions {
		if v != "" && v != "0.0.0.0" {
			return true
		}
	}
	return false
}

// missingWebView2 is the message shown when Windows lacks the runtime.
func missingWebView2() (title, text string) {
	return "Soundsync", i18n.T("Soundsync needs the Microsoft Edge WebView2 Runtime, which is not installed on this computer.\n\nOpen Microsoft's download page now? After installing it, open Soundsync again.")
}
