package i18n

// es holds the es-MX translations, keyed by the English text. Keep the fmt
// verbs in the same order (or use explicit indexes such as %[2]d); a test
// in cmd/soundsync checks that every wrapped message has an entry here.
var es = map[string]string{
	// app.go
	"%w\nrun 'soundsync config check' for details":                                                                                                 "%w\ncorre 'soundsync config check' para ver detalles",
	"warning: %v; revoke it manually at https://myaccount.google.com/permissions\n":                                                                "aviso: %v; revócalo a mano en https://myaccount.google.com/permissions\n",
	"Google access revoked. No Spotify or YouTube data was kept on disk, except reports you asked for.":                                            "Acceso a Google revocado. No se guardaron datos de Spotify ni de YouTube en disco, salvo los reportes que pediste.",
	"%w\nhint: add this Spotify account under User Management in your app's dashboard, and check that the app owner's Premium is active":           "%w\nsugerencia: agrega esta cuenta de Spotify en User Management del dashboard de tu app y revisa que el Premium del dueño de la app esté activo",
	"%w\nhint: create a channel at https://www.youtube.com/create_channel, or log in with another Google account":                                  "%w\nsugerencia: crea un canal en https://www.youtube.com/create_channel o entra con otra cuenta de Google",
	"%w\nhint: enable the YouTube Data API v3 in your Google Cloud project (docs/setup.md, step 2)":                                                "%w\nsugerencia: habilita la YouTube Data API v3 en tu proyecto de Google Cloud (docs/setup.md, paso 2)",
	"Quota used: 1 unit (channels.list).":                                                                                                          "Quota usada: 1 unidad (channels.list).",
	"Opening your browser to log in to %s.\nIf it does not open, paste this URL into it:\n\n  %s\n\nWaiting for the login (Ctrl+C to cancel)...\n": "Abriendo tu navegador para entrar a %s.\nSi no se abre, pega esta URL en él:\n\n  %s\n\nEsperando el login (Ctrl+C para cancelar)...\n",
	"Could not open the browser automatically: %v\n":                                                                                               "No se pudo abrir el navegador automáticamente: %v\n",
	"note: ": "nota: ",
	"%s login was cancelled or denied in the browser": "el login de %s se canceló o se negó en el navegador",
	"%s login timed out after %s":                     "el login de %s expiró después de %s",
	"%s login cancelled":                              "login de %s cancelado",
	"%w\nhint: the client ID or secret in .env does not match the OAuth client in the console": "%w\nsugerencia: el client ID o el secret de .env no coinciden con el cliente OAuth de la consola",
	"%s [y/N]: ":     "%s [s/N]: ",
	"Go ahead?":      "¿Continuar?",
	"Do you accept?": "¿Aceptas?",

	// config_cmd.go
	"Inspect local configuration":                          "Revisar la configuración local",
	"Validate the .env file without showing secret values": "Validar el archivo .env sin mostrar valores secretos",

	"Config: %s\n": "Configuración: %s\n",
	"Soundsync looks for .env in the current folder, then in %s; --env-file picks another file.\n": "Soundsync busca .env en la carpeta actual y luego en %s; --env-file elige otro archivo.\n",
	"Problems:":                "Problemas:",
	"configuration is invalid": "la configuración no es válida",
	"OK":                       "OK",
	"missing":                  "falta",
	"set":                      "definido",
	"invalid":                  "inválido",

	// internal/copier
	"that playlist is not one of the playlists offered for this Spotify playlist": "esa playlist no es una de las que se ofrecieron para esta playlist de Spotify",

	// internal/desktop (GUI)
	"Soundsync is still working on the previous step; wait for it to finish or stop it":           "Soundsync sigue trabajando en el paso anterior; espera a que termine o detenlo",
	"this system has no folder for app settings, so Soundsync cannot find its configuration file": "este sistema no tiene carpeta para la configuración de apps, así que Soundsync no puede encontrar su archivo de configuración",
	"connect Spotify first":                "primero conecta Spotify",
	"connect YouTube first":                "primero conecta YouTube",
	"choose at least one Spotify playlist": "elige al menos una playlist de Spotify",
	"choose your playlists again: the previous choice is no longer current": "vuelve a elegir tus playlists: la elección anterior ya no está vigente",
	"look for matches first": "primero busca las coincidencias",
	"the search stopped before every playlist was checked; look for matches again":                                                "la búsqueda se detuvo antes de revisar todas las playlists; vuelve a buscar las coincidencias",
	"this plan was already written; look for matches again to copy more":                                                          "este plan ya se escribió; vuelve a buscar coincidencias para copiar más",
	"there is no copy to report yet":                                                                                              "todavía no hay una copia para reportar",
	"Soundsync only opens links to Spotify, YouTube, Google and its own page":                                                     "Soundsync solo abre enlaces de Spotify, YouTube, Google y su propia página",
	"the configuration file already exists; open its folder to edit it":                                                           "el archivo de configuración ya existe; abre su carpeta para editarlo",
	"choose private, unlisted or public for new playlists":                                                                        "elige privada, no listada o pública para las playlists nuevas",
	"choose CSV or JSON for the report":                                                                                           "elige CSV o JSON para el reporte",
	"playlist %s is not in your Spotify library; refresh the list and choose again":                                               "la playlist %s no está en tu biblioteca de Spotify; actualiza la lista y vuelve a elegir",
	"%q: choose whether to use your existing playlist or create a new one":                                                        "%q: elige si usas tu playlist existente o creas una nueva",
	"%q already exists; choose another name (Soundsync never overwrites files)":                                                   "%q ya existe; elige otro nombre (Soundsync nunca sobrescribe archivos)",
	"YouTube's daily quota is used up; it resets at midnight Pacific time. Copy again then: Soundsync only adds what is missing.": "Se agotó la quota diaria de YouTube; se reinicia a medianoche, hora del Pacífico. Copia otra vez entonces: Soundsync solo agrega lo que falta.",
	"Save the Soundsync report":                                                                                                   "Guardar el reporte de Soundsync",
	"%w\nrevoke it by hand at https://myaccount.google.com/permissions":                                                           "%w\nrevócalo a mano en https://myaccount.google.com/permissions",

	// destinations.go
	"warning: %q (%s) carries the same Soundsync marker as another playlist; only the first one is used.\n":                                  "aviso: %q (%s) tiene el mismo marcador de Soundsync que otra playlist; solo se usa la primera.\n",
	"your channel already has a playlist called %s that Soundsync did not create; run the command without --yes to choose whether to use it": "tu canal ya tiene una playlist llamada %s que Soundsync no creó; corre el comando sin --yes para elegir si la usas",
	"\n%q: your channel already has %d playlist(s) with this title that Soundsync did not create:\n":                                         "\n%q: tu canal ya tiene %d playlist(s) con este título que Soundsync no creó:\n",
	"  %d. %q (%s, %d videos) %s\n": "  %d. %q (%s, %d videos) %s\n",
	"Use one? Soundsync adds a line to its description and copies the missing tracks into it; the videos already there stay.": "¿Usar una? Soundsync agrega una línea a su descripción y copia ahí los tracks que falten; los videos que ya tiene se quedan.",
	"Type its number (%s), or press Enter for a new playlist: ":                                                               "Escribe su número (%s) o presiona Enter para una playlist nueva: ",
	"A new playlist will be created.": "Se creará una playlist nueva.",
	"create it (%s)":                  "crearla (%s)",
	"use the existing one and add the Soundsync line to its description": "usar la existente y agregar la línea de Soundsync a su descripción",

	// login.go
	"Log in to a service and show the connected account (token kept in memory only)": "Entrar a un servicio y mostrar la cuenta conectada (token solo en memoria)",
	"Log in to Spotify and show the connected account":                               "Entrar a Spotify y mostrar la cuenta conectada",
	"Log in to Google and show the YouTube channel (token revoked on exit)":          "Entrar a Google y mostrar el canal de YouTube (el token se revoca al salir)",
	"Logged in to Spotify as %s (user id %s). The token stays in memory only.\n":     "Sesión de Spotify iniciada como %s (user id %s). El token queda solo en memoria.\n",
	"Logged in to YouTube. Playlists would be created on channel %q (id %s).\n":      "Sesión de YouTube iniciada. Las playlists se crearían en el canal %q (id %s).\n",

	// privacy.go
	"accept the privacy policy to let Soundsync connect to Spotify and YouTube":        "acepta la política de privacidad para que Soundsync se conecte a Spotify y YouTube",
	"note: no config folder found; Soundsync will ask again next time.":                "nota: no se encontró carpeta de configuración; Soundsync volverá a preguntar la próxima vez.",
	"note: could not remember your answer (%v); Soundsync will ask again next time.\n": "nota: no se pudo guardar tu respuesta (%v); Soundsync volverá a preguntar la próxima vez.\n",
	`
Before Soundsync connects to Spotify and YouTube, please read how it handles your data:
  Privacy policy:           https://github.com/CharlyCeballos/soundsync/blob/main/PRIVACY.md
  YouTube Terms of Service: https://www.youtube.com/t/terms
  Google Privacy Policy:    https://policies.google.com/privacy
Soundsync uses YouTube API Services: by continuing you agree to the YouTube Terms of Service.
Nothing from Spotify or YouTube is saved on this computer. Revoke Google access any time at
https://myaccount.google.com/permissions.
`: `
Antes de que Soundsync se conecte a Spotify y YouTube, lee cómo maneja tus datos:
  Política de privacidad:            https://github.com/CharlyCeballos/soundsync/blob/main/PRIVACY.md
  Términos de Servicio de YouTube:   https://www.youtube.com/t/terms
  Política de Privacidad de Google:  https://policies.google.com/privacy
Soundsync usa YouTube API Services: al continuar aceptas los Términos de Servicio de YouTube.
No se guarda nada de Spotify ni de YouTube en esta computadora. Revoca el acceso de Google cuando
quieras en https://myaccount.google.com/permissions.
`,

	// report.go
	"--report %q: use a .csv or .json file name":                                              "--report %q: usa un nombre de archivo .csv o .json",
	"--report %q already exists; choose another file name (Soundsync never overwrites files)": "--report %q ya existe; elige otro nombre (Soundsync nunca sobreescribe archivos)",
	"report not saved: %q already exists":                                                     "reporte no guardado: %q ya existe",
	"report not saved: %w":                                                                    "reporte no guardado: %w",
	"Report saved to %s (%d rows).\n":                                                         "Reporte guardado en %s (%d filas).\n",

	// root.go, shell.go, sources.go, version.go
	"Copy Spotify playlists to YouTube": "Copiar playlists de Spotify a YouTube",
	"Soundsync copies your Spotify playlists to YouTube playlists (visible in YouTube Music).\nTokens stay in memory and are never written to disk.": "Soundsync copia tus playlists de Spotify a playlists de YouTube (visibles en YouTube Music).\nLos tokens se quedan en memoria y nunca se escriben a disco.",
	"path to the local .env file with your OAuth client IDs":                                              "ruta al archivo .env local con tus client IDs de OAuth",
	"language: es or en (default: the system's, else es)":                                                 "idioma: es o en (por defecto: el del sistema; si no, es)",
	"Run several commands with one login per service (tokens kept in memory until exit)":                  "Correr varios comandos con un login por servicio (tokens en memoria hasta salir)",
	"Soundsync shell. Each service asks for a browser login on first use;":                                "Shell de Soundsync. Cada servicio pide login en el navegador la primera vez;",
	"tokens stay in memory until you type 'exit'. Type 'help' for commands.":                              "los tokens se quedan en memoria hasta que escribas 'exit'. Escribe 'help' para ver los comandos.",
	"\n(type 'exit' to quit)\n":                                                                           "\n(escribe 'exit' para salir)\n",
	"Error: %v\n":                                                                                         "Error: %v\n",
	"Error: already in the shell":                                                                         "Error: ya estás en el shell",
	"unclosed quote":                                                                                      "comilla sin cerrar",
	"%q is not in your Spotify library; run 'spotify playlists' to see them":                              "%q no está en tu biblioteca de Spotify; corre 'spotify playlists' para verlas",
	"%q belongs to another user: Spotify does not let Development Mode apps read it":                      "%q es de otro usuario: Spotify no deja que las apps en Development Mode la lean",
	"\ntip: each argument is one playlist; quote names with spaces, e.g. \"The Way to My Favorite Part\"": "\ntip: cada argumento es una playlist; pon entre comillas los nombres con espacios, p. ej. \"The Way to My Favorite Part\"",
	"Print the Soundsync version":                                                                         "Mostrar la versión de Soundsync",

	// update.go
	"could not check for a newer version: %v\n":              "no se pudo revisar si hay una versión nueva: %v\n",
	"A newer Soundsync is available: %s (you have %s): %s\n": "Hay una versión nueva de Soundsync: %s (tienes %s): %s\n",

	// spotify_cmd.go
	"Read your Spotify playlists (token kept in memory only)":                                                                           "Leer tus playlists de Spotify (token solo en memoria)",
	"List the playlists in your Spotify library":                                                                                        "Listar las playlists de tu biblioteca de Spotify",
	"List the tracks of one playlist: name, ID, open.spotify.com link or spotify: URI":                                                  "Listar los tracks de una playlist: nombre, ID, link de open.spotify.com o URI spotify:",
	"%w\nhint: Spotify only lets Development Mode apps read playlists you own or collaborate on":                                        "%w\nsugerencia: Spotify solo deja que las apps en Development Mode lean playlists tuyas o colaborativas",
	"%q matches %d playlists; use a longer name or the link:":                                                                           "%q coincide con %d playlists; usa un nombre más largo o el link:",
	"no playlist in your library matches %q; run 'spotify playlists' to see them":                                                       "ninguna playlist de tu biblioteca coincide con %q; corre 'spotify playlists' para verlas",
	"\nPlaylists in your Spotify library: %d\n\n":                                                                                       "\nPlaylists en tu biblioteca de Spotify: %d\n\n",
	"  #\tNAME\tTRACKS\tACCESS\tLINK":                                                                                                   "  #\tNOMBRE\tTRACKS\tACCESO\tLINK",
	"\n%d followed playlist(s) belong to other users: Spotify does not let Development Mode apps read them, so they will be skipped.\n": "\n%d playlist(s) seguidas son de otros usuarios: Spotify no deja que las apps en Development Mode las lean, así que se omitirán.\n",
	"\nPlaylist %s: %d entries\n\n":                                                                                                     "\nPlaylist %s: %d entradas\n\n",
	"  %d\t(skipped: %s)\t\t\n":                                                                                                         "  %d\t(omitido: %s)\t\t\n",
	"\nTracks: %d (local files: %d, without ISRC: %d). Skipped: %d episode(s), %d unavailable.\n":                                       "\nTracks: %d (archivos locales: %d, sin ISRC: %d). Omitidos: %d episodio(s), %d no disponibles.\n",
	"%q is not an open.spotify.com playlist link":                                                                                       "%q no es un link de playlist de open.spotify.com",
	"%q is not a Spotify playlist ID, link or URI":                                                                                      "%q no es un ID, link ni URI de playlist de Spotify",

	// youtube_add.go
	"Copy Spotify playlists to YouTube: create or reuse each one, add what is missing, remove what is gone; several run in parallel": "Copiar playlists de Spotify a YouTube: crea o reutiliza cada una, agrega lo que falta, quita lo que ya no está; varias corren en paralelo",
	"only look at the first N tracks of each playlist (0 = all)":                                                                     "ver solo los primeros N tracks de cada playlist (0 = todos)",
	"stop searching after N searches in this session; tracks left wait for the next run":                                             "dejar de buscar tras N búsquedas en esta sesión; los tracks restantes esperan a la siguiente corrida",
	"write without asking for confirmation":                                                                                          "escribir sin pedir confirmación",
	"privacy of playlists that get created: private, unlisted or public":                                                             "privacidad de las playlists que se creen: private, unlisted o public",
	"save a report of this run to a new .csv or .json file":                                                                          "guardar un reporte de esta corrida en un archivo .csv o .json nuevo",
	"--limit must be 0 (all tracks) or more, got %d":                                                                                 "--limit debe ser 0 (todos los tracks) o más; se recibió %d",
	"--max-searches must be 0 or more, got %d":                                                                                       "--max-searches debe ser 0 o más; se recibió %d",
	"invalid --privacy %q (want private, unlisted or public)":                                                                        "--privacy %q no es válido (usa private, unlisted o public)",
	"add %d video(s)":    "agregar %d video(s)",
	"remove %d video(s)": "quitar %d video(s)",
	"\nYouTube says today's searches are used up (they reset at midnight Pacific time). What was found so far can still be written.": "\nYouTube dice que las búsquedas de hoy se acabaron (se reinician a medianoche, hora del Pacífico). Lo encontrado hasta ahora sí se puede escribir.",
	"  stopped: %v\n":     "  detenido: %v\n",
	"\nNothing to write.": "\nNada que escribir.",
	"\nAbout to change %d playlist(s) on channel %q: %d units (used so far: %d of %d).\n": "\nSe van a cambiar %d playlist(s) en el canal %q: %d unidades (usadas hasta ahora: %d de %d).\n",
	"Cancelled. Nothing was changed.":      "Cancelado. No se cambió nada.",
	"\nSpotify → YouTube on channel %q:\n": "\nSpotify → YouTube en el canal %q:\n",
	"a new playlist":                       "una playlist nueva",
	"%q (%d videos)":                       "%q (%d videos)",
	"  %q → %s: %d track(s) to match\n":    "  %q → %s: %d track(s) por emparejar\n",
	"    note: only the first %d tracks: a YouTube playlist holds at most %d videos.\n":                  "    nota: solo los primeros %d tracks: una playlist de YouTube admite máximo %d videos.\n",
	"    note: only the first %d tracks (--limit).\n":                                                    "    nota: solo los primeros %d tracks (--limit).\n",
	"%d track(s) need no search: already in the YouTube playlist, or matched earlier in this session.\n": "%d track(s) no necesitan búsqueda: ya están en la playlist de YouTube o se emparejaron antes en esta sesión.\n",
	" in parallel": " en paralelo",
	"No searches left in this session (--max-searches): %d track(s) wait for the next run.\n":                               "No quedan búsquedas en esta sesión (--max-searches): %d track(s) esperan a la siguiente corrida.\n",
	"Searching %d of %d track(s)%s: %d search(es) left in this session (--max-searches); the rest wait for the next run.\n": "Buscando %d de %d track(s)%s: quedan %d búsqueda(s) en esta sesión (--max-searches); el resto espera a la siguiente corrida.\n",
	"Searching %d track(s)%s: %d of the %d daily searches.\n":                                                               "Buscando %d track(s)%s: %d de las %d búsquedas diarias.\n",
	"  created [%s] (%s) %s\n":                "  creada [%s] (%s) %s\n",
	"  marked [%s] as a Soundsync playlist\n": "  [%s] marcada como playlist de Soundsync\n",
	"add":                                "agregar",
	"already in playlist":                "ya está en la playlist",
	"same video as an earlier track":     "mismo video que un track anterior",
	"first":                              "primero",
	"playlist":                           "en playlist",
	"  #\tSPOTIFY\tYOUTUBE\tWHY\tACTION": "  #\tSPOTIFY\tYOUTUBE\tPOR QUÉ\tACCIÓN",
	"  %d\t%s\t%s %s\tyours\tkeep your video (Soundsync found %s)\n":                  "  %d\t%s\t%s %s\ttuyo\tse queda tu video (Soundsync encontró %s)\n",
	"  %d\t%s\t(no result)\t-\tskip\n":                                                "  %d\t%s\t(sin resultado)\t-\tomitir\n",
	"  %d track(s) not searched yet: no searches left; the next run picks them up.\n": "  %d track(s) sin buscar todavía: no quedan búsquedas; la siguiente corrida los toma.\n",
	"  %d match(es) left out: the playlist would pass YouTube's %d-video limit.\n":    "  %d match(es) fuera: la playlist pasaría el límite de %d videos de YouTube.\n",
	"  To remove from %q (no Spotify track matches them):\n":                          "  Por quitar de %q (ningún track de Spotify les corresponde):\n",
	"Summary:":   "Resumen:",
	" (stopped)": " (detenido)",
	" (in sync)": " (sincronizada)",
	"  %q%s: added %d, already there %d, removed %d, kept yours %d, no match %d, same video %d, not searchable %d.\n":                                 "  %q%s: agregados %d, ya estaban %d, quitados %d, tuyos conservados %d, sin match %d, mismo video %d, no buscables %d.\n",
	"    %d track(s) wait for the next run (no searches left).\n":                                                                                     "    %d track(s) esperan a la siguiente corrida (no quedan búsquedas).\n",
	"    Its other videos were kept. Later runs remove the ones no Spotify track matches, after asking.":                                              "    Sus otros videos se quedaron. Las siguientes corridas quitan los que no correspondan a ningún track de Spotify, después de preguntar.",
	"Quota used this session: %d of %d units, %d of %d searches.\n":                                                                                   "Quota usada en esta sesión: %d de %d unidades, %d de %d búsquedas.\n",
	"Data from Spotify. Data from YouTube.":                                                                                                           "Datos de Spotify. Datos de YouTube.",
	"stopped because another playlist reached the daily YouTube quota":                                                                                "detenido porque otra playlist alcanzó la quota diaria de YouTube",
	"%w\nhint: the daily YouTube quota is used up; it resets at midnight Pacific time. Run the same command again then: it only adds what is missing": "%w\nsugerencia: la quota diaria de YouTube se acabó; se reinicia a medianoche, hora del Pacífico. Corre el mismo comando entonces: solo agrega lo que falte",

	// youtube_cmd.go
	"Read and create playlists on your YouTube channel (token revoked on exit)":                     "Leer y crear playlists en tu canal de YouTube (el token se revoca al salir)",
	"List the playlists on your YouTube channel":                                                    "Listar las playlists de tu canal de YouTube",
	"Create empty YouTube playlists for Spotify playlists (reuses those Soundsync already created)": "Crear playlists vacías en YouTube para playlists de Spotify (reutiliza las que Soundsync ya creó)",
	"\nPlaylists on channel %q: %d\n\n":                                                             "\nPlaylists en el canal %q: %d\n\n",
	"  #\tTITLE\tVIDEOS\tPRIVACY\tSOUNDSYNC\tLINK":                                                  "  #\tTÍTULO\tVIDEOS\tPRIVACIDAD\tSOUNDSYNC\tLINK",
	"\n%s Quota used this session: %d units.\n":                                                     "\n%s Quota usada en esta sesión: %d unidades.\n",
	"privacy of new playlists: private, unlisted or public":                                         "privacidad de las playlists nuevas: private, unlisted o public",
	"create without asking for confirmation":                                                        "crear sin pedir confirmación",
	"\nYouTube channel: %q\n":                                                                       "\nCanal de YouTube: %q\n",
	"\nSpotify playlist: %q (%s)\n":                                                                 "\nPlaylist de Spotify: %q (%s)\n",
	"Already on YouTube as %q (%s, %d videos).\n  %s\n":                                             "Ya está en YouTube como %q (%s, %d videos).\n  %s\n",
	"Will use %q (%s, %d videos).\n":                                                                "Se usará %q (%s, %d videos).\n",
	"Will be created.":                                                                              "Se creará.",
	"\nNothing to create.":                                                                          "\nNada que crear.",
	"%s Quota used this session: %d units.\n":                                                       "%s Quota usada en esta sesión: %d unidades.\n",
	"\nAbout to change %d playlist(s) on channel %q: %d units (used so far: %d).\n":                 "\nSe van a cambiar %d playlist(s) en el canal %q: %d unidades (usadas hasta ahora: %d).\n",
	"\nCreated %q (%s).\n  %s\n  %s\n":                                                              "\nCreada %q (%s).\n  %s\n  %s\n",
	"\n%q now carries the Soundsync line; Soundsync will keep it in sync.\n  %s\n":                  "\n%q ya tiene la línea de Soundsync; Soundsync la mantendrá sincronizada.\n  %s\n",
}
