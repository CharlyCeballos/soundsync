package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// fakeProvider is a minimal authorization server that enforces PKCE S256.
type fakeProvider struct {
	t      *testing.T
	srv    *httptest.Server
	mu     sync.Mutex
	codes  map[string]string // code -> code_challenge
	gotTok url.Values        // last token request form
}

func newFakeProvider(t *testing.T) *fakeProvider {
	p := &fakeProvider{t: t, codes: map[string]string{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/authorize", p.authorize)
	mux.HandleFunc("/token", p.token)
	p.srv = httptest.NewServer(mux)
	t.Cleanup(p.srv.Close)
	return p
}

func (p *fakeProvider) endpoint() oauth2.Endpoint {
	return oauth2.Endpoint{AuthURL: p.srv.URL + "/authorize", TokenURL: p.srv.URL + "/token"}
}

func (p *fakeProvider) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		http.Error(w, "PKCE required", http.StatusBadRequest)
		return
	}
	code := "code-123"
	p.mu.Lock()
	p.codes[code] = q.Get("code_challenge")
	p.mu.Unlock()

	cb, _ := url.Parse(q.Get("redirect_uri"))
	cq := cb.Query()
	cq.Set("code", code)
	cq.Set("state", q.Get("state"))
	cb.RawQuery = cq.Encode()
	http.Redirect(w, r, cb.String(), http.StatusFound)
}

func (p *fakeProvider) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p.mu.Lock()
	p.gotTok = r.PostForm
	challenge, ok := p.codes[r.PostForm.Get("code")]
	p.mu.Unlock()

	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if !ok || base64.RawURLEncoding.EncodeToString(sum[:]) != challenge {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid_grant"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"access_token":  "access-xyz",
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": "refresh-xyz",
	})
}

func testFlow(p *fakeProvider) Flow {
	return Flow{
		Provider:    "Fake",
		ClientID:    "client-1",
		Endpoint:    p.endpoint(),
		Scopes:      []string{"a", "b"},
		RedirectURI: &url.URL{Scheme: "http", Host: "127.0.0.1:0", Path: "/callback"},
		AuthParams:  map[string]string{"show_dialog": "true"},
	}
}

// browse simulates the user's browser: follow the auth URL to the callback.
func browse(t *testing.T) func(string) error {
	return func(authURL string) error {
		go func() {
			resp, err := http.Get(authURL)
			if err != nil {
				t.Errorf("browser GET: %v", err)
				return
			}
			resp.Body.Close()
		}()
		return nil
	}
}

func ctxTimeout(t *testing.T, d time.Duration) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx
}

func TestLoginSuccess(t *testing.T) {
	p := newFakeProvider(t)
	var seenAuthURL string
	open := func(u string) error {
		seenAuthURL = u
		return browse(t)(u)
	}

	tok, err := testFlow(p).Login(ctxTimeout(t, 5*time.Second), open)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tok.AccessToken != "access-xyz" || tok.RefreshToken != "refresh-xyz" {
		t.Errorf("unexpected token: %+v", tok)
	}

	au, _ := url.Parse(seenAuthURL)
	q := au.Query()
	if q.Get("scope") != "a b" || q.Get("show_dialog") != "true" || q.Get("client_id") != "client-1" {
		t.Errorf("auth URL params wrong: %s", seenAuthURL)
	}
	if !strings.HasPrefix(q.Get("redirect_uri"), "http://127.0.0.1:") || strings.HasSuffix(q.Get("redirect_uri"), ":0/callback") {
		t.Errorf("redirect_uri should carry the real port: %s", q.Get("redirect_uri"))
	}

	p.mu.Lock()
	form := p.gotTok
	p.mu.Unlock()
	if form.Get("client_id") != "client-1" {
		t.Errorf("client_id not sent in body: %v", form)
	}
	if _, has := form["client_secret"]; has {
		t.Errorf("client_secret sent although empty: %v", form)
	}
	if form.Get("redirect_uri") != q.Get("redirect_uri") {
		t.Errorf("token redirect_uri %q != auth redirect_uri %q", form.Get("redirect_uri"), q.Get("redirect_uri"))
	}
}

