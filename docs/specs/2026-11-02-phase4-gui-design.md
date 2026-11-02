# Fase 4 — GUI de Soundsync (Wails v3 + React): diseño

Fecha: 2026-11-02. Cubre todo 4.1–4.6. Plan aprobado por el autor el 2026-11-02; la sección final dice cómo quedó.

## Contexto

Soundsync hoy es solo CLI (`cmd/soundsync`, v0.1.0 publicado). La fase 4 agrega una GUI de escritorio para Windows 11 y Debian 13 con los mismos casos de uso:

1. Conectar Spotify y Google.
2. Elegir playlists.
3. Revisar coincidencias.
4. Confirmar y escribir en YouTube.
5. Bajar un reporte.

Principios de `planV1`: "CLI es fuente de verdad, GUI es cáscara" y "core no se acopla a Wails".

Problema: el motor de copia vive en `package main` (`cmd/soundsync/youtube_add.go`, `destinations.go`, `report.go`, `app.go`), mezclado con salida de texto y preguntas por stdin, y la GUI no puede importarlo. Con dos consumidores (CLI y GUI), hay que extraerlo a paquetes limpios **sin cambiar ni un byte de lo que imprime la CLI**. Los 104 tests actuales lo protegen.

### Ya probado en esta sesión (4.1, terminado)

- **Wails v3.1.0** es estable (v3.0.0 salió el 2026-10-26).
  - Windows 11: compila sin CGO, el exe pesa 11.4 MB y lleva 3 módulos Go. WebView2 pinta y el puente funciona en los dos sentidos (JS→Go y Go→JS).
  - Debian 13 (WSL + WSLg): GTK4 + `libwebkitgtk-6.0`, exe de 10.5 MB con 2 módulos Go; ventana y puente funcionan.
  - `npm audit`: 0 vulnerabilidades.
- **Wails v2 descartado:** pide `webkit2gtk-4.0`, que Debian 13 no trae.
- **Módulo anidado:** `gui/go.mod` con `replace ../` importa `internal/` del módulo raíz, y `wails3 generate bindings` genera TS para servicios de ese paquete. Si un método recibe `ctx`, el TS devuelve `CancellablePromise`, y cancelarla cancela el `ctx` en Go.
- **Server mode** (`-tags server`): la app sirve la UI por HTTP sin ventana, lo que permite E2E reales con Playwright.
- **WebView2 faltante:** Wails v3.1.0 se cierra sin mensaje, así que hace falta un chequeo propio.
- **`.msi`:** Wails no lo genera; solo produce NSIS o MSIX.

## Decisiones

### D1. Arquitectura

```
soundsync/                    módulo raíz: CLI + core, sin CGO; la CI actual no cambia
  cmd/soundsync/              CLI: flags, prompts, texto y orden de los pasos (como hoy)
  internal/session/   NUEVO   login Spotify/Google con sus pistas de error, sesiones, revocar, caché RAM de matches y de la biblioteca (con mutex)
  internal/copier/    NUEVO   pipeline sin salida: Resolve/Adopt/Ensure de destinos, Prepare, LookUp, Match (Budget), Changing, Write, FirstErr; Observer
  internal/report/    NUEVO   Row + estados como constantes, Rows(jobs), CSV/JSON, defensa CSV injection, Save con O_EXCL
  internal/consent/   NUEVO   versión de la política + archivo de aceptación (sale de privacy.go; el texto del aviso se queda en la CLI)
  internal/update/    NUEVO   parseVersion + latestRelease (la GUI también muestra el aviso: Policies III.B.1)
  internal/config/    +       Dir(); Load(path) relee el archivo cada vez sin tocar el entorno del proceso (el entorno sigue ganando, como hoy)
  internal/desktop/   NUEVO   backend GUI: servicios ligados a Wails, sin importar Wails
  internal/fakeapi/   NUEVO   APIs falsas Spotify/YouTube/revoke (salen de los _test.go de cmd), sin *testing.T
gui/                          módulo anidado .../soundsync/gui (replace ../); solo cáscara
  main.go, host.go, webview2_windows.go, e2e.go (//go:build server && e2e)
  frontend/ (React + TS + Vite), build/ (íconos, manifest, .desktop, .wxs, package.sh), Taskfile.yml
```

