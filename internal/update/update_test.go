package update

import (
	"net/http"
	"net/http/httptest"
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

func TestCheck(t *testing.T) {
	const page = ReleasePagePrefix + "tag/v0.2.0"
	cases := []struct {
		name, current, body string
		status              int
		wantTag             string
		wantErr             bool
	}{
		{"newer", "v0.1.0", `{"tag_name":"v0.2.0","html_url":"` + page + `"}`, 200, "v0.2.0", false},
		{"same", "v0.2.0", `{"tag_name":"v0.2.0","html_url":"` + page + `"}`, 200, "", false},
		{"not found", "v0.1.0", `{"message":"Not Found"}`, 404, "", false},
		{"server error", "v0.1.0", ``, 500, "", true},
		{"foreign link", "v0.1.0", `{"tag_name":"v0.2.0","html_url":"https://evil.example/x"}`, 200, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			rel, err := Check(t.Context(), srv.Client(), srv.URL, tc.current)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error %v", err, tc.wantErr)
			}
			got := ""
			if rel != nil {
				got = rel.Tag
				if rel.URL != page {
					t.Errorf("URL = %q", rel.URL)
				}
			}
			if got != tc.wantTag {
				t.Errorf("tag = %q, want %q", got, tc.wantTag)
			}
		})
	}
}
