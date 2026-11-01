package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/CharlyCeballos/soundsync/internal/i18n"
)

const (
	latestReleaseURL = "https://api.github.com/repos/CharlyCeballos/soundsync/releases/latest"
	// releasePagePrefix guards what reaches the terminal: only links to
	// this repository's releases are printed.
	releasePagePrefix  = "https://github.com/CharlyCeballos/soundsync/releases/"
	updateCheckTimeout = 2 * time.Second
	maxReleaseBody     = 1 << 20
)

var versionRE = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)

// parseVersion reads a release tag such as v1.2.3. Anything else, including
// the "dev" of local builds and pre-releases, is not a version.
func parseVersion(s string) ([3]int, bool) {
	var v [3]int
	m := versionRE.FindStringSubmatch(s)
	if m == nil {
		return v, false
	}
	for i := range v {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return v, false // more digits than an int holds
		}
		v[i] = n
	}
	return v, true
}

func newerVersion(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

// checkForUpdate prints one line when GitHub has a newer published release
// (planV2 §9, YouTube Policies III.B.1). It sends nothing about the user,
// never downloads anything, and a failure costs one line, never the session.
// 404 means no published release (or a private repository): silent.
func checkForUpdate(ctx context.Context, out io.Writer, client *http.Client, url, current string) {
	have, ok := parseVersion(current)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateCheckTimeout)
	defer cancel()
	release, err := latestRelease(ctx, client, url, current)
	if err != nil {
		fmt.Fprintf(out, i18n.T("could not check for a newer version: %v\n"), err)
		return
	}
	if release == nil || !strings.HasPrefix(release.HTMLURL, releasePagePrefix) {
		return
	}
	if latest, ok := parseVersion(release.TagName); ok && newerVersion(latest, have) {
		fmt.Fprintf(out, i18n.T("A newer Soundsync is available: %s (you have %s): %s\n"), release.TagName, current, release.HTMLURL)
	}
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// latestRelease returns nil, nil when GitHub has no published release.
func latestRelease(ctx context.Context, client *http.Client, url, current string) (*githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "soundsync/"+current)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var release githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxReleaseBody)).Decode(&release); err != nil {
		return nil, fmt.Errorf("read GitHub answer: %w", err)
	}
	return &release, nil
}
