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
| `youtube create <playlist>... [-y] [--privacy private\|unlisted\|public]` | Only get the YouTube playlists ready, without tracks: create them (private by default) or reuse one with the same title |
| `youtube add <playlist>... [-y] [--privacy …] [--max-searches N] [--limit N] [--report file.csv\|.json]` | Copy playlists: create or reuse each YouTube playlist, add what is missing, remove videos no Spotify track matches. Shows everything and asks once before writing; several playlists run in parallel |
| `login spotify` / `login youtube` | Log in and show the connected account or channel |
| `exit` | Leave; Google access is revoked, nothing is stored on disk |

Each argument is one playlist; quote names with spaces: `youtube add "The Way to My Favorite Part" Covers`.

Messages follow your system language when it is Spanish or English (Spanish otherwise); `--lang es` or `--lang en` picks one, also inside the shell. Cobra's built-in help and the CSV/JSON report stay in English.

Matching prefers the artist's `- Topic` channel (album audio), then VEVO, then the first result. A track whose video is already in the playlist (same title, artist and length) needs no search, so a long playlist keeps advancing day after day. If you replaced a video by hand, Soundsync keeps yours.

YouTube quota (October 2026): 100 searches and 10,000 units per day per Google Cloud project; every write (create, add, remove) costs 50 units. When the searches run out, what was found is still written and the rest waits for the next run (`--max-searches` sets a lower budget).

## Privacy

Soundsync asks you to accept its [privacy policy](PRIVACY.md) once, before the first login. Nothing from Spotify or YouTube is stored on your computer.

Progress and open items: [docs/todo.md](docs/todo.md).
