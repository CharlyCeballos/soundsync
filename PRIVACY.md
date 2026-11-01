# Soundsync privacy policy

Version 2026-11-01. Soundsync asks you to accept this policy again whenever this version changes.

Soundsync is a free, open source program that runs on your own computer and copies your Spotify playlists to YouTube. There is no Soundsync server and no Soundsync account. The author never receives your data.

## YouTube API Services

Soundsync uses the **YouTube API Services**. By using Soundsync you agree to be bound by the [YouTube Terms of Service](https://www.youtube.com/t/terms). Google's handling of your data is described in the [Google Privacy Policy](https://policies.google.com/privacy).

## What Soundsync reads

| From | What | Why |
|---|---|---|
| Spotify | Your user ID, your playlists and their tracks (name, artists, album, length, Spotify ID). Read-only access. | To know what to copy |
| YouTube | Your channel name, your playlists and the videos in them, search results, video lengths | To find each track and avoid duplicates |

Spotify's handling of your data is described in the [Spotify Privacy Policy](https://www.spotify.com/legal/privacy-policy/).

## What Soundsync changes

Only on your YouTube channel, and only after showing you a summary and getting your confirmation (or when you run a command with `--yes`): it creates playlists, adds a line to the description of a playlist you choose to reuse, adds videos to playlists, and removes videos that no longer match a track in the Spotify playlist. It never changes the privacy of an existing playlist. It never writes to Spotify.

## What Soundsync stores

- **Nothing from Spotify or YouTube is saved on your computer.** Login tokens live in memory only and disappear when Soundsync exits. Matches found during a session are also kept in memory only.
- One small file records that you accepted this policy, and which version: `%APPDATA%\soundsync\privacy-accepted` on Windows, `~/.config/soundsync/privacy-accepted` on Linux. It holds no personal data.
- Reports are written to disk only when you ask for one, to the file you name.

Soundsync identifies the YouTube playlists it manages by a line it writes in their description (`soundsync:spotify:<playlist ID>`). That line lives on YouTube, under your control: deleting it makes Soundsync forget the link.

## What Soundsync shares

Soundsync talks only to Spotify (accounts.spotify.com, api.spotify.com) and Google (accounts.google.com, oauth2.googleapis.com, www.googleapis.com), and only to do what you ask. When update checks are added, they will contact GitHub to read the latest version number, without sending any data about you. There is no analytics, telemetry or advertising.

## Revoking access and deleting data

- Soundsync revokes its Google access every time it exits.
- You can revoke Google access at any time at [myaccount.google.com/permissions](https://myaccount.google.com/permissions), and Spotify access at [spotify.com/account/apps](https://www.spotify.com/account/apps/).
- To delete everything Soundsync keeps on your computer, delete the `privacy-accepted` file above.

## Contact

Questions or concerns: open an issue at [github.com/CharlyCeballos/soundsync/issues](https://github.com/CharlyCeballos/soundsync/issues).
