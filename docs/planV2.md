# Soundsync — Plan v2

Documento congelado. No se edita: cambios de alcance van a `planV3.md` (o posterior). El tracking vivo está en `todo.md`.

Fecha de cierre: 2026-11-01.

V2 es un **delta** sobre `planV1.md`: lo que no aparece aquí sigue vigente tal cual en V1. Cada sección dice qué parte de V1 reemplaza. Fuentes: revisión de términos (`compliance.md`, 2026-10-26) y spike de fase 1 (todo 1.1–1.7, GO del autor 2026-10-30).

## 1. Audiencia y cuentas

Reemplaza V1 §3 fila "Audiencia" y fila "Quota", y §4 viñeta "Client IDs".

- **Bring your own (BYO):** cada usuario crea su propia app de Spotify y su propio OAuth client de Google ([setup.md](setup.md)). Ningún Client ID ni secret va en el repo ni en binarios publicados (Spotify Terms VI.1.2–VI.1.3, YouTube Policies III.D.1).
- Consecuencia: la quota es **del proyecto Google de cada usuario**, no un proyecto compartido del autor.
- Spotify Development Mode (desde 2026-03-09): máximo **5 usuarios** por app (allowlist; antes ~25), el dueño de la app necesita **Premium activo**. Extended quota solo para organizaciones ≥250k MAU: Soundsync se queda en Dev Mode.
- Google OAuth en Testing: máximo 100 testers. La caducidad de refresh token (~7 días) no aplica en v1 porque no se guardan tokens (ver §8).
- `GOOGLE_CLIENT_SECRET` es **obligatorio**: Google lo exige en el token exchange de clientes Desktop aun con PKCE. No es confidencial, pero se guarda solo en `.env` (gitignored).
- Repo público en la cuenta personal del autor (`github.com/CharlyCeballos/soundsync`), licencia MIT.

## 2. Contenido Spotify

Reemplaza V1 §3 fila "Contenido".

- Playlists **propias y colaborativas**: se leen.
- Playlists **seguidas de otros**: Dev Mode no deja leer sus ítems → skip + reporte, sin llamar a la API.
- Endpoints 2026: `/me/playlists` y `/playlists/{id}/items` (no `/tracks`). Episodios y ítems nulos → skip; local files se marcan.

## 3. Identidad de playlist

Reemplaza V1 §5 punto 2 y §6.1 paso 1.

- **Sin `map.json`** ni ningún caché local. El marker `soundsync:spotify:<id>` en la descripción es la única identidad. Motivo: datos autorizados de YouTube ≤30 días (Policies III.E.4.c) y no guardar contenido de Spotify sin necesidad (Terms IV.3.1.1).
- Reglas de V1 §5 (renombrada → misma ID; borrada → recrear; mismo título sin marker → preguntar y combinar) siguen igual.

## 4. Matching

Reemplaza V1 §2 párrafo de matching y §6.2 "Match por track" puntos 1–2.

- Orden: **canal `- Topic` > VEVO > primer resultado**. Decisión del autor 2026-10-30: fidelidad al audio del álbum, sin intros/outros de video musical.
- Query = primer artista + título original de Spotify. 5 resultados por búsqueda (todo 2.18 evalúa subirlo: cuesta lo mismo).
- Resultado del spike: 15/15 correctos verificados escuchando. `first` no es sinónimo de malo (sellos y canales de artista sin marca Topic/VEVO).
- Caché de matches **en RAM durante la sesión** (opción A): re-correr `add` en la misma sesión cuesta 0 búsquedas. Nada a disco. Pospuestas: B (ID de Spotify en `contentDetails.note` del ítem) y C (validar por duración con `videos.list`).
- **Correcciones manuales** del usuario en YouTube (cambió el video elegido): se respetan. Cómo detectarlas se define antes de implementar el borrado de extras (todo 2.17 bloquea la parte "borrar" de 2.5).

## 5. Quota

Reemplaza V1 §4 viñeta "Sin límite por usuario", §6.4 y §11 riesgo 1.

- Overview de Google actualizado 2026-10-15: `search.list` ya **no** cuesta 100 units. Cuesta 1 unit **más** una llamada de una cubeta propia de **100 búsquedas/día**. El resto de endpoints sale del pool de 10 000 units/día.
- `playlists.insert` y `playlistItems.insert` = 50 units cada uno, sin variante batch → ~199 inserts/día por units.
- Cuello de botella = búsquedas: **~100 tracks/día**. Biblioteca del autor (~1331 tracks) ≈ 14 días.
- El cliente cuenta **las dos cubetas** (units + búsquedas) por sesión; las llamadas fallidas cuentan completas (conservador).
- `--daily-budget` = tope **de la sesión** (el contador vive en RAM, no hay estado en disco). El tope diario real lo impone Google con `quotaExceeded`: parada limpia, cancela lo pendiente, reporta la causa raíz.

