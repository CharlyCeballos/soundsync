package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/transfer"
	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

// reportAttribution goes on every row: Spotify Policy II.4.1–II.4.2 and
// YouTube Policies III.F.2 ask to credit the source next to the content.
const reportAttribution = "Data from Spotify. Data from YouTube."

// reportRow is one Spotify track and what happened to it, or one YouTube
// video removed from the playlist (todo 2.10).
type reportRow struct {
	Playlist       string `json:"playlist"`
	Position       int    `json:"position,omitempty"`
	SpotifyArtists string `json:"spotify_artists,omitempty"`
	SpotifyTrack   string `json:"spotify_track,omitempty"`
	SpotifyURL     string `json:"spotify_track_url,omitempty"`
	Status         string `json:"status"`
	YouTubeTitle   string `json:"youtube_title,omitempty"`
	YouTubeChannel string `json:"youtube_channel,omitempty"`
	YouTubeURL     string `json:"youtube_video_url,omitempty"`
	Why            string `json:"why,omitempty"`
}

var reportHeader = []string{"playlist", "position", "spotify_artists", "spotify_track", "spotify_track_url", "status",
	"youtube_title", "youtube_channel", "youtube_video_url", "why", "attribution"}

// checkReportPath rejects a bad report path before any login or quota.
func checkReportPath(path string) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv", ".json":
	default:
		return fmt.Errorf("--report %q: use a .csv or .json file name", path)
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("--report %q already exists; choose another file name (Soundsync never overwrites files)", path)
	}
	return nil
}

// saveReport writes the run's report; it refuses to replace an existing file.
func saveReport(out io.Writer, path string, jobs []*addJob) error {
	rows := reportRows(jobs)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("report not saved: %q already exists", path)
	}
	if err != nil {
		return fmt.Errorf("report not saved: %w", err)
	}
	if strings.ToLower(filepath.Ext(path)) == ".json" {
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		err = enc.Encode(struct {
			Attribution string      `json:"attribution"`
			Rows        []reportRow `json:"rows"`
		}{reportAttribution, rows})
	} else {
		err = writeCSV(f, rows)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("report not saved: %w", err)
	}
	fmt.Fprintf(out, "Report saved to %s (%d rows).\n", path, len(rows))
	return nil
}

func writeCSV(w io.Writer, rows []reportRow) error {
	cw := csv.NewWriter(w)
	cw.Write(reportHeader)
	for _, r := range rows {
		pos := ""
		if r.Position > 0 {
			pos = strconv.Itoa(r.Position)
		}
		cw.Write([]string{
			cell(r.Playlist), pos, cell(r.SpotifyArtists), cell(r.SpotifyTrack), r.SpotifyURL, r.Status,
			cell(r.YouTubeTitle), cell(r.YouTubeChannel), r.YouTubeURL, r.Why, reportAttribution,
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

// reportRows lists every track of every job in Spotify order, then the
// videos removed (or to remove) from each YouTube playlist.
func reportRows(jobs []*addJob) []reportRow {
	var rows []reportRow
	for _, j := range jobs {
		status := trackStatuses(j)
		entries := append(append([]spotify.Entry(nil), j.tracks...), j.skipped...)
		sort.Slice(entries, func(a, b int) bool { return entries[a].Position < entries[b].Position })
		for _, e := range entries {
			row := reportRow{
				Playlist:       j.src.Name,
				Position:       e.Position,
				SpotifyArtists: strings.Join(e.Track.Artists, ", "),
				SpotifyTrack:   e.Track.Name,
				SpotifyURL:     e.Track.URL(),
			}
			if e.Skip != "" {
				row.Status = "not searchable: " + string(e.Skip)
			} else if s, ok := status[e.Position]; ok {
				row.Status, row.Why = s.status, s.why
				if s.video.ID != "" {
					row.YouTubeTitle, row.YouTubeChannel, row.YouTubeURL = s.video.Title, s.video.ChannelTitle, s.video.URL()
				}
			} else if j.err != nil {
				row.Status = "not searched: the run stopped"
			} else {
				row.Status = "waiting: no searches left"
			}
			rows = append(rows, row)
		}
		for i, it := range j.plan.Extra {
			st := "removed"
			if i >= j.removed {
				st = "to remove (not done)"
			}
			rows = append(rows, reportRow{Playlist: j.src.Name, Status: st, YouTubeTitle: it.Title, YouTubeChannel: it.Channel, YouTubeURL: it.URL()})
		}
	}
	return rows
}

type trackStatus struct {
	status string
	why    string
	video  youtube.Video
}

// trackStatuses says what happened to each matched track, by position.
func trackStatuses(j *addJob) map[int]trackStatus {
	out := map[int]trackStatus{}
	put := func(ms []transfer.Matched, status string) {
		for _, m := range ms {
			out[m.Entry.Position] = trackStatus{status: status, why: string(m.Reason), video: m.Video}
		}
	}
	put(j.plan.Present, "already there")
	put(j.plan.SameVideo, "same video as an earlier track")
	put(j.plan.Unmatched, "no match")
	put(j.leftOut, "left out: playlist full")
	for i, m := range j.plan.ToAdd {
		st := "added"
		if i >= j.added {
			st = "to add (not done)"
		}
		out[m.Entry.Position] = trackStatus{status: st, why: string(m.Reason), video: m.Video}
	}
	for _, r := range j.plan.Replaced {
		v := youtube.Video{ID: r.Item.VideoID, Title: r.Item.Title, ChannelTitle: r.Item.Channel}
		out[r.Entry.Position] = trackStatus{status: "kept yours", why: "yours", video: v}
	}
	return out
}
