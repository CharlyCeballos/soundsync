package youtube

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/CharlyCeballos/soundsync/internal/oauth"

	"golang.org/x/oauth2"
)

// Endpoint is Google's OAuth 2.0 authorization server.
var Endpoint = oauth2.Endpoint{
	AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
	TokenURL: "https://oauth2.googleapis.com/token",
}

// RevokeURL is Google's token revocation endpoint.
var RevokeURL = "https://oauth2.googleapis.com/revoke"

// Scopes are the only permissions Soundsync requests (README, compliance.md).
var Scopes = []string{
	"https://www.googleapis.com/auth/youtube.force-ssl",
}

// NewFlow returns the PKCE login flow for a bring-your-own Google Desktop
// client. Google requires the client secret on the token exchange even with
// PKCE; Desktop secrets are not confidential.
func NewFlow(clientID, clientSecret string, redirectURI *url.URL) oauth.Flow {
	return oauth.Flow{
		Provider:     "YouTube",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     Endpoint,
		Scopes:       Scopes,
		RedirectURI:  redirectURI,
		// Let the user pick the Google account whose channel gets the playlists.
		AuthParams: map[string]string{"prompt": "select_account"},
	}
}

// Revoke invalidates tok at Google (YouTube API Policies III.D.2).
// Revoking the refresh token also revokes its access tokens.
// hc must be a plain client: the token goes in the form body.
func Revoke(ctx context.Context, hc *http.Client, tok *oauth2.Token) error {
	if tok == nil {
		return nil
	}
	value := tok.RefreshToken
	if value == "" {
		value = tok.AccessToken
	}
	if value == "" {
		return nil
	}

	form := url.Values{"token": {value}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, RevokeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("revoke Google token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// The body never echoes the token, but keep the error minimal anyway.
		return fmt.Errorf("revoke Google token: HTTP %d", resp.StatusCode)
	}
	return nil
}