- El `go test ./...` del raíz no ve `gui/` (es otro módulo), así que la CI de la CLI no necesita CGO, GTK ni Node.
- `internal/desktop` sí corre en la CI actual: Linux con `-race` y Windows.

### D2. API del motor (resumen)

- **`session`:**
  - `LoginSpotify(ctx, base, cfg, UI{Open, Logf})`.
  - `AuthorizeYouTube` + `LoadChannel`: el llamador revoca si falla.
  - `(*YouTube).Revoke()` con timeout propio.
  - `(*Spotify).Playlists(ctx, refresh)`.
  - Los clientes OAuth se construyen con el contexto base de la app, nunca con el de una llamada; si no, el refresh se rompería a la hora.
- **`copier`:**
  - `Resolve(ytPlaylists, srcs) []Destination`. Las homónimas sin marker vuelven como `Create` + `Candidates`.
  - `(*Destination).Adopt(playlistID)` valida que el ID esté entre las candidatas. Desde JS solo viaja el ID, nunca el objeto (si no, se pierde `defaultLanguage`).
  - **El motor nunca pregunta.** La CLI pregunta por stdin como hoy (con `--yes` sigue fallando); la GUI pregunta con un diálogo (4.4).
  - `Options{Limit, MaxItems}` reemplaza la variable global `maxPlaylistItems` que hoy usa un test.
  - `Observer func(Event)`: cada evento lleva solo valores (`SourceID`, paso `lookup|search|create|mark|add|remove`, `Done/Total`, `Title`, copia del `Target`). Puede llamarse desde varias goroutines.
  - La CLI pasa `nil` al planear. Al escribir pasa un observer que imprime las mismas líneas de hoy; la GUI los convierte en eventos.
  - `Job.InSync()` lleva la regla del resumen, que no es la misma que `Plan.InSync`.
- **En `cmd` se queda:**
  - `app` y las costuras de test.
  - Prompts, `chooseDestinations`/`askAdopt` (encima de `Resolve`/`Adopt`) y todos los `print*`.
  - `confirmAndWrite`, `runYouTubeAdd`/`Create`, `validate` y los mensajes de `saveReport`.
  - `sources.go` y `spotify_cmd.go` completos: llevan tips de CLI, y la GUI elige por ID.

### D3. Backend de la GUI (`internal/desktop`)

**Servicios ligados** (cada método exportado es llamable desde JS, así que todo se valida en Go):
- **`Settings`**:
  - `Status()`: idioma, política aceptada, ruta y problemas del `.env` (sin secretos), versión y aviso de release.
  - `SetLanguage`, `ReloadConfig`, `OpenConfigFolder`, `AcceptPolicy`, `OpenURL`.
- **`Accounts`**:
  - `ConnectSpotify(ctx)` y `ConnectYouTube(ctx)`: enfocan la ventana al volver del navegador.
  - `DisconnectSpotify`, y `DisconnectYouTube`, que revoca al instante (III.D.2.c).
  - `Playlists(ctx, refresh)`.
- **`Copy`**:
  - `Resolve(ctx, ids)`: re-valida que cada ID sea propio o colaborativo.
  - `Plan(ctx, {Choices, Privacy})`.
  - `Write(ctx)`: **una sola vez por plan**; un segundo clic no duplica videos.
  - `SaveReport(format)`.

**`Host`** (lo implementa `gui/host.go` con Wails; en tests es un grabador):
- `Emit(name, data)`, `OpenURL(url)`, `SaveFile(defaultName)` y `Focus()`.
- `Shutdown()` y las costuras de test no viven en tipos ligados.

**Eventos:**
- `login:url {provider, url}`: link de respaldo si el navegador no abre.
- `copy:progress {sourceId, step, done, total, title?}`.

**Estado y concurrencia:**
- Un mutex que nunca se sostiene durante llamadas de red, más un operador `busy`: no hay dos logins, planes o escrituras a la vez (el puerto loopback es fijo).
- Estados: `resolved → planned → written`.
- Detener una corrida se muestra como "cancelado", no como `context canceled`.
- El presupuesto de búsquedas se acota a ≥0 al mostrarlo y es por sesión, como en la CLI.

**`.env` en la GUI:** solo en la carpeta de configuración del usuario (`%APPDATA%\soundsync` o `~/.config/soundsync`). La carpeta de trabajo de un acceso directo es impredecible. La pantalla de configuración muestra la ruta exacta.

