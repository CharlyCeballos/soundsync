# Soundsync — tracking

Único archivo de tracking. Los `planV<x>.md` no se mueven.

Estado: `todo` | `doing` | `done` | `blocked`  
Última actualización: 2026-10-30.

## Ahora

Fase 1 casi cerrada: 1.1–1.7 hechos, **GO** dado por el autor el 2026-10-30 (15/15 matches correctos). Sigue **1.8: escribir `planV2.md`** con el delta listado en esa fila. Opción A (caché de matches en memoria) elegida → 2.16. **No re-correr `youtube add` sobre "The Way to My Favorite Part" ni "Covers"** hasta resolver 2.5 / 2.17 (ver notas de 1.7). Module path provisional `soundsync` hasta D.1 (repo en la cuenta personal del autor).

### Para retomar

- Comandos del spike: `go run ./cmd/soundsync shell` y dentro `spotify playlists`, `spotify tracks <playlist>`, `youtube playlists`, `youtube create <playlist>... [-y] [--privacy]`, `youtube add <playlist>... [--limit N] [-y]`, `login spotify|youtube`, `config check`, `exit`. Cada argumento = una playlist; nombres con espacios entre comillas.
- Playlists ya copiadas en YouTube (private): "The Way to My Favorite Part" (2), "Tristes 😢😓" (6), "Covers" (7, con 1 corrección manual del autor).
- Quota usada el 2026-10-30: ~950 units y 30 de 100 búsquedas. Se reinicia a medianoche hora del Pacífico.

## Fase 0 — Cuentas

| ID | Item | Estado | Notas |
|---|---|---|---|
| 0.1 | Spotify Developer App + redirect `http://127.0.0.1` | done | Dev Mode: máx 5 users (allowlist), owner con Premium activo. Extended quota solo orgs ≥250k MAU → v1 queda en Dev Mode |
| 0.2 | Google Cloud project + YouTube Data API | done | Proyecto `soundsync`. Google bloquea login en navegador automatizado: consola Cloud siempre manual |
| 0.3 | OAuth client tipo Desktop + PKCE | done | Consent Testing; autor como tester. Client `soundsync-cli`; `GOOGLE_CLIENT_SECRET` en `.env` (Google lo muestra una sola vez; requerido en el token exchange aun con PKCE, ver 1.3). Credenciales nunca en el repo ni en binario público (YT Policies III.D.1) → cada usuario trae su propio OAuth client |
| 0.7 | Revisión YouTube API Services ToS (2026-09-14) + Developer Policies (2026-10-15) | done | Ajustes en 2.2, 2.5, 2.14, D.4, 1.8 |
| 0.4 | Scopes mínimos Spotify / YouTube documentados en README | done | |
| 0.5 | `.env.example` + gitignore de secretos/Client config | done | |
| 0.6 | Revisión Spotify Developer Terms v10 + Developer Policy (2025-05-15) | done | Caso de uso permitido: Policy III.9 (transferir metadata de playlists del usuario). Ajustes en 1.4, 2.10, 2.14, 3.3, 4.3 |

## Fase 1 — Spike CLI

