package main

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/CharlyCeballos/soundsync/internal/i18n"
	"github.com/CharlyCeballos/soundsync/internal/update"
)

// checkForUpdate prints one line when GitHub has a newer published release
// (planV2 §9, YouTube Policies III.B.1). A failure costs one line, never the
// session.
func checkForUpdate(ctx context.Context, out io.Writer, client *http.Client, url, current string) {
	rel, err := update.Check(ctx, client, url, current)
	if err != nil {
		fmt.Fprintf(out, i18n.T("could not check for a newer version: %v\n"), err)
		return
	}
	if rel != nil {
		fmt.Fprintf(out, i18n.T("A newer Soundsync is available: %s (you have %s): %s\n"), rel.Tag, current, rel.URL)
	}
}
