// Package config loads Soundsync's local OAuth client configuration.
//
// Values come from process environment variables, optionally seeded from a
// .env file. Real environment variables always win over the file.
// Nothing here is ever written to disk or logged.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Environment variable names.
const (
	EnvSpotifyClientID    = "SPOTIFY_CLIENT_ID"
	EnvSpotifyRedirectURI = "SPOTIFY_REDIRECT_URI"
	EnvGoogleClientID     = "GOOGLE_CLIENT_ID"
	EnvGoogleClientSecret = "GOOGLE_CLIENT_SECRET"
	EnvGoogleRedirectURI  = "GOOGLE_REDIRECT_URI"
)

// Default loopback redirect URIs (must match what is registered on each platform).
const (
	DefaultSpotifyRedirectURI = "http://127.0.0.1:8888/callback"
	DefaultGoogleRedirectURI  = "http://127.0.0.1:8889/callback"
)

// Config holds the OAuth client settings for both platforms.
type Config struct {
	SpotifyClientID    string
	SpotifyRedirectURI *url.URL

	GoogleClientID string
	// GoogleClientSecret is required: Google's token endpoint rejects Desktop
	// clients without it even when PKCE is used ("client_secret is missing").
	// It is not confidential for Desktop clients, but it stays out of git.
	GoogleClientSecret string
	GoogleRedirectURI  *url.URL
}

// LoadDotEnv seeds the process environment from the file at path without
// overriding variables that are already set. A missing file is not an error;
// loaded reports whether the file existed and was read.
func LoadDotEnv(path string) (loaded bool, err error) {
	if _, statErr := os.Stat(path); statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read %s: %w", path, statErr)
	}
	if err := godotenv.Load(path); err != nil {
		return false, fmt.Errorf("parse %s: %w", path, err)
	}
	return true, nil
}

// FromEnv builds and validates a Config using getenv (usually os.Getenv).
// All validation problems are returned together.
func FromEnv(getenv func(string) string) (Config, error) {
	var cfg Config
	var errs []error

	cfg.SpotifyClientID = strings.TrimSpace(getenv(EnvSpotifyClientID))
	if cfg.SpotifyClientID == "" {
		errs = append(errs, fmt.Errorf("%s is required", EnvSpotifyClientID))
	}

	cfg.GoogleClientID = strings.TrimSpace(getenv(EnvGoogleClientID))
	if cfg.GoogleClientID == "" {
		errs = append(errs, fmt.Errorf("%s is required", EnvGoogleClientID))
	}

	cfg.GoogleClientSecret = strings.TrimSpace(getenv(EnvGoogleClientSecret))
	if cfg.GoogleClientSecret == "" {
		errs = append(errs, fmt.Errorf("%s is required (Google Desktop clients need it even with PKCE)", EnvGoogleClientSecret))
	}

	var err error
	if cfg.SpotifyRedirectURI, err = redirectURI(getenv, EnvSpotifyRedirectURI, DefaultSpotifyRedirectURI); err != nil {
		errs = append(errs, err)
	}
	if cfg.GoogleRedirectURI, err = redirectURI(getenv, EnvGoogleRedirectURI, DefaultGoogleRedirectURI); err != nil {
		errs = append(errs, err)
	}

	return cfg, errors.Join(errs...)
}

func redirectURI(getenv func(string) string, key, fallback string) (*url.URL, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		raw = fallback
	}
	u, err := ValidateLoopbackRedirect(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", key, err)
	}
	return u, nil
}

// ValidateLoopbackRedirect checks that raw is an http URI on a literal
// loopback IP with an explicit port, as required for PKCE loopback flows.
// "localhost" is rejected: Spotify no longer accepts it as a redirect host.
func ValidateLoopbackRedirect(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URI %q: %w", raw, err)
	}
	if u.Scheme != "http" {
		return nil, fmt.Errorf("%q must use http (loopback), got scheme %q", raw, u.Scheme)
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return nil, fmt.Errorf("%q must use a loopback IP such as 127.0.0.1", raw)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("%q must include an explicit port", raw)
	}
	if u.Path == "" || u.Path == "/" {
		return nil, fmt.Errorf("%q must include a callback path such as /callback", raw)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("%q must not include a query or fragment", raw)
	}
	return u, nil
}
