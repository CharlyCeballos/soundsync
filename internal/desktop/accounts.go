package desktop

import (
	"context"
	"fmt"

	"github.com/CharlyCeballos/soundsync/internal/i18n"
	"github.com/CharlyCeballos/soundsync/internal/session"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
)

// Accounts connects Spotify and YouTube. Tokens stay in memory; Google's is
// revoked on disconnect and when the app closes.
type Accounts struct{ st *state }

// Account is who is logged in to Spotify.
type Account struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Channel is the YouTube channel playlists are written to.
type Channel struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// SourcePlaylist is a playlist of the user's Spotify library.
type SourcePlaylist struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Owner  string `json:"owner"`
	Tracks int    `json:"tracks"` // -1 when Spotify does not say
	Access string `json:"access"` // owned, collaborative or followed (not readable)
	URL    string `json:"url"`
}

func accountOf(s *session.Spotify) *Account {
	name := s.Me.DisplayName
	if name == "" {
		name = s.Me.ID
	}
	return &Account{ID: s.Me.ID, Name: name}
}

func channelOf(y *session.YouTube) *Channel {
	return &Channel{ID: y.Channel.ID, Title: y.Channel.Title, URL: "https://www.youtube.com/channel/" + y.Channel.ID}
}

// loginUI opens the login page and also tells the frontend, so the user can
// open it by hand if the browser did not come up.
func (st *state) loginUI(provider string) session.UI {
	return session.UI{Open: func(authURL string) error {
		st.host.Emit(EventLoginURL, LoginURL{Provider: provider, URL: authURL})
		_ = st.host.OpenExternal(authURL) // not fatal: the frontend shows the link
		return nil
	}}
}

// ready checks what every login needs: consent and a valid configuration.
func (st *state) ready() error {
	if !st.policyOK() {
		return ErrPolicyNotAccepted
	}
	_, err := st.config()
	return err
}

// ConnectSpotify opens the browser to log in to Spotify and waits for it.
// Cancelling the call cancels the wait.
func (a *Accounts) ConnectSpotify(ctx context.Context) (*Account, error) {
	st := a.st
	if err := st.ready(); err != nil {
		return nil, err
	}
	ctx, end, err := st.begin(ctx, "login")
	if err != nil {
		return nil, err
	}
	defer end()
	cfg, err := st.config()
	if err != nil {
		return nil, err
	}
	s, err := st.opts.LoginSpotify(ctx, st.base, cfg, st.loginUI("spotify"))
	if err != nil {
		return nil, err
	}
	st.mu.Lock()
	st.sp = s
	st.resetCopy()
	st.mu.Unlock()
	st.host.Focus()
	return accountOf(s), nil
}

// ConnectYouTube opens the browser to log in to Google, then reads the
// channel playlists will be written to.
func (a *Accounts) ConnectYouTube(ctx context.Context) (*Channel, error) {
	st := a.st
	if err := st.ready(); err != nil {
		return nil, err
	}
	ctx, end, err := st.begin(ctx, "login")
	if err != nil {
		return nil, err
	}
	defer end()
	cfg, err := st.config()
	if err != nil {
		return nil, err
	}
	y, err := st.opts.LoginYouTube(ctx, st.base, cfg, st.loginUI("youtube"))
	if err != nil {
		return nil, err
	}
	st.mu.Lock()
	old := st.yt
	st.yt = y
	st.resetCopy()
	st.mu.Unlock()
	if old != nil {
		_ = old.Revoke() // the previous Google grant is no longer used
	}
	st.host.Focus()
	return channelOf(y), nil
}

// DisconnectSpotify forgets the Spotify session (Spotify has no way to
// revoke a token; it dies here).
func (a *Accounts) DisconnectSpotify() error {
	st := a.st
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.busy != "" {
		return ErrBusy
	}
	st.sp = nil
	st.resetCopy()
	return nil
}

// DisconnectYouTube revokes Google's grant right away (Policies III.D.2.c).
func (a *Accounts) DisconnectYouTube() error {
	st := a.st
	st.mu.Lock()
	if st.busy != "" {
		st.mu.Unlock()
		return ErrBusy
	}
	y := st.yt
	st.yt = nil
	st.resetCopy()
	st.mu.Unlock()
	if y == nil {
		return nil
	}
	if err := y.Revoke(); err != nil {
		return fmt.Errorf(i18n.T("%w\nrevoke it by hand at https://myaccount.google.com/permissions"), err)
	}
	return nil
}

// Playlists lists the Spotify library; refresh reads it again.
func (a *Accounts) Playlists(ctx context.Context, refresh bool) ([]SourcePlaylist, error) {
	st := a.st
	st.mu.Lock()
	sp := st.sp
	st.mu.Unlock()
	if sp == nil {
		return nil, ErrSpotifyFirst
	}
	pls, err := sp.Playlists(ctx, refresh)
	if err != nil {
		return nil, err
	}
	out := make([]SourcePlaylist, len(pls))
	for i, p := range pls {
		out[i] = SourcePlaylist{ID: p.ID, Name: p.Name, Owner: ownerName(p), Tracks: p.Total, Access: string(p.AccessFor(sp.Me.ID)), URL: p.URL()}
	}
	return out, nil
}

func ownerName(p spotify.Playlist) string {
	if p.OwnerName != "" {
		return p.OwnerName
	}
	return p.OwnerID
}
