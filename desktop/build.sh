#!/usr/bin/env bash
# Builds the desktop app for this computer into desktop/bin/37studio. Needs Go 1.26+, Node 22+ and, on Linux,
# GTK 4 and WebKitGTK 6.0 headers (Ubuntu/Debian: sudo apt install libgtk-4-dev libwebkitgtk-6.0-dev pkg-config).
# The binary must be named 37studio: the agent runs `37studio gen|clip|transcribe` from the folder it sits in.
set -euo pipefail
cd "$(dirname "$0")/.."
(cd web && { [ -d node_modules ] || npm ci; } && npm run build)
app=cmd/37studio-desktop
rm -rf "$app/web" && cp -r web/dist "$app/web"
cp brand/37studio-icon-1024.png "$app/icon.png"
mkdir -p desktop/bin
(cd "$app" && go build -o ../../desktop/bin/37studio .)
echo "Built desktop/bin/37studio"
