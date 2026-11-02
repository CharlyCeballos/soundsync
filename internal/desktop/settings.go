package desktop

import (
	"context"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"strings"

	"github.com/CharlyCeballos/soundsync/internal/config"
	"github.com/CharlyCeballos/soundsync/internal/consent"
	"github.com/CharlyCeballos/soundsync/internal/i18n"
	"github.com/CharlyCeballos/soundsync/internal/update"
)

// Settings is the app's setup: language, privacy consent, the .env file and
// links out of the app.
type Settings struct{ st *state }

// Status is what the frontend needs to pick the first screen.
type Status struct {
	Version        string      `json:"version"`
	Language       string      `json:"language"`
	PolicyAccepted bool        `json:"policyAccepted"`
	PolicyVersion  string      `json:"policyVersion"`
	Config         ConfigView  `json:"config"`
	Spotify        *Account    `json:"spotify"` // nil until connected
	YouTube        *Channel    `json:"youtube"` // nil until connected
	Links          ConsentURLs `json:"links"`
}

// ConfigView describes the .env without ever showing a secret.
type ConfigView struct {
	Folder   string        `json:"folder"`
	File     string        `json:"file"`
	Found    bool          `json:"found"`
	OK       bool          `json:"ok"`
	Fields   []ConfigField `json:"fields"`
	Problems []string      `json:"problems"`
}

// ConfigField is one setting: "set", "missing" or "invalid".
type ConfigField struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// ConsentURLs are the links the privacy notice and the footer show.
type ConsentURLs struct {
	Policy            string `json:"policy"`
	YouTubeTerms      string `json:"youtubeTerms"`
	GooglePrivacy     string `json:"googlePrivacy"`
	GooglePermissions string `json:"googlePermissions"`
	SetupGuide        string `json:"setupGuide"`
}

// Release is a newer Soundsync on GitHub.
type Release struct {
	Tag string `json:"tag"`
	URL string `json:"url"`
}

// Status reads the .env again on every call, so the frontend can ask after
// the user edits it.
func (s *Settings) Status() Status {
	st := s.st
	st.mu.Lock()
	accepted := st.policyAccepted
	var sp *Account
	if st.sp != nil {
		sp = accountOf(st.sp)
	}
	var yt *Channel
	if st.yt != nil {
		yt = channelOf(st.yt)
	}
	st.mu.Unlock()
	return Status{
		Version:        st.opts.Version,
		Language:       i18n.Current(),
		PolicyAccepted: accepted || consent.Accepted(st.opts.ConfigDir),
		PolicyVersion:  consent.Version,
		Config:         st.configView(),
		Spotify:        sp,
		YouTube:        yt,
		Links: ConsentURLs{
			Policy:            consent.PolicyURL,
			YouTubeTerms:      consent.YouTubeTermsURL,
			GooglePrivacy:     consent.GooglePrivacyURL,
			GooglePermissions: consent.GooglePermissionsURL,
			SetupGuide:        config.SetupGuideURL,
		},
	}
}

func (st *state) configView() ConfigView {
	v := ConfigView{Folder: st.opts.ConfigDir, File: st.envFile()}
	if v.File == "" {
		v.Problems = []string{ErrNoConfigDir.Error()}
		return v
	}
	f, err := config.ReadFile(v.File)
	if err != nil {
		v.Problems = []string{err.Error()}
		return v
	}
	v.Found = f.Loaded
	cfg, err := config.FromEnv(f.Getenv)
	v.OK = err == nil
	v.Fields = []ConfigField{
		{config.EnvSpotifyClientID, presence(cfg.SpotifyClientID)},
		{config.EnvSpotifyRedirectURI, validity(cfg.SpotifyRedirectURI)},
		{config.EnvGoogleClientID, presence(cfg.GoogleClientID)},
		{config.EnvGoogleClientSecret, presence(cfg.GoogleClientSecret)},
		{config.EnvGoogleRedirectURI, validity(cfg.GoogleRedirectURI)},
	}
	for _, e := range unwrapAll(err) {
		v.Problems = append(v.Problems, e.Error())
	}
	return v
}

