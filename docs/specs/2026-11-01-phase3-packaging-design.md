# Fase 3 — Empaque CLI: diseño

Fecha: 2026-11-01. Cubre todo 3.1, 3.2, 3.3 y 3.5. D.1 (repo público) queda fuera: el autor decidió hacerlo público después de la fase 4.

## 1. Intención y por qué

Que alguien que no es el autor pueda bajar Soundsync ya compilado (Windows o Debian), ponerle sus propias credenciales y usarlo sin instalar Go ni clonar el repo. Hoy solo se corre con `go run` desde el repo, y el `.env` solo se busca en el directorio actual, lo que no sirve para un binario instalado.

## 2. Metas, no-metas, anti-metas

- Metas: release con exe de Windows y `.deb` amd64 al empujar un tag `v*`; binario instalado encuentra su `.env` sin `--env-file`; README en inglés y español para usuarios; aviso de versión nueva al abrir `shell` (planV2 §9).
- No-metas: firma de código, `.msi`, auto-update, otras arquitecturas, repositorio apt, hacer público el repo.
- Anti-metas: credenciales dentro de un binario o del repo (Spotify Terms VI.1.2–VI.1.3, YouTube Policies III.D.1); acciones o binarios de terceros nuevos en la cadena de release; el aviso de versión mandando cualquier dato del usuario o bloqueando el arranque.

## 3. Restricciones

- Sin dependencias nuevas: Go stdlib + lo que ya trae `ubuntu-latest` (`dpkg-deb`, `sha256sum`, `zip`, `gh`).
- Repo privado hasta después de la fase 4: `releases/latest` sin auth responde 404 para todos, así que el aviso de versión no funciona en real hasta entonces y se prueba con servidor falso.
- `.gitattributes` LF; Go 1.26.0 en CI (`go.mod`).

## 4. Enfoques

1. **Workflow propio (elegido por el autor):** `go build` + `dpkg-deb --build` + `sha256sum` + `gh release create --draft`. Cero dependencias nuevas, ~60 líneas de YAML.
2. GoReleaser: un archivo de config hace todo, pero agrega una acción y un binario de terceros en cada release.
3. nfpm + gh: igual que el 1 con una dependencia más para ahorrar ~10 líneas.

## 5. Decidido

Del autor:

- `.env`: primero `./.env`; si no existe, `<UserConfigDir>/soundsync/.env` (`%APPDATA%\soundsync\.env` en Windows, `~/.config/soundsync/.env` en Linux), la misma carpeta que ya guarda `privacy-accepted`. `--env-file` explícito sigue mandando. `config check` imprime la ruta usada.
- Release: tag `v*` → release **borrador**; el autor lo revisa y lo publica a mano. `releases/latest` ignora borradores, así que el aviso de versión solo ve lo publicado.
- README: `README.md` en inglés + `README.es.md` en español, con link cruzado arriba.
- Repo público: después de la fase 4.

De Claude (propuestas):

- Tags válidos solo `vMAJOR.MINOR.PATCH`; el workflow falla con otro formato. `main.version` = tag; versión del `.deb` = tag sin `v`.
- Artefactos: `soundsync_<ver>_windows_amd64.zip` (exe + `LICENSE` + `.env.example`), `soundsync_<ver>_amd64.deb` (`/usr/bin/soundsync`, `/usr/share/doc/soundsync/copyright` y `env.example`, `Recommends: xdg-utils` porque el login abre el navegador con `xdg-open`), `SHA256SUMS`. Builds con `CGO_ENABLED=0 -trimpath`.
- El job de release corre `go vet` + `go test` antes de compilar, instala el `.deb` en el runner y comprueba que `soundsync version` imprime el tag (smoke test).
- Aviso de versión: solo en `shell`, solo si `version != "dev"`; GET `https://api.github.com/repos/CharlyCeballos/soundsync/releases/latest` con timeout de 2 s, síncrono antes del prompt. Versión mayor → una línea con el link. 404 = sin release publicado → silencio. Otro error → una línea y sigue. Comparación semver con un parser propio pequeño (sin `x/mod`), con test.

## 6. Seguridad

- Workflow de release: `permissions: contents: write` solo en ese job; acciones fijadas por SHA (las mismas de `ci.yml`); `persist-credentials: false`; el nombre del tag pasa por variable de entorno, nunca interpolado en `run:` (inyección de scripts). Solo quien puede empujar tags (el autor) dispara un release.
- Aviso de versión: no manda nada del usuario; GitHub ve la IP y el `User-Agent: soundsync/<ver>` → se agrega a `PRIVACY.md`. Respuesta limitada a 1 MiB; `tag_name` debe ser semver válido y `html_url` debe empezar con `https://github.com/CharlyCeballos/soundsync/releases/`, si no se ignora (nada arbitrario llega a la terminal).
- `.env` en la carpeta de usuario: el README pide permisos solo del usuario; Soundsync solo lo lee, nunca lo escribe.
- Historial git revisado 2026-11-01: las credenciales reales del `.env` no aparecen en ningún commit.

## 7. UX

Sin GUI. En la CLI: `config check` dice qué `.env` leyó (o que no encontró ninguno y dónde buscó); el aviso de versión es una línea traducida (`--lang`) con el link. README explica la advertencia de SmartScreen (exe sin firmar: "Más información → Ejecutar de todas formas") y `sudo apt install ./soundsync_<ver>_amd64.deb`.

## 8. Datos y migraciones

No hay datos ni esquema. El `.env` del autor en el repo sigue funcionando igual (tiene prioridad).

## 9. Despliegue y rollback

Release = `git tag vX.Y.Z && git push origin vX.Y.Z` (con OK del autor). Rollback: borrar el release borrador y el tag (`gh release delete vX.Y.Z --cleanup-tag`). Primer release real `v0.1.0` sirve de prueba del workflow; el autor baja el zip, corre `soundsync version` y `config check` en su Windows.

## 10. Para quien construye

- Texto exacto de README y README.es (contenido de usuario: qué es, requisitos — Premium del dueño de la app Spotify, máximo 5 usuarios, Google en Testing —, descarga e instalación, credenciales con link a `docs/setup.md`, dónde va el `.env`, comandos, quota, privacidad, compilar desde el código, aviso de no afiliación con Spotify/Google sin logos).
- Mensajes nuevos en el catálogo `i18n` con su traducción.
- Actualizar `docs/todo.md` (3.1, 3.2, 3.3, 3.5) y el log.
