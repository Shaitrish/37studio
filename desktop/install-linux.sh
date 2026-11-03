#!/usr/bin/env bash
# Builds the desktop app and adds it to this Linux user's app menu (no sudo): the program goes to
# ~/.local/lib/37studio, the menu entry and icon under ~/.local/share. Run it again after pulling changes.
# ponytail: until there are .deb and AppImage packages. To remove: ./desktop/install-linux.sh --uninstall
set -euo pipefail
cd "$(dirname "$0")/.."
app="$HOME/.local/lib/37studio"
entry="$HOME/.local/share/applications/37studio.desktop"
icon="$HOME/.local/share/icons/hicolor/1024x1024/apps/37studio.png"

if [ "${1:-}" = "--uninstall" ]; then
  rm -rf "$app" "$entry" "$icon"
  echo "Removed 37Studio from the app menu. Your brands and videos in ~/.local/share/37studio are still there."
  exit 0
fi

./desktop/build.sh
mkdir -p "$app" "$(dirname "$entry")" "$(dirname "$icon")"
# Keep the name 37studio: the agent runs `37studio gen|clip|transcribe` from the folder the app sits in.
install -m 755 desktop/bin/37studio "$app/37studio"
cp brand/37studio-icon-1024.png "$icon"
cat > "$entry" <<EOF
[Desktop Entry]
Type=Application
Name=37Studio
Comment=AI video studio and social media desk
Exec=$app/37studio
Icon=37studio
Terminal=false
Categories=AudioVideo;Video;
StartupWMClass=37studio
EOF
update-desktop-database "$(dirname "$entry")" 2>/dev/null || true
# Ubuntu 23.10+ needs an AppArmor profile before WebKit's sandbox can start, or the window crashes when opened from
# the menu. Writing to /etc needs root, so say how instead of asking for a password here.
if [ "$(cat /proc/sys/kernel/apparmor_restrict_unprivileged_userns 2>/dev/null)" = 1 ] && ! grep -qs "$app/37studio" /etc/apparmor.d/*; then
  echo "Ubuntu needs one more step so the window can open from the menu (it asks for your password):"
  echo "  printf 'abi <abi/4.0>,\\ninclude <tunables/global>\\nprofile 37studio-local %s flags=(unconfined) {\\n  userns,\\n}\\n' '$app/37studio' | sudo tee /etc/apparmor.d/37studio-local"
  echo "  sudo apparmor_parser -r /etc/apparmor.d/37studio-local"
fi
echo "Installed. Open 37Studio from your app menu (press the Super key and type 37Studio)."
