# Soundsync

Standalone app that copies playlists **Spotify → YouTube** (YouTube Data API playlists; they show up in YouTube Music).

v1 stays in developer/testing mode. Tokens live in RAM only.

## OAuth scopes (minimum)

Register these and nothing extra.

### Spotify

| Scope | Why |
|---|---|
| `playlist-read-private` | List the user’s private playlists and tracks |
| `playlist-read-collaborative` | List collaborative playlists (read snapshot; destination playlist on YouTube is owned by the user) |

No Spotify write scopes. Redirect URI: `http://127.0.0.1:8888/callback` (loopback + PKCE).

### YouTube (Google)

| Scope | Why |
|---|---|
| `https://www.googleapis.com/auth/youtube.force-ssl` | Create playlist, insert/delete/list items (`playlists.insert`, `playlistItems.insert` / `delete` / `list`, `search.list`) |

OAuth client type: **Desktop**. Consent screen: **Testing**. Add the author as a tester. Redirect URI: `http://127.0.0.1:8889/callback`.

YouTube Data API v3 must be enabled on the Google Cloud project.

## Local config

```bash
cp .env.example .env
```

Put Client IDs (and the Google Desktop client secret) in `.env`. Do not commit `.env` or Google `client_secret*.json`.

Each user creates their own Spotify app and Google OAuth client: see [docs/setup.md](docs/setup.md). Platform rules that shape the app: [docs/compliance.md](docs/compliance.md).

## Build and run (phase 1 spike)

Requires Go 1.26 or newer.

```bash
go test ./...
go run ./cmd/soundsync config check   # validates .env without printing values
go run ./cmd/soundsync shell          # one browser login per service for the whole session
```

Inside the shell:

| Command | What it does |
|---|---|
| `spotify playlists` | List your Spotify library (owned, collaborative, followed) |
| `spotify tracks <playlist>` | List a playlist's tracks; accepts a name, ID, link or `spotify:` URI |
| `youtube playlists` | List your YouTube playlists and which ones Soundsync created |
| `youtube create <playlist>... [-y] [--privacy private\|unlisted\|public]` | Create empty YouTube playlists (private by default); reuses existing ones by their marker |
| `youtube add <playlist>... [--limit N] [-y]` | Search each track, show the matches, ask once, then add only what is missing; several playlists run in parallel |
| `login spotify` / `login youtube` | Log in and show the connected account or channel |
| `exit` | Leave; Google access is revoked, nothing is stored on disk |

Each argument is one playlist; quote names with spaces: `youtube add "The Way to My Favorite Part" Covers`.

Matching prefers the artist's `- Topic` channel (album audio), then VEVO, then the first result. YouTube quota (October 2026): 100 searches and 10,000 units per day per Google Cloud project; adding a video costs 50 units. Progress and open items: [docs/todo.md](docs/todo.md).
