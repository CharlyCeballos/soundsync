# Soundsync — credential setup

Each user brings their own Spotify app and Google OAuth client (see `compliance.md`: Spotify Terms VI.1.2, YouTube Policies III.D.1).
Done once per user. Author's setup completed 2026-10-26.

## Before you start

- Spotify **Premium** on the account that owns the Spotify app (Dev Mode requirement).
- Use your normal browser. Google blocks sign-in in automated browsers (Playwright: “This browser or app may not be secure”).

## 1. Spotify app

1. https://developer.spotify.com/dashboard → **Create app**.
2. Fill in:
   - App name: `Soundsync` (must not contain “Spotify” or start with “Spot”).
   - Description: personal, non-commercial, development mode.
   - Redirect URI: `http://127.0.0.1:8888/callback` → **Add**.
   - APIs: **Web API** only.
3. Accept Developer Terms + Design Guidelines → **Save**.
4. Copy the **Client ID** to `.env` → `SPOTIFY_CLIENT_ID`. Do not copy the client secret (PKCE).
5. Other users of the same app: **User Management** (max 5).

## 2. Google Cloud project + YouTube Data API

1. https://console.cloud.google.com/projectcreate → name `soundsync` → **Create**. Check it is selected.
2. https://console.cloud.google.com/apis/library/youtube.googleapis.com → **Enable**.

## 3. Consent screen + OAuth client

1. https://console.cloud.google.com/auth/overview → **Get started**: app name `Soundsync`, support email, audience **External**, contact email, accept the User Data Policy.
2. **Audience** → **Test users** → add your Gmail. Publishing status stays **Testing**.
3. **Data Access** → **Add or remove scopes** → add `https://www.googleapis.com/auth/youtube.force-ssl` → tick its checkbox → **Update** → **Save** at the bottom of the page. Without **Save** nothing is stored; check it is listed under *sensitive scopes*.
4. **Clients** → **Create client** → type **Desktop app**, name `soundsync-cli` → **Create**.
5. Copy **Client ID** and **Client secret** right away (the secret is shown only once) to `.env` → `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`.

## 4. Check

| Where | Expected |
|---|---|
| Spotify dashboard → app | Development mode, Web API, redirect `http://127.0.0.1:8888/callback` |
| `/apis/api/youtube.googleapis.com` | Status: Enabled |
| `/auth/audience` | Testing, External, your Gmail as test user |
| `/auth/scopes` | Only `youtube.force-ssl`, under sensitive |
| `/auth/clients` | Desktop client; ID prefix matches `.env` |

Final proof is the first login (todo 1.2 / 1.3). Typical Google errors:

| Error | Cause |
|---|---|
| `access_denied` | Account is not a test user |
| `invalid_client` | Wrong client type or ID |
| `accessNotConfigured` (403) | YouTube Data API not enabled |

Spotify returns 403 when the user is not on the app allowlist or the owner's Premium lapsed.
