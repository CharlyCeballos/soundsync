# Soundsync

[Español](README.es.md)

Soundsync copies your playlists from **Spotify to YouTube** (YouTube playlists, so they show up in YouTube Music too). It runs on your computer, as a desktop app (Windows and Debian) or as a command-line program: no server, no account, nothing from Spotify or YouTube saved on disk.

Run it again whenever you like: it adds what is new, removes what you took out of the Spotify playlist, and leaves alone what is already there, including videos you replaced by hand.

Soundsync is an independent project. It is not affiliated with, endorsed or sponsored by Spotify, YouTube or Google.

## Before you start

Soundsync uses the official Spotify and YouTube APIs with **your own** developer credentials, which you set up once:

- A Spotify app of your own. The account that owns it needs **Spotify Premium**, and the app stays in Development Mode: up to 5 Spotify accounts can use it.
- A Google Cloud project with the YouTube Data API and an OAuth client of type Desktop, in Testing mode with your Google account as a tester.
- A YouTube channel on that Google account (if it has none, Soundsync tells you where to create it).

Step-by-step guide: [docs/setup.md](docs/setup.md).

## Install

Download the latest release from the [Releases page](https://github.com/CharlyCeballos/soundsync/releases). `SHA256SUMS` lists the checksum of every file.

### Desktop app

**Windows 10/11 (64-bit):** run `soundsync-gui_<version>_windows_amd64.msi`. It installs for your user only (no administrator rights) and adds **Soundsync** to the Start menu; a newer `.msi` replaces the old version. The installer is not code-signed, so SmartScreen may warn: choose **More info → Run anyway**. Soundsync needs the Microsoft Edge WebView2 Runtime, which Windows 11 already includes; if it is missing, Soundsync says so and offers Microsoft's download page. Uninstall it from **Settings → Apps**; your settings folder `%APPDATA%\soundsync` (your `.env`, consent and the window's cache) stays until you delete it.

**Debian 13 (64-bit):**

```bash
sudo apt install ./soundsync-gui_<version>_amd64.deb
```

It appears as **Soundsync** in the applications menu and pulls in GTK 4 and WebKitGTK 6.0. It is built for Debian 13; other distributions are not tested. `sudo apt remove soundsync-gui` uninstalls it; `~/.config/soundsync` keeps your settings until you delete it.

### Command line

**Windows (64-bit):** unzip `soundsync_<version>_windows_amd64.zip` into a folder of your choice and run `soundsync.exe` from a terminal (PowerShell or Command Prompt) in that folder. The program is not code-signed, so Windows SmartScreen may warn the first time: choose **More info → Run anyway**.

**Debian, Ubuntu and derivatives (64-bit):**

```bash
sudo apt install ./soundsync_<version>_amd64.deb
```

Updates: the desktop app, and `soundsync shell` when you open it, tell you if a newer version is out and where to get it. Neither downloads or installs anything by itself.

## Configure

Copy `.env.example` to a file named `.env` and fill it in with your credentials ([docs/setup.md](docs/setup.md) says where each value comes from). The example ships in the Windows zip and, with the Debian package, at `/usr/share/doc/soundsync/env.example`.

Soundsync looks for `.env` in the current folder first, then in:

- Windows: `%APPDATA%\soundsync\.env`
- Linux: `~/.config/soundsync/.env`

`--env-file <path>` uses any other file. The desktop app reads only the file in that folder: its first screen shows which values are missing (never the values themselves) and can create the file for you, ready to fill in. Check it from the command line with:

```bash
soundsync config check   # says which file it read; never prints the values
```

Never share or commit your `.env`.

## Use the desktop app

Open **Soundsync**. The first time, it asks you to accept its privacy policy; then three steps:

1. **Choose playlists** — connect Spotify (your browser opens to log in) and tick the playlists to copy.
2. **Review on YouTube** — connect your YouTube channel, decide what to do when your channel already has a playlist with the same name, and let Soundsync look for the songs. You see every change and its quota cost; nothing is written until you press **Write to YouTube**.
3. **Done** — what changed in each playlist, a link to open it in YouTube Music, and a report you can download as CSV or JSON.

**Disconnect** next to each account ends that session; closing the app revokes Google access.

## Use the command line

```bash
soundsync shell
```

The shell keeps you logged in until you type `exit`: each service asks for a browser login once. Inside it:

| Command | What it does |
|---|---|
| `spotify playlists` | List your Spotify library (owned, collaborative, followed) |
| `spotify tracks <playlist>` | List a playlist's tracks; accepts a name, ID, link or `spotify:` URI |
| `youtube playlists` | List your YouTube playlists and which ones Soundsync created |
| `youtube add <playlist>... [-y] [--privacy private\|unlisted\|public] [--max-searches N] [--limit N] [--report file.csv\|.json]` | Copy playlists: create or reuse each YouTube playlist, add what is missing, remove videos no Spotify track matches. Shows everything and asks once before writing; several playlists run in parallel |
| `youtube create <playlist>... [-y] [--privacy …]` | Only get the YouTube playlists ready, without tracks |
| `login spotify` / `login youtube` | Log in and show the connected account or channel |
| `exit` | Leave; Google access is revoked |

Each argument is one playlist; quote names with spaces: `youtube add "The Way to My Favorite Part" Covers`. New playlists are private unless you pass `--privacy`.

Messages follow your system language when it is Spanish or English (Spanish otherwise); `--lang es` or `--lang en` picks one, also inside the shell.

**Matching.** Soundsync prefers the artist's `- Topic` channel (album audio), then VEVO, then the first search result. A track whose video is already in the playlist (same title, artist and length) needs no search, so a long playlist keeps advancing day after day. If you replaced a video by hand, Soundsync keeps yours.

**YouTube quota.** Your Google Cloud project gets 100 searches and 10,000 units per day; every write (create, add, remove) costs 50 units. That is roughly 100 new tracks a day. When the searches run out, what was found is still written and the rest waits for the next run (`--max-searches` sets a lower budget).

## Privacy

Soundsync asks you to accept its [privacy policy](PRIVACY.md) once, before the first login. Login tokens live in memory only, and Google access is revoked when Soundsync exits. Nothing from Spotify or YouTube is stored on your computer. The desktop app also keeps its window's web cache (its own page and the language you picked) in the same settings folder, under `webview`.

## OAuth scopes

Register these and nothing extra.

| Platform | Scope | Why |
|---|---|---|
| Spotify | `playlist-read-private` | List your private playlists and their tracks |
| Spotify | `playlist-read-collaborative` | List collaborative playlists |
| Google | `https://www.googleapis.com/auth/youtube.force-ssl` | Create playlists, list/insert/delete their items, search videos |

No Spotify write scopes. Redirect URIs (loopback + PKCE): `http://127.0.0.1:8888/callback` for Spotify, `http://127.0.0.1:8889/callback` for Google.

## Build from source

Requires Go 1.26 or newer.

```bash
go test ./...
go run ./cmd/soundsync shell
```

The desktop app lives in `gui/` (Wails v3 + React; Node 20.19+ and `wails3` too): see [gui/README.md](gui/README.md).

Releases are built by GitHub Actions from a `vX.Y.Z` tag (`.github/workflows/release.yml`). Platform rules that shape the app: [docs/compliance.md](docs/compliance.md). Progress and open items: [docs/todo.md](docs/todo.md).

## License

[MIT](LICENSE)
