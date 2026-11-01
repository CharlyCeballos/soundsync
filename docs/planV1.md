# Soundsync — Plan v1

Documento congelado. No se edita: cambios de alcance van a `planV2.md` (o posterior). El tracking vivo está en `todo.md`.

Fecha de cierre: 2026-10-23.

## 1. Qué es

App **standalone, gratuita, open source** para transferir playlists **Spotify → YouTube Music**.

No hay servidor propio. No hay cuentas propias. No hay pagos. El usuario corre un binario local.

Referencia de producto (pagos, no clonar negocio): TuneMyMusic, Soundiiz. Documentación Google del flujo oficial de transferencia: [Transfer your playlists from another service](https://support.google.com/youtubemusic/answer/14729358).

## 2. Destino real (restricción de APIs)

No existe API pública oficial de YouTube Music para playlists nativas del catálogo YTM.

v1 usa **solo APIs oficiales**:

- Spotify Web API (lectura de playlists y tracks; ISRC cuando venga).
- YouTube Data API v3 (`playlists.insert`, `playlistItems.insert` / `delete` / `list`, `search.list`).

Eso crea **playlists de YouTube** (videos). Aparecen en Library de YouTube Music. El usuario aceptó este destino.

Matching no puede filtrar ISRC de forma fiable en Data API. Búsqueda = título + artista de metadata Spotify original; preferir video oficial (canal Topic / VEVO); si no hay, **primer resultado**. Miss = skip + reporte.

`ytmusicapi`, InnerTube y cualquier cliente no oficial: **fuera de v1**.

## 3. Alcance v1 vs después

| Tema | v1 | Después |
|---|---|---|
| Pares | Solo Spotify → YouTube | Más orígenes/destinos |
| Contenido | Playlists (incluye colaborativas Spotify: snapshot, playlist YouTube **tuya**) | Liked songs, albums, artists, library |
| Sync | Re-transfer **idempotente** (ver §6): no duplicar playlists ni tracks; append faltantes; borrar en destino lo que ya no está en origen; reintentar unmatched | Sync periódico; **sync bidireccional** |
| UX | CLI primero, luego GUI | Mobile no |
| Distribución | Windows + Debian y derivados | Mac u otras distros si hace falta |
| Audiencia | Hobby / testers: Spotify Development Mode (~25 users), Google OAuth Testing (~100 testers, refresh ~7 días) | Verificación OAuth / Quota Extension si se sale de testers |
| Quota | Un proyecto Google del autor + **tope diario** | Evolucionar (BYO keys, etc.) |

Podcasts: no (Google no los importa a YTM). Extensión de browser: no. Android / stores: no. Auto-update: no. Pagos / cloud / DB de servidor: no.

## 4. Decisiones de producto (lock)

- Nombre: **Soundsync**.
- Open source (GitHub).
- Idiomas UI: **español MX** e **inglés**.
- Privacidad default de playlist YouTube: **`private`**. Flag CLI `--privacy private|unlisted|public`.
- Orden de tracks: **preservar** el de Spotify. Huecos por skip no se rellenan con otra canción. Si el **set** ya coincide, **no reordenar** el destino.
- Progreso en vivo + reporte final (matched / failed / skipped / already in sync).
- Reporte CSV/JSON: **descargable on demand** (el usuario pide guardar a disco). Tokens nunca a disco.
- Tokens OAuth: RAM; borrar al salir.
- Auth: OAuth **PKCE** Spotify + Google. Redirect loopback `http://127.0.0.1:<port>/callback`. Sin email/password propio.
- YouTube Music: cuenta **free**.
- Sin límite por usuario de producto; el límite real es **quota diaria del proyecto Google** (sobre todo `search.list` = 100 units; default ~10 000 units/día).
- Code signing: no (`.msi` unsigned → SmartScreen). Certificado de pago: no.
- Client IDs: env / archivo local gitignored. En binario público el Client ID se puede extraer (cliente público + PKCE; el secret no se trata como secreto).

## 5. Identidad playlist Spotify ↔ YouTube

Estrategia **C**:

1. Marker en la **descripción** de la playlist YouTube, p. ej. `soundsync:spotify:<SPOTIFY_PLAYLIST_ID>`.
2. Cache local opcional (`%APPDATA%/soundsync/map.json` en Windows; XDG en Linux). El **marker manda**; el archivo es atajo.

Reglas:

- Marker intacto y usuario **renombró** la playlist YouTube: se sigue esa **playlist ID**.
- Usuario **borró** la playlist YouTube: se **crea de nuevo** (destino muerto, no es duplicado).
- Primera vez, sin marker ni mapa, pero existe YouTube con **el mismo título**: **preguntar**. Si confirma, **combinar** (adoptar esa playlist y fusionar tracks, sin duplicar videoIds).
- Combinar = una sola playlist destino; nunca crear una segunda con el mismo propósito.

## 6. Algoritmo de transferencia (idempotencia)

Propósito: **nunca duplicar playlists ni canciones dentro de una playlist**.

Identidad de canción en destino = **`videoId`**. Dos tracks Spotify distintos que matchean el mismo video = un solo ítem.

Dentro de una corrida, si Spotify trae el mismo track dos veces: **dedupe**.

### 6.1 Resolver playlist destino

1. Buscar marker `soundsync:spotify:<id>` en playlists del usuario YouTube (y/o cache local).
2. Si existe y la playlist sigue viva → usar esa ID.
3. Si existía y fue borrada → crear nueva + escribir marker.
4. Si no hay marker: listar YouTube por **mismo nombre** → **preguntar** (usar y combinar / crear nueva).
5. Si crea nueva: título = nombre Spotify, privacy default private, descripción incluye marker.

### 6.2 Diff de tracks

Sea `S` el set de `videoId` obtenidos al matchear tracks Spotify (únicos, orden Spotify, skips fuera del set).  
Sea `Y` el set de `videoId` ya en la playlist YouTube.

- Si `S == Y` (igualdad de conjuntos, **no** de orden) → **cero writes**. Reporte: `already in sync`.
- Si hay `videoId` en `S` y no en `Y` → **append** solo esos (match + insert). Reintentar unmatched de corridas previas.
- Si hay `videoId` en `Y` y no en `S` → **borrar del destino** (sync unidireccional Spotify → YouTube en re-transfer; no es bidireccional).
- Mismo set, distinto orden → **no tocar orden**.
- `videoId` ya presente → skip insert.

Match por track:

1. Preferir oficial (Topic / VEVO) en resultados de search.
2. Si no hay oficial → primer resultado.
3. Si no hay resultado usable → skip + línea en reporte.
4. Metadata de búsqueda: original de Spotify (sin traducir).

### 6.3 Límites YouTube

Una playlist YouTube: máximo **5000** ítems. Si el origen excede: transferir hasta el tope, resto skip + reporte.

### 6.4 Quota

Contar units **antes** de cada `search.list`. Tope diario configurable. Al llegar al tope: parar limpio, reporte `quota`. No reintentos agresivos.

## 7. Stack

Motor **Go**, independiente de cualquier toolkit GUI.

```
soundsync/
  cmd/soundsync/        # CLI (Cobra)
  internal/oauth/
  internal/spotify/
  internal/youtube/
  internal/match/
  internal/transfer/
  internal/i18n/        # es-MX, en
  frontend/             # React — fase GUI
  docs/planV1.md
  docs/todo.md
```

Un solo Git repo (binario local, no dual-repo web).

| Fase | Pieza |
|---|---|
| 1–3 | Core Go + CLI |
| 4 | GUI **Wails v3 + React** (a 2026-10-23 Wails v3 es beta, p. ej. `v3.0.0-beta.34`, Go 1.26+). Core no se acopla a Wails. |
| Plan B GUI | Wails v2 si v3 falla en Debian |

No Electron. No Tauri (evitar reescribir el core en Rust). No app móvil v1.

CLI es fuente de verdad. GUI es cáscara.

## 8. Distribución v1

- Windows: `.msi` (unsigned) y/o exe en GitHub Releases.
- Debian y derivados: `.deb` amd64.
- GUI (fase 4): deps WebView2 (Windows) / WebKitGTK (Debian). CLI no las necesita.

## 9. Fases de trabajo

**Fase 0 — Cuentas.** Spotify Developer App (redirect loopback). Google Cloud: YouTube Data API, OAuth Desktop, consent Testing, testers. Client IDs fuera de git.

**Fase 1 — Spike CLI.** Login ambos. Una playlist, ~5 tracks. Medir match y quota real. Go/no-go.

**Fase 2 — CLI usable.** Multi-playlist, progreso, skip+reporte, `--privacy`, `--lang`, algoritmo §6, CSV on demand, tope diario. Tests de match con fixtures (CI sin pegarle a APIs).

**Fase 3 — Empaque CLI.** GitHub Actions: Windows amd64 + `.deb` amd64. README.

**Fase 4 — GUI.** Wails v3 + React. Mismos casos. `.msi` + `.deb` con WebView.

**Fase 5+ (fuera de este plan).** Más pares, liked/albums/artists, sync periódico, bidireccional (hay que definir ganador de conflictos), verificación Google/Spotify si audiencia pública.

## 10. Prioridad MVP (orden de valor)

1. Listar playlists Spotify.  
2. Crear o adoptar playlist en YouTube.  
3. Match + agregar tracks (sin duplicar).  
4. Reporte de fallos / already in sync.  
5. UI (después del CLI).

## 11. Riesgos

1. Quota de search: un proyecto compartido se agota fácil. Mitigación: tope diario, una search por track, stop early.
2. Google Testing: tokens ~7 días → re-login. Aceptable en hobby.
3. Covers y primer resultado: calidad de match imperfecta. Mitigación: reporte.
4. Wails v3 en Linux: riesgo de empaque. CLI permanece usable.
5. SmartScreen en MSI unsigned.
6. ToS Spotify/Google: riesgo aceptado por el autor.
7. Scopes YouTube sensibles: app pública real exige verificación + privacy policy en URL. v1 se queda en testers.

## 12. Seguridad (v1)

- PKCE. Loopback solo en localhost.
- No loguear access/refresh tokens.
- No persistir tokens.
- Marker en descripción es identificador de playlist, no secreto.
- `map.json` local: IDs de playlists, no tokens.
- Least privilege scopes: Spotify playlist-read; YouTube el mínimo que permita crear/editar playlists (`youtube` o `youtube.force-ssl` según lo que exija insert).

## 13. i18n y UX CLI (mínimo)

- `--lang es|en` (default: locale del OS si se detecta, si no `es`).
- Prompts: combinar playlist mismo nombre; re-transfer implícito por §6 (no preguntar “¿duplicar?”; el diff decide). Preguntar solo cuando hay ambigüedad de **identidad** (mismo título, sin marker).
- Reporte: matched, skipped, removed from destination, already in sync, quota.

## 14. Fuera de v1 (explícito)

Pagos, cuentas propias, hosting, DB servidor, telemetría, InnerTube, Apple Music, extensión browser, Android/iOS, auto-update, code signing de pago, reordenar destino cuando el set es igual, crear playlist nueva si el set ya está sincronizado.
