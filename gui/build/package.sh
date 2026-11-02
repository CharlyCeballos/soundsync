#!/usr/bin/env bash
# Builds the GUI release packages from a clean checkout (todo 4.6):
#   soundsync-gui_<version>_amd64.deb              Debian 13 (GTK4 + WebKitGTK 6.0)
#   soundsync-gui_<version>_windows_amd64.msi      Windows 10/11, per-user, no admin
#
# Usage: gui/build/package.sh vX.Y.Z <output dir> [--smoke]
#
# Runs on Debian 13 (trixie) only: the Linux binary links the system's GTK and
# WebKitGTK, so it must be built against the release it ships for (CI runs it
# in a debian:trixie container). Needs Go 1.26+, Node 20.19+ with npm,
# build-essential, pkg-config, libgtk-4-dev, libwebkitgtk-6.0-dev, wixl and
# wails3 v3.1.0 on PATH. --smoke installs the .deb with apt (as root) and runs
# it, which proves its dependencies resolve on a bare system.
set -euo pipefail

TAG=${1:?usage: package.sh vX.Y.Z <output dir> [--smoke]}
OUT=${2:?usage: package.sh vX.Y.Z <output dir> [--smoke]}
SMOKE=${3:-}
[[ "$TAG" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]] || { echo "tag must look like v1.2.3, got: $TAG" >&2; exit 1; }
VERSION=${TAG#v}

GUI=$(cd "$(dirname "$0")/.." && pwd)
ROOT=$(dirname "$GUI")
mkdir -p "$OUT"
OUT=$(cd "$OUT" && pwd)
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"; rm -f "$GUI"/wails_windows_amd64.syso' EXIT
cd "$GUI"

echo "== frontend"
# Install scripts are not needed by any dependency: refusing them shrinks
# what a compromised package could run on the build machine.
(cd frontend && npm ci --ignore-scripts --no-audit --no-fund)
wails3 generate bindings -clean=true -ts -i -f '-tags production'
(cd frontend && npm run build)

LDFLAGS="-w -s -X main.version=$TAG"

echo "== linux binary"
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
  go build -tags production -trimpath -buildvcs=false -ldflags "$LDFLAGS" -o "$WORK/soundsync-gui" .

echo "== windows binary"
# Stamp the version into the Windows resources (file properties, manifest).
sed -E "s/\"(file_version|product_version|FileVersion|ProductVersion)\": \"[^\"]*\"/\"\1\": \"$VERSION\"/" \
  build/windows/info.json > "$WORK/info.json"
sed -E "s/(assemblyIdentity type=\"win32\" name=\"[^\"]*\" version=\")[^\"]*/\1$VERSION.0/" \
  build/windows/wails.exe.manifest > "$WORK/app.manifest"
wails3 generate syso -arch amd64 -icon build/windows/icon.ico -manifest "$WORK/app.manifest" \
  -info "$WORK/info.json" -out wails_windows_amd64.syso
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -tags production -trimpath -buildvcs=false -ldflags "$LDFLAGS -H windowsgui" -o "$WORK/soundsync-gui.exe" .
rm -f wails_windows_amd64.syso

NOTICE=frontend/src/assets/brands/NOTICE.md

echo "== debian package"
PKG="$WORK/deb"
APP_ID=io.github.charlyceballos.soundsync
install -Dm755 "$WORK/soundsync-gui" "$PKG/usr/bin/soundsync-gui"
install -Dm644 "build/linux/$APP_ID.desktop" "$PKG/usr/share/applications/$APP_ID.desktop"
install -Dm644 build/linux/soundsync-256.png "$PKG/usr/share/icons/hicolor/256x256/apps/$APP_ID.png"
install -Dm644 build/appicon.svg "$PKG/usr/share/icons/hicolor/scalable/apps/$APP_ID.svg"
install -Dm644 "$ROOT/.env.example" "$PKG/usr/share/doc/soundsync-gui/env.example"
install -d "$PKG/usr/share/doc/soundsync-gui"
{ cat "$ROOT/LICENSE"; printf '\n'; cat "$NOTICE"; } > "$PKG/usr/share/doc/soundsync-gui/copyright"
chmod 644 "$PKG/usr/share/doc/soundsync-gui/copyright"
mkdir -p "$PKG/DEBIAN"
cat > "$PKG/DEBIAN/control" <<EOF
Package: soundsync-gui
Version: $VERSION
Section: sound
Priority: optional
Architecture: amd64
Maintainer: Carlos Ceballos <74975887+CharlyCeballos@users.noreply.github.com>
Depends: libc6, libgtk-4-1, libwebkitgtk-6.0-4
Recommends: xdg-utils
Homepage: https://github.com/CharlyCeballos/soundsync
Description: copy Spotify playlists to YouTube (desktop app)
 Desktop app that copies your Spotify playlists to YouTube playlists
 (visible in YouTube Music). Each user brings their own Spotify app and
 Google OAuth client. The command-line tool is the soundsync package.
EOF
DEB="$OUT/soundsync-gui_${VERSION}_amd64.deb"
dpkg-deb --root-owner-group --build "$PKG" "$DEB"

echo "== windows installer"
MSI="$OUT/soundsync-gui_${VERSION}_windows_amd64.msi"
cp "$ROOT/LICENSE" "$WORK/LICENSE.txt"
wixl -a x64 -o "$MSI" \
  -D "Version=$VERSION" \
  -D "ExePath=$WORK/soundsync-gui.exe" \
  -D "IconPath=$GUI/build/windows/icon.ico" \
  -D "LicensePath=$WORK/LICENSE.txt" \
  -D "NoticePath=$GUI/$NOTICE" \
  build/windows/soundsync.wxs

if [[ "$SMOKE" == "--smoke" ]]; then
  echo "== smoke test"
  apt-get install -y --no-install-recommends "$DEB" >/dev/null
  soundsync-gui -version | tee /dev/stderr | grep -F "soundsync-gui $TAG "
  desktop-file-validate "/usr/share/applications/$APP_ID.desktop" 2>/dev/null || true
fi

ls -l "$DEB" "$MSI"
