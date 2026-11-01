package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestParseVersion(t *testing.T) {
	for in, want := range map[string]bool{
		"v1.2.3": true, "v0.10.0": true,
		"1.2.3": false, "v1.2": false, "v1.2.3-rc.1": false, "dev": false, "": false,
		"v99999999999999999999.0.0": false,
	} {
		if _, ok := parseVersion(in); ok != want {
			t.Errorf("parseVersion(%q) ok = %v, want %v", in, ok, want)
		}
	}
	a, _ := parseVersion("v0.10.0")
	b, _ := parseVersion("v0.9.9")
	if !newerVersion(a, b) || newerVersion(b, a) || newerVersion(a, a) {
		t.Error("v0.10.0 must be newer than v0.9.9, and a version is not newer than itself")
	}
}

func TestCheckForUpdate(t *testing.T) {
	const page = releasePagePrefix + "tag/v0.2.0"
	cases := []struct {
		name    string
		current string
		status  int
		body    string
		want    string // "" = prints nothing
	}{
		{"newer", "v0.1.0", 200, `{"tag_name":"v0.2.0","html_url":"` + page + `"}`, "A newer Soundsync is available: v0.2.0 (you have v0.1.0): " + page},
		{"same", "v0.2.0", 200, `{"tag_name":"v0.2.0","html_url":"` + page + `"}`, ""},
		{"older", "v0.3.0", 200, `{"tag_name":"v0.2.0","html_url":"` + page + `"}`, ""},
		{"no release or private repo", "v0.1.0", 404, `{"message":"Not Found"}`, ""},
		{"server error", "v0.1.0", 500, ``, "could not check for a newer version: GitHub answered 500"},
		{"broken json", "v0.1.0", 200, `{"tag_name":`, "could not check for a newer version: read GitHub answer"},
		{"foreign link", "v0.1.0", 200, `{"tag_name":"v0.2.0","html_url":"https://evil.example/soundsync"}`, ""},
		{"tag not semver", "v0.1.0", 200, `{"tag_name":"v0.2.0\u001b[2J","html_url":"` + page + `"}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if ua := r.Header.Get("User-Agent"); ua != "soundsync/"+tc.current {
					t.Errorf("User-Agent = %q", ua)
				}
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			var out bytes.Buffer
			checkForUpdate(context.Background(), &out, srv.Client(), srv.URL, tc.current)
			got := out.String()
			if tc.want == "" && got != "" {
				t.Errorf("want silence, got %q", got)
			}
			if tc.want != "" && (!strings.Contains(got, tc.want) || strings.Count(got, "\n") != 1) {
				t.Errorf("want one line with %q, got %q", tc.want, got)
			}
		})
	}
}

func TestCheckForUpdateSkipsDevBuilds(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer srv.Close()

	var out bytes.Buffer
	checkForUpdate(context.Background(), &out, srv.Client(), srv.URL, "dev")
	if calls.Load() != 0 || out.Len() != 0 {
		t.Errorf("dev build must not ask GitHub: %d calls, output %q", calls.Load(), out.String())
	}
}
