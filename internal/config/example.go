package config

// SetupGuideURL explains how to create the OAuth clients.
const SetupGuideURL = "https://github.com/CharlyCeballos/soundsync/blob/main/docs/setup.md"

// exampleHeader opens a .env written by the GUI; the rest matches the
// repository's .env.example (a test keeps them in step).
const exampleHeader = "# Soundsync settings: fill in your own Spotify and Google OAuth clients.\n" +
	"# Guide: " + SetupGuideURL + "\n"

// Example is the .env the GUI creates for the user to fill in. It holds no
// value worth protecting: only comments, empty IDs and default URIs.
const Example = exampleHeader + exampleBody

const exampleBody = `
# Spotify Developer Dashboard — app type Web or Desktop, PKCE.
# Redirect URI to register exactly:
#   http://127.0.0.1:8888/callback
SPOTIFY_CLIENT_ID=
SPOTIFY_REDIRECT_URI=http://127.0.0.1:8888/callback

# Google Cloud — OAuth client type Desktop. PKCE.
# Loopback redirect (Google Desktop clients allow 127.0.0.1):
#   http://127.0.0.1:8889/callback
GOOGLE_CLIENT_ID=
# Desktop clients get a secret that is not confidential, but Google's token
# endpoint may still require it. Shown only once at creation: save it here.
GOOGLE_CLIENT_SECRET=
GOOGLE_REDIRECT_URI=http://127.0.0.1:8889/callback
`
