# Soundsync

[English](README.md)

Soundsync copia tus playlists de **Spotify a YouTube** (playlists de YouTube, así que también aparecen en YouTube Music). Es un programa de línea de comandos que corre en tu computadora: sin servidor, sin cuenta, sin guardar en disco nada de Spotify ni de YouTube.

Puedes correrlo de nuevo cuando quieras: agrega lo nuevo, quita lo que sacaste de la playlist de Spotify y deja en paz lo que ya está, incluidos los videos que cambiaste a mano.

Soundsync es un proyecto independiente. No está afiliado, respaldado ni patrocinado por Spotify, YouTube ni Google.

## Antes de empezar

Soundsync usa las APIs oficiales de Spotify y YouTube con **tus propias** credenciales de desarrollador, que configuras una sola vez:

- Una app de Spotify tuya. La cuenta dueña de la app necesita **Spotify Premium**, y la app se queda en Development Mode: hasta 5 cuentas de Spotify pueden usarla.
- Un proyecto de Google Cloud con la YouTube Data API y un cliente OAuth de tipo Desktop, en modo Testing con tu cuenta de Google como tester.
- Un canal de YouTube en esa cuenta de Google (si no tiene, Soundsync te dice dónde crearlo).

Guía paso a paso (en inglés): [docs/setup.md](docs/setup.md).

## Instalar

Baja la versión más reciente desde la [página de Releases](https://github.com/CharlyCeballos/soundsync/releases). `SHA256SUMS` trae el checksum de cada archivo.

**Windows (64 bits):** descomprime `soundsync_<versión>_windows_amd64.zip` en la carpeta que quieras y corre `soundsync.exe` desde una terminal (PowerShell o Símbolo del sistema) en esa carpeta. El programa no está firmado, así que SmartScreen de Windows puede avisar la primera vez: elige **Más información → Ejecutar de todas formas**.

**Debian, Ubuntu y derivados (64 bits):**

```bash
sudo apt install ./soundsync_<versión>_amd64.deb
```

Actualizaciones: al abrir `soundsync shell`, te avisa si salió una versión nueva y dónde bajarla. Nunca descarga ni instala nada por su cuenta.

## Configurar

Copia `.env.example` a un archivo llamado `.env` y llénalo con tus credenciales ([docs/setup.md](docs/setup.md) dice de dónde sale cada valor). El ejemplo viene en el zip de Windows y, con el paquete de Debian, en `/usr/share/doc/soundsync/env.example`.

Soundsync busca `.env` primero en la carpeta actual y luego en:

- Windows: `%APPDATA%\soundsync\.env`
- Linux: `~/.config/soundsync/.env`

`--env-file <ruta>` usa cualquier otro archivo. Revísalo con:

```bash
soundsync config check   # dice qué archivo leyó; nunca muestra los valores
```

Nunca compartas ni subas a git tu `.env`.

## Usar

```bash
soundsync shell
```

El shell mantiene tu sesión hasta que escribes `exit`: cada servicio pide login en el navegador una sola vez. Adentro:

| Comando | Qué hace |
|---|---|
| `spotify playlists` | Lista tu biblioteca de Spotify (propias, colaborativas, seguidas) |
| `spotify tracks <playlist>` | Lista las canciones de una playlist; acepta nombre, ID, link o URI `spotify:` |
| `youtube playlists` | Lista tus playlists de YouTube y cuáles creó Soundsync |
| `youtube add <playlist>... [-y] [--privacy private\|unlisted\|public] [--max-searches N] [--limit N] [--report archivo.csv\|.json]` | Copia playlists: crea o reusa cada playlist de YouTube, agrega lo que falta y quita los videos que ya no corresponden a ninguna canción de Spotify. Muestra todo y pregunta una vez antes de escribir; varias playlists corren en paralelo |
| `youtube create <playlist>... [-y] [--privacy …]` | Solo deja listas las playlists de YouTube, sin canciones |
| `login spotify` / `login youtube` | Entra y muestra la cuenta o el canal conectado |
| `exit` | Sale; se revoca el acceso a Google |

Cada argumento es una playlist; pon entre comillas los nombres con espacios: `youtube add "The Way to My Favorite Part" Covers`. Las playlists nuevas son privadas salvo que uses `--privacy`.

Los mensajes siguen el idioma de tu sistema cuando es español o inglés (si no, español); `--lang es` o `--lang en` elige uno, también dentro del shell.

**Matching.** Soundsync prefiere el canal `- Topic` del artista (audio del álbum), luego VEVO, luego el primer resultado de búsqueda. Una canción cuyo video ya está en la playlist (mismo título, artista y duración) no necesita búsqueda, así que una playlist larga sigue avanzando día con día. Si cambiaste un video a mano, Soundsync respeta el tuyo.

**Quota de YouTube.** Tu proyecto de Google Cloud tiene 100 búsquedas y 10,000 unidades por día; cada escritura (crear, agregar, quitar) cuesta 50 unidades. Eso da unas 100 canciones nuevas al día. Cuando se acaban las búsquedas, lo encontrado sí se escribe y lo demás espera a la siguiente corrida (`--max-searches` fija un presupuesto menor).

## Privacidad

Soundsync te pide aceptar su [política de privacidad](PRIVACY.md) (en inglés) una vez, antes del primer login. Los tokens de login viven solo en memoria y el acceso a Google se revoca cuando Soundsync termina. No se guarda nada de Spotify ni de YouTube en tu computadora.

## Scopes de OAuth

Registra estos y nada más.

| Plataforma | Scope | Para qué |
|---|---|---|
| Spotify | `playlist-read-private` | Listar tus playlists privadas y sus canciones |
| Spotify | `playlist-read-collaborative` | Listar playlists colaborativas |
| Google | `https://www.googleapis.com/auth/youtube.force-ssl` | Crear playlists, listar/agregar/borrar sus ítems, buscar videos |

Sin scopes de escritura en Spotify. Redirect URIs (loopback + PKCE): `http://127.0.0.1:8888/callback` para Spotify, `http://127.0.0.1:8889/callback` para Google.

## Compilar desde el código

Requiere Go 1.26 o más reciente.

```bash
go test ./...
go run ./cmd/soundsync shell
```

Los releases los arma GitHub Actions a partir de un tag `vX.Y.Z` (`.github/workflows/release.yml`). Reglas de las plataformas que dan forma a la app: [docs/compliance.md](docs/compliance.md). Avance y pendientes: [docs/todo.md](docs/todo.md).

## Licencia

[MIT](LICENSE)
