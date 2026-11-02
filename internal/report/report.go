// Package report turns a copy run into one row per Spotify track (and per
// YouTube video removed), and writes the rows as CSV or JSON (todo 2.10).
// The CLI saves it on --report; the GUI shows the same rows and saves them
// on demand.
package report

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/CharlyCeballos/soundsync/internal/copier"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/transfer"
	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

// Attribution goes on every row: Spotify Policy II.4.1–II.4.2 and YouTube
// Policies III.F.2 ask to credit the source next to the content.
const Attribution = "Data from Spotify. Data from YouTube."

// Status says what happened to a track or a removed video. The values are
// part of the report format: keep them stable.
type Status string

const (
	Added             Status = "added"
	ToAdd             Status = "to add (not done)"
	AlreadyThere      Status = "already there"
	SameVideo         Status = "same video as an earlier track"
	NoMatch           Status = "no match"
	LeftOut           Status = "left out: playlist full"
	KeptYours         Status = "kept yours"
	NotSearchedFailed Status = "not searched: the run stopped"
	Waiting           Status = "waiting: no searches left"
	Episode           Status = "not searchable: " + Status(spotify.SkipEpisode)
	Unavailable       Status = "not searchable: " + Status(spotify.SkipUnavailable)
	Removed           Status = "removed"
	ToRemove          Status = "to remove (not done)"
)

// Row is one Spotify track and what happened to it, or one YouTube video
// removed from the playlist (no Spotify fields).
type Row struct {
	Playlist       string `json:"playlist"`
	Position       int    `json:"position,omitempty"`
	SpotifyArtists string `json:"spotify_artists,omitempty"`
	SpotifyTrack   string `json:"spotify_track,omitempty"`
	SpotifyURL     string `json:"spotify_track_url,omitempty"`
	Status         Status `json:"status"`
	YouTubeTitle   string `json:"youtube_title,omitempty"`
	YouTubeChannel string `json:"youtube_channel,omitempty"`
	YouTubeURL     string `json:"youtube_video_url,omitempty"`
	Why            string `json:"why,omitempty"`
}

var header = []string{"playlist", "position", "spotify_artists", "spotify_track", "spotify_track_url", "status",
	"youtube_title", "youtube_channel", "youtube_video_url", "why", "attribution"}

// Rows lists every track of every job in Spotify order, then the videos
// removed (or to remove) from each YouTube playlist.
func Rows(jobs []*copier.Job) []Row {
	var rows []Row
	for _, j := range jobs {
		status := trackStatuses(j)
		entries := append(append([]spotify.Entry(nil), j.Tracks...), j.Skipped...)
		sort.Slice(entries, func(a, b int) bool { return entries[a].Position < entries[b].Position })
		for _, e := range entries {
			row := Row{
				Playlist:       j.Source.Name,
				Position:       e.Position,
				SpotifyArtists: strings.Join(e.Track.Artists, ", "),
				SpotifyTrack:   e.Track.Name,
				SpotifyURL:     e.Track.URL(),
			}
			switch s, known := status[e.Position]; {
			case e.Skip != "":
				row.Status = "not searchable: " + Status(e.Skip)
			case known:
				row.Status, row.Why = s.status, s.why
				if s.video.ID != "" {
					row.YouTubeTitle, row.YouTubeChannel, row.YouTubeURL = s.video.Title, s.video.ChannelTitle, s.video.URL()
				}
			case j.Err != nil:
				row.Status = NotSearchedFailed
			default:
				row.Status = Waiting
			}
			rows = append(rows, row)
		}
		for i, it := range j.Plan.Extra {
			st := Removed
			if i >= j.Removed {
				st = ToRemove
			}
			rows = append(rows, Row{Playlist: j.Source.Name, Status: st, YouTubeTitle: it.Title, YouTubeChannel: it.Channel, YouTubeURL: it.URL()})
		}
	}
	return rows
}

type trackStatus struct {
	status Status
	why    string
	video  youtube.Video
}

// trackStatuses says what happened to each matched track, by position.
func trackStatuses(j *copier.Job) map[int]trackStatus {
	out := map[int]trackStatus{}
	put := func(ms []transfer.Matched, status Status) {
		for _, m := range ms {
			out[m.Entry.Position] = trackStatus{status: status, why: string(m.Reason), video: m.Video}
		}
	}
	put(j.Plan.Present, AlreadyThere)
	put(j.Plan.SameVideo, SameVideo)
	put(j.Plan.Unmatched, NoMatch)
	put(j.LeftOut, LeftOut)
	for i, m := range j.Plan.ToAdd {
		st := Added
		if i >= j.Added {
			st = ToAdd
		}
		out[m.Entry.Position] = trackStatus{status: st, why: string(m.Reason), video: m.Video}
	}
	for _, r := range j.Plan.Replaced {
		v := youtube.Video{ID: r.Item.VideoID, Title: r.Item.Title, ChannelTitle: r.Item.Channel}
		out[r.Entry.Position] = trackStatus{status: KeptYours, why: "yours", video: v}
	}
	return out
}

// Format names a report format.
type Format string

const (
	CSV  Format = "csv"
	JSON Format = "json"
)

// ErrFormat rejects a file name that is neither .csv nor .json.
var ErrFormat = errors.New("not a .csv or .json file name")

// FormatOf picks the format from a file name.
func FormatOf(path string) (Format, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv":
		return CSV, nil
	case ".json":
		return JSON, nil
	}
	return "", ErrFormat
}

// Write encodes rows in the given format.
func Write(w io.Writer, f Format, rows []Row) error {
	switch f {
	case JSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Attribution string `json:"attribution"`
			Rows        []Row  `json:"rows"`
		}{Attribution, rows})
	case CSV:
		return writeCSV(w, rows)
	}
	return ErrFormat
}

func writeCSV(w io.Writer, rows []Row) error {
	cw := csv.NewWriter(w)
	cw.Write(header)
	for _, r := range rows {
		pos := ""
		if r.Position > 0 {
			pos = strconv.Itoa(r.Position)
		}
		cw.Write([]string{
			cell(r.Playlist), pos, cell(r.SpotifyArtists), cell(r.SpotifyTrack), r.SpotifyURL, string(r.Status),
			cell(r.YouTubeTitle), cell(r.YouTubeChannel), r.YouTubeURL, r.Why, Attribution,
		})
	}
	cw.Flush()
	return cw.Error()
}

// cell defuses spreadsheet formulas: names come from Spotify and YouTube,
// and a cell starting with = + - @ would run as a formula in Excel (OWASP
// "CSV injection").
func cell(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// Save writes the rows to a new file; the format comes from its extension.
// It never replaces an existing file: that error wraps fs.ErrExist.
func Save(path string, rows []Row) error {
	format, err := FormatOf(path)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	err = Write(f, format, rows)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
