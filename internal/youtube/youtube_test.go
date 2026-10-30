package youtube

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"
)

func serve(t *testing.T, h http.HandlerFunc) *httptest.Server {
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func TestMyChannel(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/channels" || q.Get("mine") != "true" || q.Get("part") != "snippet" {
			http.Error(w, "bad request "+r.URL.String(), http.StatusBadRequest)
			return
		}
		w.Write([]byte(`{"items":[{"id":"UC123","snippet":{"title":"Charly"}}]}`))
	})
	ch, err := NewClient(srv.Client()).WithBaseURL(srv.URL).MyChannel(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ch.ID != "UC123" || ch.Title != "Charly" {
		t.Errorf("unexpected channel %+v", ch)
	}
}

func TestMyChannelNone(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items":[]}`))
	})
	_, err := NewClient(srv.Client()).WithBaseURL(srv.URL).MyChannel(context.Background())
	if !errors.Is(err, ErrNoChannel) {
		t.Fatalf("want ErrNoChannel, got %v", err)
	}
}

func TestAPIErrorReason(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":{"code":403,"message":"YouTube Data API v3 has not been used in project 1 before or it is disabled.","errors":[{"reason":"accessNotConfigured"}]}}`))
	})
	_, err := NewClient(srv.Client()).WithBaseURL(srv.URL).MyChannel(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 403 || apiErr.Reason != "accessNotConfigured" {
		t.Fatalf("want 403 accessNotConfigured, got %v", err)
	}
}

func TestRevokePrefersRefreshToken(t *testing.T) {
	var got string
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		r.ParseForm()
		got = r.PostForm.Get("token")
	})
	old := RevokeURL
	RevokeURL = srv.URL
	t.Cleanup(func() { RevokeURL = old })

	tok := &oauth2.Token{AccessToken: "access", RefreshToken: "refresh"}
	if err := Revoke(context.Background(), srv.Client(), tok); err != nil {
		t.Fatal(err)
	}
	if got != "refresh" {
		t.Errorf("revoked %q, want refresh token", got)
	}

	if err := Revoke(context.Background(), srv.Client(), &oauth2.Token{AccessToken: "access"}); err != nil {
		t.Fatal(err)
	}
	if got != "access" {
		t.Errorf("revoked %q, want access token when no refresh token", got)
	}
}

func TestRevokeErrorHidesToken(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"invalid_token"}`, http.StatusBadRequest)
	})
	old := RevokeURL
	RevokeURL = srv.URL
	t.Cleanup(func() { RevokeURL = old })

	err := Revoke(context.Background(), srv.Client(), &oauth2.Token{AccessToken: "secret-access"})
	if err == nil {
		t.Fatal("want error")
	}
	if got := err.Error(); got != "revoke Google token: HTTP 400" {
		t.Errorf("unexpected error text %q", got)
	}
}

func TestNewFlow(t *testing.T) {
	f := NewFlow("id", "secret", nil)
	if f.ClientSecret != "secret" {
		t.Error("Google flow must send the client secret")
	}
	if len(f.Scopes) != 1 || f.Scopes[0] != "https://www.googleapis.com/auth/youtube.force-ssl" {
		t.Errorf("unexpected scopes %v", f.Scopes)
	}
	if f.AuthParams["prompt"] != "select_account" {
		t.Errorf("want account picker, got %v", f.AuthParams)
	}
}
