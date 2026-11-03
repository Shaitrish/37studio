#!/usr/bin/env bash
# Packages the desktop app for this machine's architecture into desktop/dist: a .deb for Ubuntu 24.04+ and Debian 13+,
# a .tar.gz of the bare program for other Linux, and their SHA-256 sums. CI runs it on every pull request and attaches
# the files to each GitHub release (.github/workflows/desktop.yml).   Usage: ./desktop/package-linux.sh [version]
set -euo pipefail
cd "$(dirname "$0")/.."
V=${1:-$(sed -n 's/^const Version = "\(.*\)"/\1/p' internal/update/update.go)}
V=${V#v}
arch=$(dpkg --print-architecture) # amd64 or arm64
./desktop/build.sh
out=desktop/dist
rm -rf "$out" && mkdir -p "$out"

umask 022 # files in the package get the usual system permissions, whatever this shell's umask is
root=$(mktemp -d)
chmod 755 "$root" # it becomes / on install
trap 'rm -rf "$root"' EXIT
# The program keeps the name 37studio in its own folder: the agent runs `37studio gen|clip|transcribe` from there.
install -Dm755 desktop/bin/37studio "$root/usr/lib/37studio/37studio"
mkdir -p "$root/usr/bin" && ln -s ../lib/37studio/37studio "$root/usr/bin/37studio"
install -Dm644 brand/37studio-icon-1024.png "$root/usr/share/icons/hicolor/1024x1024/apps/37studio.png"
install -Dm644 /dev/stdin "$root/usr/share/applications/37studio.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=37Studio
Comment=AI video studio and social media desk
Exec=/usr/lib/37studio/37studio
Icon=37studio
Terminal=false
Categories=AudioVideo;Video;
StartupWMClass=37studio
EOF
# Depends: what the window needs, and the agent's tools every supported release has. Recommends: Node.js 22+, which
# Ubuntu 24.04's own nodejs (18) isn't, so it can't be a hard dependency, and the fonts Bangla and emoji render with.
# Claude Code isn't packaged by any distribution: the app's Finish setup list says how to install it.
mkdir -p "$root/DEBIAN"
cat > "$root/DEBIAN/control" <<EOF
Package: 37studio
Version: $V
Architecture: $arch
Maintainer: Shaitrish <jahangir@shaitrish.com>
Installed-Size: $(du -sk "$root/usr" | cut -f1)
Depends: libgtk-4-1, libwebkitgtk-6.0-4, ffmpeg, python3, python3-venv
Recommends: nodejs (>= 22), fonts-noto-core, fonts-noto-color-emoji
Section: video
Priority: optional
Homepage: https://studio.shaitrish.com
Description: AI video studio and social media desk
 37Studio makes videos for a business and runs its social accounts from one app on
 your own computer, with your own AI keys. This is the desktop app: the same studio
 as the Docker image, in its own window, without Docker.
EOF
# Ubuntu 23.10 on stops apps without an AppArmor profile from making user namespaces, and WebKit's web process sandbox
# (bubblewrap) needs one: without it the window crashes on start ("bwrap: setting up uid map: Permission denied")
# unless the app is launched from a terminal that already has the permission. The profile is the one Ubuntu ships for
# Epiphany: it confines nothing, it only names the program and allows the namespace. postinst writes it rather than
# the package shipping it, so a copy someone made by hand for 0.9.0 (docs/desktop.md) is replaced without a prompt.
install -m755 /dev/stdin "$root/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
if [ "$1" = configure ] && [ -d /etc/apparmor.d ]; then
  cat > /etc/apparmor.d/37studio <<'PROFILE'
abi <abi/4.0>,
include <tunables/global>

profile 37studio /usr/lib/37studio/37studio flags=(unconfined) {
  userns,

  include if exists <local/37studio>
}
PROFILE
  if [ -d /sys/kernel/security/apparmor ] && command -v apparmor_parser >/dev/null; then
    apparmor_parser -r -W /etc/apparmor.d/37studio || true
  fi
fi
EOF
install -m755 /dev/stdin "$root/DEBIAN/postrm" <<'EOF'
#!/bin/sh
set -e
if [ "$1" = remove ] || [ "$1" = purge ]; then
  if [ -f /etc/apparmor.d/37studio ] && [ -d /sys/kernel/security/apparmor ] && command -v apparmor_parser >/dev/null; then
    apparmor_parser -R /etc/apparmor.d/37studio 2>/dev/null || true
  fi
  rm -f /etc/apparmor.d/37studio
fi
EOF
dpkg-deb --build --root-owner-group "$root" "$out/37studio_${V}_${arch}.deb"
tar -C desktop/bin -czf "$out/37studio-${V}-linux-${arch}.tar.gz" 37studio
(cd "$out" && sha256sum 37studio[_-]* > "37studio-${V}-linux-${arch}.sha256")
ls -l "$out"
