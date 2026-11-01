package youtube

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestParseISODuration(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"PT3M45S": 3*time.Minute + 45*time.Second,
		"PT45S":   45 * time.Second,
		"PT1H2M":  time.Hour + 2*time.Minute,
		"P1DT2S":  24*time.Hour + 2*time.Second,
		"P0D":     0,
	} {
		if got, err := parseISODuration(in); err != nil || got != want {
			t.Errorf("parseISODuration(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "P", "PT", "3M45S", "PT3X"} {
		if _, err := parseISODuration(bad); err == nil {
			t.Errorf("parseISODuration(%q) must fail", bad)
		}
	}
}

func TestVideoLengthsBatchesOf50(t *testing.T) {
	var calls int
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/videos" || q.Get("part") != "contentDetails" {
			http.Error(w, "bad request "+r.URL.String(), http.StatusBadRequest)
			return
		}
		calls++
		var items []string
		for _, id := range strings.Split(q.Get("id"), ",") {
			if id == "gone" {
				continue // deleted videos are not returned
			}
			items = append(items, fmt.Sprintf(`{"id":%q,"contentDetails":{"duration":"PT3M1S"}}`, id))
		}
		w.Write([]byte(`{"items":[` + strings.Join(items, ",") + `]}`))
	})
	ids := []string{"gone"}
	for i := range 60 {
		ids = append(ids, fmt.Sprintf("v%d", i))
	}
	c := NewClient(srv.Client()).WithBaseURL(srv.URL)
	got, err := c.VideoLengths(context.Background(), ids)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 60 || got["v59"] != 3*time.Minute+time.Second || calls != 2 || c.UnitsUsed() != 2 {
		t.Errorf("len=%d v59=%v calls=%d units=%d", len(got), got["v59"], calls, c.UnitsUsed())
	}
	if _, ok := got["gone"]; ok {
		t.Error("a video YouTube did not return must be absent")
	}
}
