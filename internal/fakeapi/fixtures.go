// Package fakeapi serves in-memory fakes of the Spotify Web API, the YouTube
// Data API and Google's token revocation, for tests and the GUI's e2e build.
// Nothing here talks to the network: callers wrap the handlers in
// httptest.NewServer (or any loopback server) and point the clients at it.
package fakeapi

import "github.com/CharlyCeballos/soundsync/internal/transfer"

// IDs of the fixture library: two playlists the user owns and one followed.
const (
	UserID     = "me"
	WayID      = "5ZwLvrI8d6VnEQXxB0bMms"
	CoversID   = "3yQi6hff0Ic45VJfJoaXIn"
	FollowedID = "3REKkLasecJIJnNwk8Kzeg"
)

// NewLibrary is the fixture Spotify library: "The Way to My Favorite Part"
// (two tracks and an episode), "Covers" (two tracks) and a playlist that
// belongs to someone else.
func NewLibrary() *Spotify {
	return &Spotify{
		Me: map[string]any{"id": UserID, "display_name": "Charly"},
		Playlists: []map[string]any{
			SpotifyPlaylist(WayID, "The Way to My Favorite Part", UserID, 2),
			SpotifyPlaylist(CoversID, "Covers", UserID, 2),
			SpotifyPlaylist(FollowedID, "Pentatonix Evolution of Music", "someone", -1),
		},
		Entries: map[string][]map[string]any{
			WayID: {
				Track("1Onx", "The Way (feat. Mac Miller)", 227000, "Ariana Grande", "Mac Miller"),
				Episode("ep", "A podcast"),
				Track("66wk", "My Favorite Part", 215000, "Mac Miller", "Ariana Grande"),
			},
			CoversID: {
				Track("c1", "Hallelujah", 413000, "Jeff Buckley"),
				Track("c2", "Hurt", 218000, "Johnny Cash"),
			},
		},
	}
}

// NewChannel is the fixture YouTube channel: it already has both Soundsync
// playlists of NewLibrary, empty, and search results for every track.
func NewChannel() *YouTube {
	return &YouTube{
		Playlists: []map[string]any{
			Playlist("PLway", "The Way to My Favorite Part", WayID),
			Playlist("PLcov", "Covers", CoversID),
		},
		Search: map[string][]map[string]any{
			"Ariana Grande The Way (feat. Mac Miller)": {
				Video("vevoV", "The Way (Official Video)", "ArianaGrandeVevo"),
				Video("lyricV", "The Way lyrics", "Lyrics Hub"),
				Video("topicV", "The Way", "Ariana Grande - Topic"),
			},
			"Mac Miller My Favorite Part": {
				Video("firstV", "Mac Miller - My Favorite Part (feat. Ariana Grande) &amp; more", "Some Uploader"),
			},
			"Jeff Buckley Hallelujah": {Video("hallV", "Hallelujah", "Jeff Buckley - Topic")},
			"Johnny Cash Hurt":        {Video("hurtV", "Hurt", "JohnnyCashVEVO")},
		},
		Items: map[string][]Item{},
		// Lengths match the Spotify fixture's, except the VEVO video's intro.
		Lengths: map[string]string{"topicV": "PT3M47S", "vevoV": "PT4M10S", "firstV": "PT3M35S", "hallV": "PT6M53S", "hurtV": "PT3M38S"},
	}
}

// Video is one search.list result.
func Video(id, title, channel string) map[string]any {
	return map[string]any{
		"id":      map[string]string{"videoId": id},
		"snippet": map[string]string{"title": title, "channelTitle": channel},
	}
}

// Playlist is a private playlist carrying the Soundsync marker of spotifyID.
func Playlist(id, title, spotifyID string) map[string]any {
	return map[string]any{
		"id":      id,
		"snippet": map[string]string{"title": title, "description": transfer.Description(spotifyID)},
		"status":  map[string]string{"privacyStatus": "private"},
	}
}

// UnmarkedPlaylist is a private playlist Soundsync did not create.
func UnmarkedPlaylist(id, title, description string, videos int) map[string]any {
	return map[string]any{
		"id":             id,
		"snippet":        map[string]string{"title": title, "description": description},
		"status":         map[string]string{"privacyStatus": "private"},
		"contentDetails": map[string]int{"itemCount": videos},
	}
}
