# Soundsync — platform rules (Spotify + YouTube)

Reviewed 2026-10-26. Source for `planV2.md` (todo 1.8) and the README.
Clause numbers point at the documents below; re-check them if a version date changes.
Not legal advice.

| Document | Version reviewed |
|---|---|
| Spotify Developer Terms | v10, effective 2025-05-15 |
| Spotify Developer Policy | effective 2025-05-15 |
| Spotify Design & Branding Guidelines | page as of 2026-10-26 |
| Spotify quota modes + "February 2026 Dev Mode changes" | page as of 2026-10-26 |
| YouTube API Services Terms of Service | last updated 2026-09-14 |
| YouTube API Services Developer Policies | last updated 2026-10-15 |

Not reviewed yet: Spotify Compliance Tips, YouTube Branding Guidelines, EU User Consent Policy, Google Software Principles.

## Verdict

Allowed. Spotify Developer Policy III.9 bans transferring data to another service **except** to let "a user transfer their personal data, or the metadata of the user's playlists to another service". That is exactly Soundsync. YouTube has no rule against metadata from other services as long as it is not presented as YouTube search results.

## Spotify

### Access (Dev Mode, since 2026-03-09)

- Max **5 users** per app, each on the app allowlist; others get 403.
- App owner needs **active Premium**; if it lapses the app stops working.
- Extended quota: organizations only, ≥250k MAU (since 2025-05-15). Soundsync stays in Dev Mode for good.
- Refresh token lifetime shown in dashboard: 180 days.

### API changes that affect code (Feb 2026)

- Use `/playlists/{id}/items`, not `/tracks`. JSON: `tracks` → `items`, `tracks.items[].track` → `items.items[].item`.
- Items readable only for playlists the user **owns or collaborates on**. Followed playlists of others → skip + report.
- `GET /me/playlists` still works. `external_ids` (ISRC) was removed then restored (March 2026).
- Search `limit` max 10 (not used by Soundsync, noted for reference).

### Rules

| Rule | Source | Impact |
|---|---|---|
| Private, personal, non-commercial use | Terms III.1.1, Policy III.10, IV.2 | Free app; no ads, no paid tier |
| Request only needed data | Terms V.3, Policy I.1.1 | Scopes: `playlist-read-private`, `playlist-read-collaborative` only |
| No ML training / no analysis of Spotify Content | Policy III.13–III.14 | Matching is search, no metrics derived |
| No integration with streams of another service | Policy III.5 | No playback, no previews in GUI |
| No mimicking Spotify core experience | Policy III.11 | Not a player |
| Do not store Spotify Content indefinitely; cache only if strictly needed | Terms IV.3.1.1, IV.3.2 | No local cache (`map.json` dropped) |
| Delete user data on disconnect, ≤5 days | Terms V.8, Appendix A.5(c) | Nothing stored on disk; tokens RAM only |
| Security Codes (Client ID) per app, not disclosed to third parties | Terms VI.1.2–VI.1.3 | Bring-your-own Client ID; never embedded in public binaries |
| Attribute Spotify content; link back to Spotify | Policy II.4.1–II.4.2 | Report: “Data from Spotify” + `open.spotify.com` link per track |
| App name must not start with “Spot” or include “Spotify”; no implied endorsement | Policy II.1, VI.2; Terms IX.7 | “Soundsync” OK |
| Permissions unused 90 days may be suspended | Terms IX.8.5 | Expect re-enabling after long idle |

### Design Guidelines (GUI, phase 4)

- Attribute with the official logo: full logo ≥70px, icon alone ≥21px (only if no room). Exclusion zone = ½ icon height. Green logo only on black/white; monochrome otherwise. Never modify it.
- Link back on every playlist/track; text `OPEN SPOTIFY` (or `GET SPOTIFY FREE` if not installed).
- Metadata exactly as provided; truncation OK only if the full text is reachable (tooltip). Layout fits playlist 25 / artist 18 / track 23 chars.
- Cover art: untouched, no crop, no overlays, rounded 4px (small/medium) or 8px (large). Soundsync v1 shows no cover art.
- No co-branding: never show Spotify and YouTube logos together; Spotify content never seated next to content from similar services → Spotify selection view and YouTube result view are separate.
- Soundsync's own logo must not look like Spotify's (no green circle with waves).

## YouTube

### Rules

| Rule | Source | Impact |
|---|---|---|
| No API credentials in open source projects; one API project per client; never share credentials | Policies III.D.1 | Bring-your-own OAuth client; `.env` gitignored |
| Published privacy policy, accepted by the user before use, always reachable | ToS §7, Policies III.A.2 | `PRIVACY.md` from v1 (todo D.4, 2.15) |
| Link YouTube ToS; own terms say users are bound by it | Policies III.A.1 | First-run notice (todo 2.15) |
| Privacy policy must say it uses YouTube API Services, link Google Privacy Policy, explain revocation (myaccount.google.com/permissions) | Policies III.A.2 | Content of `PRIVACY.md` |
| Actions on YouTube clearly initiated by the user, consent before execution, show which channel | Policies III.C.2, III.E.3.d–e | Show channel + summary (create / +N / −M), confirm; `--yes` to skip (todo 2.5) |
| Do not change playlist privacy unless the user says so | Policies III.E.3.f | Privacy only set on create (`--privacy`, default private) |
| Authorized data stored ≤30 days, then delete or refresh | Policies III.E.4.c | No local cache of YouTube IDs; marker in playlist description is the identity |
| Revoke token when the user disconnects; delete data ≤7 days | Policies III.D.2, III.E.4.g | Call Google `revoke` at end of run (todo 2.14) |
| Do not alter search results or mix other sources into them | Policies III.C.5 | Picking one result is fine; never present Spotify data as YouTube results |
| Make clear YouTube is the source of YouTube content | Policies III.F.2 | Report: “Data from YouTube” + video link |
| No derived metrics from API data | Policies III.E.4.h | Match ranking stays internal |
| No scraping, no undocumented APIs | Policies III.E.6, III.D.7 | Data API only |
| Installed apps must be updatable to current API versions | Policies III.B.1 | Conflicts with V1 “no auto-update”; decide in planV2 (update notice likely enough) |
| Quota: default 10,000 units/day for all endpoints except search; `search.list` = 1 unit **plus** its own bucket of 100 calls/day (overview updated 2026-10-15; before it cost 100 units of the shared pool) | ToS §15, Policies III.D.3, Data API overview | ~100 tracks/day, limited by searches; inserts cost 50 units each (~199/day); extension needs a compliance audit |
| Projects idle 90 days may be curtailed | Policies III.D.4 | Expect re-enabling after long idle |
| No public statements implying YouTube endorsement | ToS §13 | README wording |

### Google OAuth facts

- Consent screen in **Testing**: max 100 test users; refresh tokens expire after ~7 days.
- Desktop client secret is not confidential, but it is shown **only once** at creation; whether the token endpoint requires it with PKCE is checked in todo 1.3.
- Desktop clients accept any loopback port; no redirect URI registration.

## Accepted risks

- The CSV/JSON report pairs a Spotify track with a YouTube video. That pairing is the transfer allowed by Spotify Policy III.9; keep it plain text, no logos or thumbnails, saved only when the user asks.
- “Spotify → YouTube” as a description pairs both brands. Keep it textual, never with logos.
