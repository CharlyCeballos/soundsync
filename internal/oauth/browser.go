package oauth

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenBrowser asks the OS to open u in the default browser. Callers should
// also print u so the user can open it by hand if this fails.
func OpenBrowser(u string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// rundll32 passes the URL untouched; "cmd /c start" would split it at '&'.
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	case "darwin":
		cmd = exec.Command("open", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start browser: %w", err)
	}
	// Do not wait: some openers block until the browser exits.
	go cmd.Wait()
	return nil
}