**Reporte:**
- El nombre por defecto lleva fecha y hora (`soundsync-report-2026-11-02-1530.csv`).
- Se mantiene "Soundsync nunca sobrescribe archivos": si el usuario elige uno existente, error claro y elegir otro.

**Idioma y seguridad:**
- `i18n.current` pasa a ser atómico, porque la GUI cambia de idioma con goroutines vivas.
- El test de i18n se amplía a `internal/{session,copier,desktop}`. Hoy pasaría aunque faltaran mensajes movidos.
- `OpenURL` solo acepta `https` a una allowlist: `open.spotify.com`, `www.youtube.com`, `music.youtube.com`, `myaccount.google.com`, `security.google.com`, `policies.google.com` y `github.com/CharlyCeballos/soundsync`.
- Al cerrar: cancela lo que corre, espera ≤5 s y revoca Google.

### D4. UX (4.2–4.5) y reglas de marca

Investigación del 2026-11-02, que irá a `docs/compliance.md`:
- **YouTube Branding Guidelines:** logo "Developed with YouTube" junto al contenido de YouTube, clickeable al canal. III.F.2.b: si se mezclan fuentes, cada ítem se atribuye por separado.
- **Spotify Design Guidelines:** full logo de 70px o más junto a todo metadato de Spotify, link de vuelta (`OPEN SPOTIFY`), conjuntos de 20 ítems o menos, nunca junto a contenido de servicios similares.
- **Choque:** la pantalla de coincidencias juntaría las dos marcas y ninguna cláusula lo resuelve. **Solución: separar por vista, no por fila.**

**Asistente de 3 pasos.** Ninguna pantalla lleva los dos logos; las cuentas conectadas aparecen solo como texto.

0. **Puertas** (solo cuando aplican):
   - Configuración incompleta: qué falta y dónde va el `.env`, con botones para abrir la carpeta, ver la guía y revisar de nuevo.
   - Aviso de privacidad la primera vez, con los links de la CLI.
1. **Elige playlists** (vista Spotify):
   - Logo de Spotify, monocromo según el tema.
   - "Conectar con Spotify". Mientras espera: "Esperando que inicies sesión en tu navegador", link de respaldo y Cancelar.
   - Casillas, filtro y páginas de 20.
   - Las playlists seguidas de otros aparecen deshabilitadas, con el motivo.
   - Nombres truncados con tooltip.
   - `OPEN SPOTIFY` por playlist y al final de cada página.
2. **Revisa en YouTube** (vista YouTube):
   - "Developed with YouTube" enlazado al canal y "Se copiará al canal: X".
   - "Conectar con YouTube (cuenta de Google)".
   - Privacidad de las playlists nuevas: privada por defecto.
   - **4.4:** diálogo por cada playlist con homónimas sin marker. Lista las candidatas (título, privacidad, videos, link) o permite crear una nueva, y explica que se agrega una línea a la descripción y que sus videos se quedan.
   - "Buscar coincidencias", con barra de progreso y botón Detener.
   - Resultado por playlist en **dos pestañas que nunca se mezclan**, unidas por el número de posición:
     - **Canciones (Spotify):** estado de cada track, logo y `OPEN SPOTIFY`, páginas de 20.
     - **Videos (YouTube):** a agregar y a quitar, con título, canal y link.
   - Resumen exacto de lo que se escribirá:
     - crear "X" (privacidad y descripción con la línea de Soundsync);
     - +N videos, −M videos;
     - unidades de quota.
   - Botones "Escribir en YouTube" y "Cancelar" (III.C.2, III.E.3.d–e).
3. **Listo:**
   - Progreso en vivo, con Detener.
   - Resumen por playlist igual al de la CLI, con "Abrir en YouTube Music" y la quota usada en la sesión.
   - **4.5:** "Descargar reporte CSV / JSON" con diálogo nativo; incluye la última corrida aunque se haya detenido.
   - "Copiar otras playlists".

**Siempre visibles:**
- Footer con Privacidad, Términos de YouTube, Política de privacidad de Google y Revocar acceso (III.A.1–A.2).
- Selector ES/EN; por defecto, el idioma del sistema.
- Aviso de versión nueva.
- "Desconectar" (Compliance Tips de Spotify; III.D.2.c).