func TestLoginSendsSecretWhenSet(t *testing.T) {
	p := newFakeProvider(t)
	f := testFlow(p)
	f.ClientSecret = "shh"
	if _, err := f.Login(ctxTimeout(t, 5*time.Second), browse(t)); err != nil {
		t.Fatalf("Login: %v", err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.gotTok.Get("client_secret") != "shh" {
		t.Errorf("client_secret not sent: %v", p.gotTok)
	}
}

// hitCallback opens the callback directly with custom query values.
func hitCallback(t *testing.T, mutate func(q url.Values, state string)) func(string) error {
	return func(authURL string) error {
		au, _ := url.Parse(authURL)
		cb, _ := url.Parse(au.Query().Get("redirect_uri"))
		q := url.Values{}
		mutate(q, au.Query().Get("state"))
		cb.RawQuery = q.Encode()
		go func() {
			resp, err := http.Get(cb.String())
			if err == nil {
				resp.Body.Close()
			}
		}()
		return nil
	}
}

// TestLoginIgnoresStrayCallbacks reproduces the first real login: a bare
// request to /callback (no state) arrived before Spotify's redirect.
func TestLoginIgnoresStrayCallbacks(t *testing.T) {
	p := newFakeProvider(t)
	var mu sync.Mutex
	var notes []string
	f := testFlow(p)
	f.Logf = func(format string, args ...any) {
		mu.Lock()
		notes = append(notes, fmt.Sprintf(format, args...))
		mu.Unlock()
	}

	open := func(authURL string) error {
		au, _ := url.Parse(authURL)
		cb := au.Query().Get("redirect_uri")
		go func() {
			for _, stray := range []string{cb, cb + "?code=evil&state=forged"} {
				resp, err := http.Get(stray)
				if err != nil {
					t.Errorf("stray GET: %v", err)
					return
				}
				if resp.StatusCode != http.StatusBadRequest {
					t.Errorf("stray %s: status %d, want 400", stray, resp.StatusCode)
				}
				resp.Body.Close()
			}
			// Then the real browser redirect arrives.
			resp, err := http.Get(authURL)
			if err != nil {
				t.Errorf("browser GET: %v", err)
				return
			}
			resp.Body.Close()
		}()
		return nil
	}

	tok, err := f.Login(ctxTimeout(t, 5*time.Second), open)
	if err != nil {
		t.Fatalf("Login should survive stray callbacks: %v", err)
	}
	if tok.AccessToken != "access-xyz" {
		t.Errorf("unexpected token %+v", tok)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(notes) != 2 || !strings.Contains(notes[0], "no state") || !strings.Contains(notes[1], "different login") {
		t.Errorf("unexpected notes: %q", notes)
	}
	for _, n := range notes {
		if strings.Contains(n, "evil") || strings.Contains(n, "forged") {
			t.Errorf("note leaks request values: %q", n)
		}
	}
}

func TestLoginOnlyForeignStateTimesOut(t *testing.T) {
	p := newFakeProvider(t)
	open := hitCallback(t, func(q url.Values, _ string) {
		q.Set("code", "code-123")
		q.Set("state", "forged")
	})
	_, err := testFlow(p).Login(ctxTimeout(t, 300*time.Millisecond), open)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("foreign state must not complete or abort the login, got %v", err)
	}
}

func TestLoginProviderError(t *testing.T) {
	p := newFakeProvider(t)
	open := hitCallback(t, func(q url.Values, state string) {
		q.Set("error", "access_denied")
		q.Set("state", state)
	})
	_, err := testFlow(p).Login(ctxTimeout(t, 5*time.Second), open)
	var pe *ProviderError
	if !errors.As(err, &pe) || pe.Code != "access_denied" {
		t.Fatalf("want ProviderError access_denied, got %v", err)
	}
}

func TestLoginTimeout(t *testing.T) {
	p := newFakeProvider(t)
	noop := func(string) error { return nil }
	_, err := testFlow(p).Login(ctxTimeout(t, 100*time.Millisecond), noop)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want deadline exceeded, got %v", err)
	}
}

func TestLoginPortBusy(t *testing.T) {
	p := newFakeProvider(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	f := testFlow(p)
	f.RedirectURI = &url.URL{Scheme: "http", Host: ln.Addr().String(), Path: "/callback"}
	_, err = f.Login(ctxTimeout(t, 2*time.Second), func(string) error {
		t.Error("browser must not open when the port is busy")
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "listen on") {
		t.Fatalf("want listen error, got %v", err)
	}
}

func TestOpenErrorStopsLogin(t *testing.T) {
	p := newFakeProvider(t)
	_, err := testFlow(p).Login(ctxTimeout(t, 2*time.Second), func(string) error {
		return errors.New("no browser")
	})
	if err == nil || !strings.Contains(err.Error(), "no browser") {
		t.Fatalf("want open error, got %v", err)
	}
}
