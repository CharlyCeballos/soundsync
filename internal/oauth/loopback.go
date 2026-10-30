package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/oauth2"
)

// Flow describes one OAuth 2.0 authorization-code + PKCE login that receives
// the callback on a loopback HTTP server.
type Flow struct {
	// Provider is a human-readable name used in messages ("Spotify").
	Provider string
	ClientID string
	// ClientSecret is optional (public clients). Never logged.
	ClientSecret string
	Endpoint     oauth2.Endpoint
	Scopes       []string
	// RedirectURI must be http on a loopback IP with an explicit port.
	// Port 0 picks a free port (tests only; providers need a fixed one).
	RedirectURI *url.URL
	// AuthParams are extra query parameters for the authorization URL.
	AuthParams map[string]string
	// Logf, if set, receives diagnostic notes (never tokens or codes).
	Logf func(format string, args ...any)
}

// ProviderError is an error the provider returned on the callback
// (for example access_denied when the user cancels).
type ProviderError struct {
	Code        string
	Description string
}

func (e *ProviderError) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("provider returned %s: %s", e.Code, e.Description)
	}
	return "provider returned " + e.Code
}

type callbackResult struct {
	code string
	err  error
}

// Login runs the full flow: it starts the loopback listener, calls open with
// the authorization URL, waits for the callback, and exchanges the code.
// The returned token lives in memory only; callers must not persist it.
func (f Flow) Login(ctx context.Context, open func(authURL string) error) (*oauth2.Token, error) {
	if f.RedirectURI == nil {
		return nil, errors.New("redirect URI is required")
	}

	// Listen before opening the browser so the callback cannot race us.
	ln, err := net.Listen("tcp", f.RedirectURI.Host)
	if err != nil {
		return nil, fmt.Errorf("listen on %s (is another login running?): %w", f.RedirectURI.Host, err)
	}
	redirect := *f.RedirectURI
	redirect.Host = ln.Addr().String()

	cfg := f.config(redirect.String())
	verifier := oauth2.GenerateVerifier()
	state, err := randomState()
	if err != nil {
		ln.Close()
		return nil, err
	}

	results := make(chan callbackResult, 1)
	srv := &http.Server{
		Handler:           callbackHandler(redirect.Path, state, f.Provider, f.logf, results),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go srv.Serve(ln)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	opts := []oauth2.AuthCodeOption{oauth2.S256ChallengeOption(verifier)}
	for k, v := range f.AuthParams {
		opts = append(opts, oauth2.SetAuthURLParam(k, v))
	}
	if err := open(cfg.AuthCodeURL(state, opts...)); err != nil {
		return nil, fmt.Errorf("open browser: %w", err)
	}

	var res callbackResult
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("waiting for %s login: %w", f.Provider, ctx.Err())
	case res = <-results:
	}
	if res.err != nil {
		return nil, res.err
	}

	tok, err := cfg.Exchange(ctx, res.code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("exchange %s authorization code: %w", f.Provider, err)
	}
	return tok, nil
}

func (f Flow) logf(format string, args ...any) {
	if f.Logf != nil {
		f.Logf(format, args...)
	}
}

func (f Flow) config(redirectURL string) *oauth2.Config {
	ep := f.Endpoint
	// Send client_id (and the secret, if any) in the form body: public PKCE
	// clients have no meaningful Basic auth, and auto-detection would waste
	// a request on providers that reject the header.
	ep.AuthStyle = oauth2.AuthStyleInParams
	return &oauth2.Config{
		ClientID:     f.ClientID,
		ClientSecret: f.ClientSecret,
		Endpoint:     ep,
		RedirectURL:  redirectURL,
		Scopes:       f.Scopes,
	}
}

// callbackHandler reports the first callback that carries this login's state.
// Requests with a missing or foreign state (stale tabs, reloads, omnibox
// prefetches, forged links) are rejected but do not end the login: otherwise
// any stray request to the loopback port could abort it.
func callbackHandler(path, state, provider string, logf func(string, ...any), results chan<- callbackResult) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")

		q := r.URL.Query()
		if got := q.Get("state"); got != state {
			reason := "a different login"
			if got == "" {
				reason = "no state parameter"
			}
			logf("Ignored a request to the callback URL (%s); still waiting for the %s redirect.", reason, provider)
			w.WriteHeader(http.StatusBadRequest)
			writePage(w, provider+" login: link not part of this session",
				"This page does not belong to the login in progress. Finish the login in the tab that Soundsync opened.")
			return
		}

		var res callbackResult
		switch {
		case q.Get("error") != "":
			res.err = &ProviderError{Code: q.Get("error"), Description: q.Get("error_description")}
		case q.Get("code") == "":
			res.err = errors.New("callback without authorization code")
		default:
			res.code = q.Get("code")
		}

		if res.err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writePage(w, provider+" login failed", "Return to the terminal for details.")
		} else {
			writePage(w, provider+" connected", "You can close this tab and return to the terminal.")
		}

		// Only the first valid callback counts; later ones are ignored.
		select {
		case results <- res:
		default:
		}
	})
	return mux
}

func writePage(w http.ResponseWriter, heading, body string) {
	title := "Soundsync: " + heading
	fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>%[1]s</title></head>
<body style="font-family:system-ui,sans-serif;max-width:32rem;margin:4rem auto;padding:0 1rem">
<h1 style="font-size:1.25rem">%[1]s</h1><p>%[2]s</p></body></html>`,
		html.EscapeString(title), html.EscapeString(body))
}

func randomState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Client returns an HTTP client that authorizes requests with tok and
// refreshes it in memory when it expires.
func (f Flow) Client(ctx context.Context, tok *oauth2.Token) *http.Client {
	redirect := ""
	if f.RedirectURI != nil {
		redirect = f.RedirectURI.String()
	}
	return f.config(redirect).Client(ctx, tok)
}