**Seguridad de la UI:**
- Nunca un `<a href>` que navegue el webview: `ExternalLink` llama a `Settings.OpenURL`, con un guard global de clics como defensa en profundidad.
- CSP estricta.
- Los datos de las APIs se muestran solo como texto; nunca `dangerouslySetInnerHTML`, porque los títulos llegan sin escapar.
- Sin recursos remotos.

**Marca e i18n:**
- Ícono propio de Soundsync: sin verde, sin círculo con ondas y sin triángulo de play.
- Los textos de UI van en un diccionario TS tipado `en`/`es`: el compilador exige las dos traducciones.

### D5. Empaque (4.6)

- **Debian:** paquete aparte `soundsync-gui`, para que la CLI siga sin GTK.
  - Contenido: `/usr/bin/soundsync-gui`, `.desktop` e ícono hicolor.
  - Dependencias: `Depends: libgtk-4-1, libwebkitgtk-6.0-4` y `Recommends: xdg-utils`.
  - Se compila **dentro de `debian:trixie`** (imagen fijada por digest). Si se compilara en `ubuntu-latest`, que migra a 26.04, el binario pediría un glibc más nuevo que el de Debian 13.
  - Se empaqueta con `dpkg-deb`, como la CLI.
- **Windows `.msi`:**
  - Instalación por usuario en `%LOCALAPPDATA%\Programs\Soundsync`, sin admin.
  - Acceso en el menú Inicio, `MajorUpgrade` y desinstalación limpia.
  - Exe compilado en Linux para Windows (CGO=0), sin firma: SmartScreen, igual que el zip.
  - **Herramienta: wixl 0.106** (msitools, LGPL, sin EULA), del apt del mismo contenedor trixie. Decide la prueba real: instalar, actualizar y desinstalar en tu Windows 11.
  - Plan B: WiX v7.0.1. Exige que **tú** aceptes su EULA OSMF; yo no lo aceptaré por ti.

  | | wixl 0.106 (elegido) | WiX v7.0.1 | WiX 3.14 |
  |---|---|---|---|
  | Licencia | LGPL, sin cuota | MS-RL + EULA OSMF obligatorio (cuota solo si facturas ≥ US$10k/año) | MS-RL |
  | Corre en | Linux (trixie) | solo Windows | Windows |
  | Mantenimiento | lento (último 2025-03) | activo (2026-10-26) | sin soporte desde 2025-02 |
  | CVEs | ninguno en el tracker de Debian | 3 de 2024 (Burn), corregidos | CVE-2024-29187 (Burn) |

- **WebView2:** antes de crear la ventana, la app lee las claves `pv` del registro, como recomienda Microsoft, con `x/sys/windows/registry` (ya llega con Wails). Si falta, muestra un MessageBox nativo que ofrece abrir la página de descarga.
- **`gui/build/package.sh`** (LF), un solo script dentro de trixie:
  1. `npm ci` (probaré `--ignore-scripts`).
  2. Bindings.
  3. Build del frontend.
  4. Exe de Linux y exe de Windows.
  5. `.deb` y `.msi`.
  6. Smoke: `apt install ./soundsync-gui_*.deb` y luego `soundsync-gui -version`.

  Lo pruebo primero en WSL Debian 13 (mismo sistema).
  - `release.yml` lo corre en el mismo job vía `docker run`, sin acciones nuevas.
  - `ci.yml` agrega un job que lo corre en modo check.
  - Sin auto-update: el aviso de versión cubre III.B.1.

### D6. Dependencias (pido aprobación con este plan)