## 6. UX de la CLI

Reemplaza V1 §9 fase 2 "Multi-playlist" en cuanto a forma, y agrega a §13.

- **`soundsync shell`:** un login por servicio para toda la sesión; los comandos corren dentro. Comandos sueltos siguen existiendo.
- Varias playlists por comando (`youtube create A B`, `youtube add A B`): se valida todo antes de gastar quota, **una sola confirmación**, playlists en paralelo (dentro de cada una, orden Spotify), salida en orden de argumentos.
- Playlists por nombre, ID, link o URI; ambigüedad → lista candidatas.
- **Confirmación antes de escribir** en YouTube: mostrar canal destino + resumen (crear / +N / −M) y pedir `y/N`; `-y`/`--yes` la salta (Policies III.C.2, III.E.3.d–e).
- Nunca cambiar la privacy de una playlist existente; solo se fija al crear (Policies III.E.3.f).

## 7. Atribución y privacidad

Agrega a V1 §4 y §13; reemplaza V1 §11 riesgo 7 en cuanto a la privacy policy.

- Reporte y listados: "Data from Spotify" + link `open.spotify.com` por track (Policy II.4.1–II.4.2); "Data from YouTube" + link al video (Policies III.F.2).
- GUI: logo oficial de Spotify según Design Guidelines; contenido de Spotify y de YouTube en vistas separadas, nunca logos juntos (sin co-branding). Logo de Soundsync sin parecido al de Spotify.
- **Privacy policy desde v1** (`PRIVACY.md` en el repo público), aun en Testing: dice que usa YouTube API Services, enlaza Google Privacy Policy y explica cómo revocar (myaccount.google.com/permissions) (ToS §7, Policies III.A.1–A.2).
- **Primer uso:** aceptar la privacy policy, con links a YouTube ToS y Google Privacy Policy (todo 2.15).

## 8. Tokens y datos del usuario

Reemplaza V1 §4 viñeta "Tokens OAuth" y §12 viñeta `map.json`.

- Tokens solo en RAM, por sesión. Sin persistencia entre corridas en v1.
- Al salir del proceso (`exit`, EOF o comando suelto) se **revoca** el token de Google (Policies III.D.2). Spotify no tiene endpoint de revoke; el token muere con el proceso.
- No hace falta comando `forget`: no hay datos locales que borrar.
- Pospuesto a después de v1 (opción C): refresh token en el llavero del SO + comando `logout`.
- Callbacks OAuth con `state` ajeno o ausente se rechazan (400) sin abortar el login en curso.

## 9. Actualizaciones

Reemplaza V1 §3 "Auto-update: no" y la entrada "auto-update" de §14.

- Policies III.B.1: las apps instaladas deben poder actualizarse a la versión vigente de la API.
- Decisión del autor 2026-11-01: **aviso de versión al iniciar**. Al abrir `shell`, un GET a GitHub Releases `latest` del repo; si hay versión más nueva, imprime el link de descarga.
- No descarga ni instala nada. No manda datos del usuario. Falla de red = una línea de aviso y sigue. Builds `dev` (sin versión) no consultan.
- Requiere repo público (todo D.1).
- Auto-update real (descargar e instalar solo) sigue fuera de v1.

## 10. Riesgos nuevos

Agrega a V1 §11.

1. Spotify: si el Premium del dueño de la app caduca, la app deja de funcionar. Permisos sin uso 90 días pueden suspenderse (Terms IX.8.5).
2. Google: proyectos sin uso 90 días pueden recortarse (Policies III.D.4).
3. Correcciones manuales del usuario: un diff ingenuo las revierte (todo 2.17).
4. Re-correr cuesta búsquedas aunque no haya writes; mitigado dentro de la sesión por el caché en RAM.

## 11. Estado al cierre

- Fase 0: done.
- Fase 1: done, **GO** 2026-10-30. Hecho en el spike y heredado por fase 2: match (2.3), dedupe (2.4), rename por marker (2.13), revoke (2.14); parcial: diff sin borrado (2.5), contador sin `--daily-budget` (2.8), `--privacy` sin `--lang` (2.9), fakes sin CI (2.11).
- Siguiente: fase 2.
