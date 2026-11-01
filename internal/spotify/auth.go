package spotify

import (
	"net/url"

	"github.com/CharlyCeballos/soundsync/internal/oauth"

	"golang.org/x/oauth2"
)

// Endpoint is Spotify's OAuth 2.0 authorization server.
var Endpoint = oauth2.Endpoint{
	AuthURL:  "https://accounts.spotify.com/authorize",
	TokenURL: "https://accounts.spotify.com/api/token",
}

// Scopes are the only permissions Soundsync requests (README, compliance.md).
var Scopes = []string{
	"playlist-read-private",
	"playlist-read-collaborative",
}

// NewFlow returns the PKCE login flow for a bring-your-own Spotify app.
// Spotify PKCE clients have no secret.
func NewFlow(clientID string, redirectURI *url.URL) oauth.Flow {
	return oauth.Flow{
		Provider:    "Spotify",
		ClientID:    clientID,
		Endpoint:    Endpoint,
		Scopes:      Scopes,
		RedirectURI: redirectURI,
	}
}