| ID | Item | Estado | Notas |
|---|---|---|---|
| 1.1 | Módulo Go: `cmd/soundsync`, layout `internal/` | done | Core sin Wails. Go 1.27.3 (directiva `go 1.26.0`), Cobra v1.10.2, godotenv v1.5.1. `internal/config` valida `.env` (redirect loopback IP + puerto + path; `localhost` rechazado). `soundsync config check` muestra set/missing, nunca valores. Tests `-race` OK; build linux + windows/amd64. `go test` + `config check` OK en Windows del autor (2026-10-30) |
| 1.2 | OAuth PKCE Spotify (loopback) | done | Tokens solo RAM. `internal/oauth` genérico sobre `golang.org/x/oauth2` v0.37.0 (PKCE S256, state, listener antes del browser, client_id en body). `soundsync login spotify` + `GET /me` (403 → hint allowlist/Premium). Tests con proveedor falso que valida PKCE. 1er login real falló: un `/callback` sin `state` llegó antes que el redirect de Spotify y abortó la sesión → fix: callbacks con `state` ajeno se rechazan (400) sin terminar el login. Login real OK 2026-10-30 (cuenta del autor, scopes solo lectura); el `/callback` pelado se repitió y se ignoró |
| 1.3 | OAuth PKCE Google (loopback) | done | `GOOGLE_CLIENT_SECRET` obligatorio: Google lo exige a clientes Desktop aun con PKCE (`client_secret is missing`), confirmado en docs/foros. `soundsync login youtube`: `prompt=select_account`, muestra canal (`channels.list mine`, 1 unit; Policies III.C.2) y revoca al salir (adelanta 2.14). Hints: `accessNotConfigured`, sin canal, `invalid_client`. Login real OK 2026-10-30: exchange con PKCE + secret aceptado, `channels.list` 200, revoke OK. La cuenta usada no tenía canal YouTube (`ErrNoChannel`); el autor lo creó y `login youtube` lo confirmó el mismo día (canal "Carlos Ceballos") |
| 1.4 | Listar playlists Spotify (paginado) | done | Incluir colaborativas. Usar `/me/playlists` + `/playlists/{id}/items` (no `/tracks`, campo `items.items[].item`). Dev Mode solo lee ítems de playlists propias/colaborativas: seguidas ajenas → skip + reporte. Código: `MyPlaylists`, `PlaylistItems` (decodifica forma 2026 y legacy; episodios/null → skip; local files marcados), paginación solo sobre `api.spotify.com` (el token viaja en cada request). CLI `spotify playlists` / `spotify tracks <id|link|uri>` con atribución “Data from Spotify” + links. Corrida real OK 2026-10-30: 39 playlists (34 owned, 5 followed, 0 collaborative), totales con valor en todas las propias; 2 playlists leídas (6 y 2 tracks), 8/8 con ISRC. Pendiente UX: columnas se desalinean con emojis (tabwriter cuenta bytes/runas, no ancho visual). Nombre de playlist aceptado desde el refactor de `shell` |
| 1.5 | Crear playlist YouTube + marker `soundsync:spotify:<id>` | done | privacy default private. Código: `youtube create <playlist>` (`--privacy`, `--yes`) y `youtube playlists`. Busca el marker en `playlists.list` (1 unit/página): si existe, reusa y no pregunta; mismo título sin marker → aviso y crea aparte (adopción en 2.1). Muestra canal + plan y pide `y/N` antes de escribir (III.C.2); `playlists.insert` = 50 units (confirmado en docs). Contador de quota en `youtube.Client` (fallidas cuentan completo, conservador). Prueba real OK 2026-10-30: "The Way to My Favorite Part" creada private en canal "Carlos Ceballos"; 2º `create` la encontró por marker sin preguntar (idempotente); `youtube playlists` muestra el marker. Quota real = 54 units (1 channels + 3×1 playlists.list + 50 insert), igual a lo estimado |
| 1.6 | Search + insert ≤5 tracks | done | `playlistItems.insert` = 50 units por video, sin variante batch. **Cambio de quota de Google (overview 2026-10-15):** `search.list` ya no cuesta 100 units: cuesta 1 unit + 1 llamada de una cubeta propia de 100 búsquedas/día → techo ~100 tracks/día (antes ~65); `youtube.Client` cuenta las dos cubetas. Código: `youtube add <playlist>` (`--limit 5`, `--yes`): busca cada track (`match.Query` = 1er artista + título original; `match.Pick` = primer Topic/VEVO en ranking, si no primer resultado; 5 resultados por búsqueda, mismo costo), diff con `transfer.BuildPlan` (agrega faltantes en orden Spotify, dedupe por videoId, ya presentes = 0 writes, extras solo se reportan → 2.5), tabla de matches + `y/N`, para limpio en `quotaExceeded`. Requiere `youtube create` antes. Prueba real OK 2026-10-30 con "The Way to My Favorite Part": 2/2 matches correctos verificados por el autor escuchando (1 `vevo` = video oficial, 1 `topic` = audio); 2º `add` = in sync sin preguntar. Quota real = 109 units + 4 búsquedas, igual a lo estimado. Observación para 1.7: con ranking puro, VEVO (video musical, puede traer intro/outro) gana a Topic (audio de álbum) si aparece antes |
| 1.7 | Go/no-go por quota y calidad de match | done | 2026-10-30: autor decide **Topic > VEVO > primer resultado** (fidelidad al audio del álbum, sin agregados de video musical); `match.Pick` cambiado. Ojo: "The Way to My Favorite Part" tiene el VEVO de *The Way*; un nuevo `add` sumaría el Topic sin quitar el VEVO → no re-correr hasta 2.5. Muestra: "Tristes" (6, español/feat./Film Version) y "Covers" (7, peor caso), en paralelo en **una sola sesión** (decisión del autor: nada de dos terminales). `youtube create` y `youtube add` aceptan varias playlists (cada argumento = una playlist; nombres con espacios entre comillas), validan todo antes de gastar quota, una sola confirmación, `-y` como atajo. `add` procesa playlists en paralelo (goroutines; dentro de cada una, orden Spotify), tablas completas en orden de argumentos, progreso con prefijo `[playlist]`; quota agotada cancela el resto y se reporta la causa raíz. Tests estresados 300× con `-race`. **Corrida real 2026-10-30** (una sesión, 1 confirmación, inserts intercalados entre playlists y en orden dentro de cada una): 13 tracks → 7 `topic`, 2 `vevo`, 4 `first`. De los `first`: 2 canales oficiales sin marca VEVO/Topic (RHINO = sello, "Tenacious D" = artista), 1 subida de terceros (Jenni Rivera en "GPE CARO"), 1 sospechoso ("Mentiras" → clip de Prime Video con otro título). Covers: el riesgo "sale el original" no ocurrió (la query lleva al artista del cover). Quota real 768 units (estimado ~770) + 13 búsquedas; 2º `add` = 0 writes pero **13 búsquedas más** (re-correr cuesta búsquedas). Biblioteca propia ≈ 1331 tracks → ~14 días a 100 búsquedas/día. Calidad acumulada: **15/15 correctas** verificadas escuchando (2 de "The Way" + 13 de esta muestra, incluidos los 2 dudosos: Jenni Rivera en canal de terceros y "Mentiras" en Prime Video). Después el autor encontró el Topic de "Mentiras" (https://youtu.be/DZquZF0PoVc) y lo cambió **a mano** en YouTube: no apareció en los 5 resultados pedidos → ver 2.18; un `add` futuro sobre Covers re-agregaría el clip y reportaría el Topic como extra → ver 2.17. **GO del autor 2026-10-30.** Re-correr cuesta búsquedas: autor elige **opción A** (caché de matches en memoria por sesión, 2.16); B (ID de Spotify en `contentDetails.note` de cada ítem, sin verificar) y C (validar por duración con `videos.list`) quedan pospuestas |
| 1.8 | `planV2.md` con hallazgos de ToS + spike | todo | Tras 1.7 (GO dado). Delta sobre V1 ("reemplaza V1 §X"): §39 audiencia, §58 BYO Client ID, §35 seguidas ajenas, atribución, `forget`, §52 confirmación antes de escribir, privacy policy desde v1, auto-update (YT Policies III.B.1 vs V1 “sin auto-update”), §5 sin `map.json`, persistencia de tokens entre corridas (opción C: refresh token en llavero del SO + `logout`; descartada para v1 el 2026-10-30, se eligió `shell`), §3/§4/§6.4/§11 quota (`search.list` ya no es 100 units: cubeta propia de 100/día; cuello de botella = búsquedas, ~14 días para la biblioteca del autor), §6.2 match **Topic > VEVO > primero**, `shell` + varias playlists por comando en paralelo, `GOOGLE_CLIENT_SECRET` obligatorio, opción A (caché en memoria) elegida y B/C pospuestas, correcciones manuales del usuario (2.17) |

## Fase 2 — CLI usable

| ID | Item | Estado | Notas |
|---|---|---|---|
| 2.1 | Resolver destino (marker → mismo nombre + prompt) | todo | planV1 §5–§6, sin cache local (ver 2.2) |
| 2.2 | ~~Cache local `map.json`~~ | done | Descartado 2026-10-26: datos YouTube ≤30 días (YT Policies III.E.4.c); el marker basta |
| 2.3 | Match: Topic > VEVO > primer resultado; skip+reporte | done | Metadata Spotify original. Hecho en el spike (`internal/match`), orden decidido por el autor 2026-10-30; 15/15 correctos en real |
| 2.4 | Dedupe `videoId` y tracks Spotify repetidos | done | Hecho en el spike (`transfer.BuildPlan`: mismo video = un ítem; reportado como "same video") |
| 2.5 | Diff: append faltantes, borrar extra en YT, `already in sync` | doing | Spike: append faltantes en orden Spotify, `already in sync` sin reordenar, confirmación con canal + resumen, `-y`/`--yes`. **Falta borrar extras** (hoy solo se reportan). No reordenar si set igual. Antes de escribir: mostrar canal YouTube destino + resumen (crear / +N / −M) y pedir confirmación; `--yes` para saltarla (YT Policies III.C.2, III.E.3.d–e). Nunca cambiar privacy de playlist existente (III.E.3.f) |
| 2.6 | Reintento de unmatched en re-transfer | todo | |
| 2.7 | Tope 5000 ítems | todo | |
| 2.8 | Contador quota + `--daily-budget` + parada limpia | doing | Spike: contador de dos cubetas (units + búsquedas) por sesión y parada limpia en `quotaExceeded` (cancela el resto, causa raíz). Falta `--daily-budget` |
| 2.9 | `--privacy` `--lang es\|en` progreso | doing | Spike: `--privacy` en `youtube create` y progreso de inserts. Falta `--lang` (hoy todo en inglés) |
| 2.10 | Reporte CSV/JSON on demand (no auto-escribir) | todo | Cada track con link `open.spotify.com` + “Data from Spotify” (Policy II.4.1–II.4.2) |
| 2.11 | Tests fixtures match (CI sin APIs) | doing | Spike: fakes HTTP de Spotify y YouTube en `cmd/soundsync` e `internal/*`, sin red. Falta CI (fase 3) |
| 2.12 | Playlist destino borrada → recrear | todo | |
| 2.13 | Rename destino + marker → misma ID | done | `transfer.ResolveDestination`: el marker manda aunque cambie el título (test unitario) |
| 2.14 | Revocar token Google (endpoint `revoke`) al terminar la corrida | done | Al salir del proceso (`exit`, EOF o comando suelto) y al instante si el canal no se puede leer. YT Policies III.D.2. Sin datos locales que borrar; Spotify no tiene endpoint de revoke, tokens solo RAM |
| 2.15 | Primer uso: aceptar privacy policy (links a YouTube ToS, Google Privacy Policy, revocar en myaccount.google.com/permissions) | todo | YT ToS §7 + Policies III.A.1–A.2: obligatorio aun en Testing |
| 2.16 | Caché de matches en memoria durante la sesión | todo | **Opción A**, elegida por el autor 2026-10-30. Recordar track Spotify → video elegido en RAM para que re-correr `add` en la misma sesión cueste 0 búsquedas. Nada a disco |
| 2.17 | Respetar correcciones manuales del usuario en YouTube | todo | Caso real: el autor cambió a mano el match de "Mentiras" en Covers por su Topic. Hoy un `add` re-agregaría el clip y reportaría el Topic como extra. Definir cómo detectar/conservar el cambio antes de implementar el borrado de extras (2.5) |
| 2.18 | Evaluar más resultados por búsqueda | todo | `search.list` cuesta lo mismo con 5 que con 50 resultados (`match.SearchResults` = 5 hoy). El Topic de "Mentiras" no salió en los 5. Medir si subirlo encuentra más Topics sin empeorar el resto |

## Fase 3 — Empaque CLI

| ID | Item | Estado | Notas |
|---|---|---|---|
| 3.1 | GitHub Actions: Windows amd64 exe | todo | Unsigned |
| 3.2 | GitHub Actions: `.deb` amd64 | todo | Debian y derivados |
| 3.3 | README: build, OAuth, testers, SmartScreen | todo | ES + EN o README + README.es. Spotify: cada usuario trae su propio Client ID (BYO, no embebido en binario — Terms VI.1.2–VI.1.3), requiere Premium. No sugerir endorsement (Policy II.1); logo/ícono Soundsync sin parecido a Spotify (nada verde con ondas). Evitar logos Spotify + YouTube juntos (Design: no co-branding) |
| 3.4 | LICENSE | todo | |

## Fase 4 — GUI (Wails v3 + React)

| ID | Item | Estado | Notas |
|---|---|---|---|
| 4.1 | Evaluar Wails v3 vs v2 en Win + Debian | todo | Core intacto; Plan B = v2 |
| 4.2 | Shell React: Connect Spotify / Google | todo | |
| 4.3 | Select playlists + progreso + reporte | todo | i18n es-MX / en. Sin reproducción ni previews (Policy III.5). Design Guidelines: vista de selección Spotify con logo oficial (full ≥70px, ícono ≥21px, zona libre ½ ícono, verde solo sobre negro/blanco); link “OPEN SPOTIFY” por playlist/track; metadata sin alterar, truncar OK si se ve completa (tooltip); sin cover art en v1. Contenido Spotify y YouTube en vistas separadas, nunca lado a lado con logos |
| 4.4 | Prompt combinar mismo nombre | todo | |
| 4.5 | Download reporte on demand | todo | |
| 4.6 | `.msi` + `.deb` con WebView2 / webkitgtk | todo | Sin auto-update |

## Pruebas (acumulado)

| ID | Item | Estado | Notas |
|---|---|---|---|
| T.1 | Unit: parser marker, diff sets, dedupe | todo | |
| T.2 | Unit: match ranking Topic/VEVO vs primer hit | todo | |
| T.3 | Manual: playlist 5 tracks spike | todo | Fase 1 |
| T.4 | Manual: re-transfer set igual → already in sync | todo | Cero inserts |
| T.5 | Manual: origen agregó 1 track → un insert | todo | |
| T.6 | Manual: origen quitó 1 track → delete en YT | todo | |
| T.7 | Manual: collab Spotify → playlist YT propia | todo | |
| T.8 | Manual: mismo título sin marker → prompt | todo | |
| T.9 | Manual: quota tope → stop + reporte | todo | |

## Despliegue / releases

| ID | Item | Estado | Notas |
|---|---|---|---|
| D.1 | Repo GitHub público | todo | Aún local |
| D.2 | Release CLI Windows + deb | todo | Tras fase 3 |
| D.3 | Release GUI msi + deb | todo | Tras fase 4 |
| D.4 | Privacy policy URL (`PRIVACY.md` en repo público) | todo | Requerida desde v1, no solo fuera de testers (YT ToS §7, Policies III.A.2). Debe decir que usa YouTube API Services |

## Bloqueos

_Ninguno registrado._

## Log

| Fecha | Qué |
|---|---|
| 2026-10-30 | **1.7 done: GO del autor.** Opción A elegida (2.16). Autor corrigió a mano el match de "Mentiras" en Covers por su Topic (https://youtu.be/DZquZF0PoVc) → nuevos 2.17 (respetar correcciones) y 2.18 (más resultados por búsqueda). Fase 2 actualizada con lo que ya hizo el spike (2.3, 2.4, 2.13, 2.14 done; 2.5, 2.8, 2.9, 2.11 doing). README con uso actual de la CLI. Sigue 1.8. |
| 2026-10-30 | 1.7: autor confirma escuchando que los 2 matches dudosos son correctos → 15/15 en total. |
| 2026-10-30 | 1.7 primera muestra: "Tristes" + "Covers" en paralelo en una sesión, 13/13 insertados, idempotencia OK. Hallazgos: `first` ≠ malo (sellos/canales de artista), 2 matches dudosos, re-correr gasta búsquedas, cuello de botella = 100 búsquedas/día. |
| 2026-10-30 | Varias playlists por comando (`youtube create A B -y`, `youtube add A B --limit 7`), procesadas en paralelo en una sola sesión. Bugs encontrados por los tests: error raíz tapado por el de la playlist cancelada (intermitente), carrera de lectura en el fake del test, fake de quota irreal (solo fallaba 1 insert). |
| 2026-10-30 | Prioridad de match cambiada a Topic > VEVO > primero (decisión del autor). 1.7 arranca con "Tristes" + "Covers" en paralelo. |
| 2026-10-30 | 1.6 done: primera transferencia real Spotify → YouTube (2/2 canciones correctas, verificado escuchando), idempotencia OK, quota medida = estimada (109 units, 4 búsquedas). |
| 2026-10-30 | 1.6 código: `internal/match`, `transfer.BuildPlan`, `youtube.SearchVideos`/`PlaylistVideoIDs`/`AddVideo`, comando `youtube add` (autor eligió un solo comando con `y/N` en vez de `match` + `add`). Hallazgo: Google cambió la quota (overview 2026-10-15), `search.list` = 1 unit + cubeta de 100/día; `compliance.md` actualizado, planV2 debe corregir §3/§4/§6.4/§11. |
| 2026-10-30 | 1.5 done: primera escritura real en YouTube, idempotencia por marker verificada, quota medida = estimada (54). |
| 2026-10-30 | 1.5 código: `internal/transfer` (marker, `ResolveDestination`), `youtube.MyPlaylists`/`CreatePlaylist` + contador de quota, comandos `youtube create`/`youtube playlists`, confirmación `y/N` que lee del mismo scanner del shell. Primera escritura real elegida: "The Way to My Favorite Part". |
| 2026-10-30 | `shell` verificado en Windows: 1 pestaña Spotify + 1 Google para 5 comandos; `tracks Tristes` y `"The Way to My Favorite Part"` resueltos por nombre; playlist seguida rechazada sin llamar a la API; revoke OK al `exit`. |
| 2026-10-30 | Login repetido por comando: autor elige opción B (`soundsync shell`) sobre A (status quo) y C (llavero SO, va a planV2). Refactor: `app` con sesiones en memoria; timeout solo cubre la espera del login; clientes OAuth con ctx de proceso (refresh); revoke de Google al salir del proceso (cualquier comando); `spotify tracks` acepta nombre (`FindPlaylists`: exacto > parcial, ambigüedad lista candidatas). |
| 2026-10-30 | 1.4 código: `internal/spotify/playlists.go` + comandos `spotify playlists` y `spotify tracks`. `loginRunner` recibe args. Autor creó canal YouTube. |
| 2026-10-30 | 1.2 done (login real OK). 1.3 código: `internal/youtube` (endpoints, `MyChannel`, `Revoke`), `login youtube`, `GOOGLE_CLIENT_SECRET` pasa a obligatorio. |
| 2026-10-30 | 1.2 fix: `state` ajeno o ausente ya no aborta el login (antes cualquier request a `/callback` lo mataba; también era DoS trivial). `Flow.Logf` para notas de diagnóstico. Spotify devuelve un param extra `ubi` en el callback (ignorado). |
| 2026-10-30 | 1.2 código: `internal/oauth` (flujo loopback PKCE reutilizable para Google), `internal/spotify` (endpoints, scopes, `Me`), comando `login spotify`. Repo irá a la cuenta personal del autor. |
| 2026-10-30 | 1.1: scaffold Go (`cmd/soundsync`, `internal/{config,oauth,spotify,youtube,match,transfer,i18n}`), comandos `version` y `config check`, tests. `.gitignore` + `/soundsync`. |
| 2026-10-26 | Docs de la sesión: `docs/compliance.md` (reglas Spotify/YouTube con cláusulas), `docs/setup.md` (credenciales BYO paso a paso). README enlaza ambos. |
| 2026-10-26 | Fase 0 cerrada: proyecto Google `soundsync`, YouTube Data API v3 habilitada, consent Testing + tester, OAuth client Desktop. Credenciales en `.env` (formato validado). Consola revisada por capturas: API habilitada, Testing + External + tester, scope `youtube.force-ssl` en sensibles, client Desktop (ID coincide con `.env`). `.env.example` + `GOOGLE_CLIENT_SECRET`. |
| 2026-10-26 | Revisión YouTube ToS + Developer Policies: credenciales fuera del repo (BYO), privacy policy obligatoria desde v1, confirmación antes de escribir, revocar token, datos autorizados ≤30 días → `map.json` descartado (decisión autor). Ajustes en 0.3, 2.1, 2.2, 2.5, 2.14, 2.15, D.4, 1.8. |
| 2026-10-26 | Revisión Spotify Design Guidelines: logo obligatorio para atribuir en GUI, link back, metadata intacta, no co-branding ni contenido junto a servicios similares. Ajustes en 3.3, 4.3. App Spotify `Soundsync` creada (autor aceptó Terms); Client ID en `.env`. 0.1 done. |
| 2026-10-26 | Revisión ToS Spotify: transferir metadata de playlists permitido (Policy III.9). Dev Mode feb-2026: 5 users, owner Premium, endpoints `/items`. Ajustes en 0.1, 1.4, 2.10, 2.14, 3.3, 4.3. `.gitignore` + `.playwright-mcp/`. |
| 2026-10-26 | Fase 0: README scopes, `.env.example`, `.gitignore`. Dashboard Spotify espera login. |
| 2026-10-23 | Cerrado planV1. Creados `docs/planV1.md` y `docs/todo.md`. |