| Dependencia | Dónde | Seguridad |
|---|---|---|
| `github.com/wailsapp/wails/v3` v3.1.0 (fijo) | solo `gui/go.mod` | MIT; 5 advisories corregidas; el binario lleva 2–3 módulos |
| react/react-dom 18, @wailsio/runtime 3.1.0, vite 8, @vitejs/plugin-react 6, typescript 5, @types/* | `gui/frontend` (de la plantilla, lockfile commiteado, `npm ci`) | `npm audit` 0 |
| wails3 CLI v3.1.0 | dev + CI (`go install` fijo, verificado por sum.golang.org) | — |
| wixl 0.106, dev de gtk4/webkitgtk | apt de Debian trixie, solo para build | — |
| imagen `debian:trixie` | CI y release, fijada por digest | imagen oficial |

- Sin librerías nuevas en el módulo raíz.
- Frontend sin router, sin store y sin librería de UI.
- E2E con el kit global `~/.e2e`: nada entra al proyecto.

## Orden de trabajo (tests en verde en cada paso)

1. **Refactor sin cambio de comportamiento:**
   1. i18n atómico y escaneo ampliado.
   2. `fakeapi`, con los tests de `cmd` migrados.
   3. `consent` y `config.Load`/`Dir`.
   4. `session`.
   5. Destinos de `copier`.
   6. Jobs, Observer y Budget de `copier`, con tests `-race` nuevos.
   7. `report` y `update`.

   Los 104 tests pasan con salida idéntica; `-race` corre en WSL Debian.
2. `internal/desktop` con tests (fakeapi + Host grabador).
3. `gui/`:
   - init dentro del repo y poda de darwin/ios/android/docker;
   - `main.go` y `host.go`;
   - chequeo de WebView2 y flag `-version`;
   - ícono.
4. Frontend (skill `impeccable` para el oficio, sin `new-work`):
   - tokens claro/oscuro e i18n;
   - componentes y pantallas;
   - CSP y guard de links.
5. E2E: build `-tags server,e2e` con fakes y specs de Playwright.
6. Pruebas reales con las playlists "Soundsync test" en Windows y en Debian (WSLg). El OAuth se termina en tu Chrome.
7. Empaque: `package.sh`, `.deb` y `.msi`, con instalaciones reales; luego `ci.yml` y `release.yml`.
8. Docs:
   - design doc y `compliance.md`;
   - `todo.md` (4.1–4.6, log y pruebas);
   - README EN/ES con sección GUI (msi/deb, WebView2, SmartScreen);
   - NOTICE de marcas de terceros (los logos no son MIT).
9. Git:
   - rama `phase4-gui`, commits por paso como CharlyCeballos, sin trailer de Claude;
   - **push de la rama al repo privado solo para correr la CI**, sin tags ni release;
   - D.3 (release GUI) y D.1 (repo público) quedan fuera y necesitan tu OK aparte.

## Verificación

- **Go raíz:** `go vet ./...` y `go test ./...` en Windows; `go test -race ./...` en WSL Debian.
  - Motor: destinos y adopción por ID, orden de eventos por job, paro por quota de una playlist hermana, Budget, cancelación, `MaxItems`.
  - Reporte: filas, CSV/JSON, injection, O_EXCL.
  - Consentimiento, versión y `config.Load` (relectura).
  - Desktop:
    - flujos completos con fakes;
    - puerta `busy`;
    - `Write` solo una vez;
    - allowlist de URL;
    - cambio de idioma con `-race`;
    - revocar al desconectar y al cerrar;
    - guardar reporte.
  - i18n ampliado.
- **`gui/`:** `go vet` + build en Windows y Linux, test de la lógica de WebView2, `tsc --noEmit` estricto.
- **E2E Playwright** contra server mode con fakes:
  - puertas de configuración y privacidad;
  - conectar y seleccionar (paginación, seguidas deshabilitadas);
  - 4.4 en sus dos ramas;
  - plan → confirmar → escribir → resumen, y Detener;
  - reporte CSV/JSON con el contenido verificado;
  - ES/EN y temas claro y oscuro;
  - cero errores de consola.
- **Pruebas reales:** la GUI en Windows y en WSLg con cuentas reales; escribir sobre "Soundsync test"; revocar al cerrar.
- **Empaque:**
  - `.msi` en Windows 11: instalar → abrir → actualizar a una versión mayor → desinstalar.
  - `.deb` en WSL Debian: instalar → abrir → quitar.
  - Tu VM Debian 13 queda como prueba opcional.
- **Limpieza:** al terminar cada prueba, matar procesos y liberar puertos (8888/8889 y el de E2E).

## Riesgos

- Wails v3 lleva una semana estable: versión fija y leer las notas antes de subirla.
- wixl puede tener bugs de secuencia; la prueba real de instalar, actualizar y desinstalar decide.
- Cláusula "Synchronization" de los Compliance Tips de Spotify: Soundsync copia metadatos, no grabaciones. Va a `compliance.md` como riesgo aceptado, igual que el reporte.
- El binario Linux se compila contra trixie: soporte oficial solo para Debian 13 o posterior.
- WSLg no es un escritorio Debian real; tu VM sirve como prueba final opcional.
- El `.msi` sin firma muestra SmartScreen.
- El presupuesto de búsquedas es por sesión, no por día (igual que la CLI): una GUI abierta pasada la medianoche del Pacífico no lo recarga hasta reiniciar.

## Cómo quedó (2026-11-02)

Rama `phase4-gui`. Lo que cambió respecto al plan, y por qué:

- **4.4 en línea, no en diálogo.** La pregunta "¿usar tu playlist o crear una nueva?" aparece dentro de la lista de destinos, con las candidatas como opciones y el botón de búsqueda deshabilitado hasta responder. Un modal interrumpía sin necesidad y ocultaba las otras playlists.
- **`Copy.Stop()`.** Cancelar la promesa desde JS perdía el `RunView` parcial; ahora "Detener" cancela el paso en Go y `Plan`/`Write` regresan lo que hicieron, marcado `cancelled`. Cancelar la promesa queda solo para abandonar un login.
- **Re-planear.** `Plan` acepta la fase `resolved` o `planned` (tras un paro se vuelve a buscar sin elegir de nuevo); solo un plan ya escrito exige `Resolve` otra vez.
- **"Crear el archivo de configuración".** La pantalla de configuración crea el `.env` (comentarios, URIs por defecto, IDs vacíos; nunca sobrescribe) y abre su carpeta; al volver a la ventana revisa sola. Un test mantiene `config.Example` igual a `.env.example`.
- **Separación de marcas por vista.** El plan tiene una vista YouTube (videos, "Developed with YouTube", el botón para escribir) y una vista Spotify (canciones, logo de Spotify, `OPEN SPOTIFY`, páginas de 20) que reemplazan toda la región. Escribir solo se ofrece desde la vista YouTube, donde está el resumen exacto.
- **Allowlist.** Además de la del plan: `accounts.spotify.com` y `accounts.google.com`, para el enlace "¿No se abrió el navegador?".
- **Empaque: wixl confirmado.** El `.msi` pasó 17 verificaciones reales en Windows 11 sin admin: instalar, archivos, acceso en Inicio, registro en Programas, versión del exe, abrir la app, actualizar 0.2.0 → 0.2.1 con una sola entrada, rechazar la bajada (1603) y desinstalar sin dejar nada. No hizo falta WiX ni el EULA OSMF.
- **Puerto de depuración solo en desarrollo.** WebView2 ignora `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS` cuando Wails pasa sus propios flags. `debugport_dev.go` (`!production`) abre `--remote-debugging-port` si existe `SOUNDSYNC_DEBUG_PORT`; las builds de release (`-tags production`) nunca lo incluyen. Sirve para probar la ventana real por CDP.
- **Taskfile propio.** Los cuatro Taskfiles de la plantilla (docker, NSIS, MSIX, AppImage, firma) se reemplazaron por uno corto; el release usa `gui/build/package.sh`.
- **Ícono.** Cuadro índigo-violeta con una lista cuya línea central termina en un chevron abierto; el triángulo relleno se descartó por parecerse a un botón de play.
- **`PRODUCT.md`** en la raíz, inferido del repo sin entrevista (pedido del autor); los supuestos están marcados.

## Verificación hecha

- Go raíz: 13 paquetes en verde, `-race` en WSL Debian 13; tests nuevos en `session`, `copier`, `report`, `update`, `consent`, `config` y `desktop`. Cuatro mutaciones a salvaguardas clave (escribir una vez, rechazar playlists ajenas, puerta de consentimiento, `Write` sin duplicar) fueron detectadas por los tests.
- `gui/`: `go vet` en las variantes dev, production, e2e y `GOOS=windows`; test de la lógica de WebView2.
- E2E (Playwright, kit global): 8 escenarios × 3 proyectos (claro, oscuro, ventana mínima) = 24 en verde, y 48 con `--repeat-each=2`.
- Ventana real en Windows (WebView2 por CDP): pinta, CSP activa, cero errores de consola, los enlaces no navegan la ventana, el cierre normal termina con código 0.
- Debian 13 (WSL): `.deb` instala, `soundsync-gui -version` = `v0.2.1`, la GUI pinta en Xvfb, `desktop-file-validate` sin errores, `apt purge` limpio.
- Pendiente con cuentas reales: necesita que el autor acepte la política 2026-11-02 en la app y ponga su `.env` en `%APPDATA%\soundsync` (ver `todo.md`).
