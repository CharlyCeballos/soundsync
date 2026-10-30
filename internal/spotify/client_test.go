package spotify

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/me" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"id":"user1","display_name":"Charly","country":"MX"}`))
	}))
	defer srv.Close()

	u, err := NewClient(srv.Client()).WithBaseURL(srv.URL).Me(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != "user1" || u.DisplayName != "Charly" {
		t.Errorf("unexpected user %+v", u)
	}
}

func TestMeForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":{"status":403,"message":"User not registered in the Developer Dashboard"}}`))
	}))
	defer srv.Close()

	_, err := NewClient(srv.Client()).WithBaseURL(srv.URL).Me(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 403 || apiErr.Message == "" {
		t.Fatalf("want 403 APIError with message, got %v", err)
	}
}

func TestNewFlowUsesReadOnlyScopes(t *testing.T) {
	f := NewFlow("id", nil)
	if f.ClientSecret != "" {
		t.Error("Spotify PKCE flow must not carry a secret")
	}
	for _, s := range f.Scopes {
		if s != "playlist-read-private" && s != "playlist-read-collaborative" {
			t.Errorf("unexpected scope %q", s)
		}
	}
}