func presence(v string) string {
	if v == "" {
		return "missing"
	}
	return "set"
}

func validity(u *url.URL) string {
	if u == nil {
		return "invalid"
	}
	return "set"
}

func unwrapAll(err error) []error {
	if err == nil {
		return nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		return joined.Unwrap()
	}
	return []error{err}
}

// config loads a valid configuration for a login.
func (st *state) config() (config.Config, error) {
	if st.envFile() == "" {
		return config.Config{}, ErrNoConfigDir
	}
	f, err := config.ReadFile(st.envFile())
	if err != nil {
		return config.Config{}, err
	}
	return config.FromEnv(f.Getenv)
}

// SetLanguage switches the language of the backend's messages ("es" or "en").
func (s *Settings) SetLanguage(lang string) error { return i18n.Set(lang) }

// AcceptPolicy records the user's consent to the privacy policy and the
// YouTube Terms of Service (Policies III.A.1–A.2). Without a config folder
// the consent lasts until the app closes.
func (s *Settings) AcceptPolicy() error {
	st := s.st
	err := consent.Remember(st.opts.ConfigDir)
	if err != nil && !errors.Is(err, consent.ErrNoConfigDir) {
		return err
	}
	st.mu.Lock()
	st.policyAccepted = true
	st.mu.Unlock()
	return nil
}

func (st *state) policyOK() bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.policyAccepted || consent.Accepted(st.opts.ConfigDir)
}

// allowedLinks are the only pages the app opens: the frontend never
// navigates its own window, and never opens anything else. Every prefix
// ends with "/", so open.spotify.com.evil.example or soundsync-evil never
// match.
var allowedLinks = []string{
	"https://open.spotify.com/",
	"https://www.youtube.com/",
	"https://music.youtube.com/",
	"https://myaccount.google.com/",
	"https://security.google.com/",
	"https://policies.google.com/",
	"https://github.com/CharlyCeballos/soundsync/",
}

// OpenURL opens a link in the user's browser.
func (s *Settings) OpenURL(link string) error {
	u, err := url.Parse(link)
	if err != nil || u.User != nil || !allowed(u.String()) {
		return ErrURLNotAllowed
	}
	return s.st.host.OpenExternal(u.String())
}

func allowed(link string) bool {
	for _, prefix := range allowedLinks {
		if strings.HasPrefix(link, prefix) {
			return true
		}
	}
	return false
}

// OpenConfigFolder shows the folder where the .env goes, creating it first.
func (s *Settings) OpenConfigFolder() error {
	dir := s.st.opts.ConfigDir
	if dir == "" {
		return ErrNoConfigDir
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return s.st.host.OpenExternal(dir)
}

// CreateConfigFile writes a .env with comments and empty IDs for the user
// to fill in, then shows its folder. It never replaces an existing file.
func (s *Settings) CreateConfigFile() error {
	path := s.st.envFile()
	if path == "" {
		return ErrNoConfigDir
	}
	if err := os.MkdirAll(s.st.opts.ConfigDir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return ErrConfigExists
	}
	if err != nil {
		return err
	}
	_, err = f.WriteString(config.Example)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return s.st.host.OpenExternal(s.st.opts.ConfigDir)
}

// CheckUpdate asks GitHub for a newer published release (Policies III.B.1).
// It sends nothing about the user; nil means none, or that it could not
// tell (a failed check is not worth bothering the user about).
func (s *Settings) CheckUpdate(ctx context.Context) *Release {
	rel, err := update.Check(ctx, s.st.opts.HTTP, s.st.opts.UpdateURL, s.st.opts.Version)
	if err != nil || rel == nil {
		return nil
	}
	return &Release{Tag: rel.Tag, URL: rel.URL}
}
