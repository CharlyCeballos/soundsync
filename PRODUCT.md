# Product
<!-- impeccable:product-schema 1 -->

> Written 2026-11-02 without the usual interview: the author asked Claude to settle open questions on its own. Facts come from the repository (`docs/planV1.md`, `planV2.md`, `compliance.md`, `README.md`); lines marked *(inferred)* are Claude's reading and need the author's confirmation.

## Platform
web

The GUI is a Wails v3 desktop window (WebView2 on Windows 11, WebKitGTK on Debian 13) rendering a React page. Desktop window sizes only: 900×640 minimum.

## Users
A person who keeps their music library in Spotify and wants the same playlists on YouTube / YouTube Music. They brought their own Spotify app and Google OAuth client (a one-time, technical setup guided by `docs/setup.md`); after that they use the app occasionally, a few playlists at a time. *(inferred)* Comfortable with a desktop app, not with terminals: the GUI exists for people who would not use the CLI.

## Product Purpose
Copy the user's own Spotify playlists into YouTube playlists on their channel, and keep them in step on later runs (add what is new, remove what is gone, never duplicate, respect videos the user replaced by hand). Success: the playlist plays in YouTube Music with the right recordings, with no surprises on the user's channel.

## Positioning
Free, personal, open source (MIT), no account with Soundsync: it runs on the user's computer with the user's own API clients, keeps tokens in memory only and stores nothing from Spotify or YouTube. It prefers the artist's official audio ("- Topic") over music videos.

## Operating Context
- Quotas shape the work: about 100 YouTube searches a day, so a big library takes days; re-running costs no searches for tracks already copied.
- Every write to YouTube is shown first and needs the user's explicit OK (YouTube API Policies III.C.2).
- Logins happen in the system browser; the app waits and comes back to the front.
- Spanish (es-MX) and English, following the system language.

## Capabilities and Constraints
- Same engine as the CLI (`internal/copier`); the GUI adds no rules of its own.
- Spotify Development Mode: only playlists the user owns or collaborates on can be read; followed playlists of others are shown but cannot be chosen.
- A YouTube playlist holds at most 5,000 videos.
- Brand rules (`docs/compliance.md`): the Spotify logo and the YouTube logo never share a view; Spotify content is never seated next to YouTube content; every Spotify item links back ("OPEN SPOTIFY"); no cover art, no playback, no previews.
- No remote resources in the page (strict CSP); the page never navigates away; links open in the system browser through an allowlist.

## Brand Commitments
- Name: Soundsync. Never "Spotify" or "YouTube" in the name or the icon.
- Icon: indigo-to-violet rounded square with a white list whose middle line becomes a chevron (`gui/build/appicon.svg`). No green, no circle with waves, no play triangle.
- Voice *(inferred)*: plain, calm, specific. Says what will happen before it happens; errors say what went wrong and what to do.

## Evidence on Hand
- Official Spotify logos (2024 kit) and YouTube "Developed with YouTube" artwork, used under each platform's terms; they are not covered by the MIT license.
- No testimonials, metrics or user research exist; do not invent any.

## Product Principles
1. Nothing happens on the user's channel without them seeing exactly what and saying yes.
2. The user's own changes win: kept videos, chosen titles, privacy of existing playlists.
3. Spend quota like it is scarce, and say how much each step costs.
4. Keep each platform's content and brand in its own space.
5. Every screen has one obvious next action.

## Accessibility & Inclusion
*(inferred)* WCAG 2.2 AA contrast, full keyboard use with visible focus, screen-reader labels on icon-only controls, reduced-motion respected.
