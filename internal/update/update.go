// Package update tells whether GitHub has a newer published Soundsync
// (planV2 §9, YouTube API Policies III.B.1). It sends nothing about the
// user and never downloads anything.
package update

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
)

const (
	// LatestURL is GitHub's latest published release of Soundsync.
	LatestURL = "https://api.github.com/repos/CharlyCeballos/soundsync/releases/latest"
	// ReleasePagePrefix guards what reaches the user: only links to this
	// repository's releases are shown.
	ReleasePagePrefix = "https://github.com/CharlyCeballos/soundsync/releases/"
	checkTimeout      = 2 * time.Second
	maxReleaseBody    = 1 << 20
)

// Release is a published release newer than the running build.
type Release struct {
	Tag string // e.g. v0.2.0
	URL string // its page on GitHub
}

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

// Check returns the newer release, or nil when there is none. Builds that
// are not a release version (dev) never ask. 404 means no published
// release (or a private repository) and is not an error. Only a release
// whose page is in this repository and whose tag is a version counts, so
// nothing arbitrary from the network reaches the user.
func Check(ctx context.Context, client *http.Client, url, current string) (*Release, error) {
	have, ok := parseVersion(current)
	if !ok {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	rel, err := latest(ctx, client, url, current)
	if err != nil || rel == nil || !strings.HasPrefix(rel.HTMLURL, ReleasePagePrefix) {
		return nil, err
	}
	if v, ok := parseVersion(rel.TagName); ok && newerVersion(v, have) {
		return &Release{Tag: rel.TagName, URL: rel.HTMLURL}, nil
	}
	return nil, nil
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// latest returns nil, nil when GitHub has no published release.
func latest(ctx context.Context, client *http.Client, url, current string) (*githubRelease, error) {
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
	var rel githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxReleaseBody)).Decode(&rel); err != nil {
		return nil, fmt.Errorf("read GitHub answer: %w", err)
	}
	return &rel, nil
}
