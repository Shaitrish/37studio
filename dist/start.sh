#!/bin/bash
# 37Studio launcher for macOS and Linux: starts Docker Desktop if needed, updates, starts and opens the app.
cd "$(dirname "$0")"
if ! docker info >/dev/null 2>&1; then
  echo "Starting Docker Desktop..."
  open -a Docker 2>/dev/null || systemctl --user start docker-desktop 2>/dev/null
  for _ in $(seq 90); do docker info >/dev/null 2>&1 && break; sleep 2; done
fi
if ! docker info >/dev/null 2>&1; then
  echo "Docker Desktop did not start. Open it yourself, wait until it says Engine running, then open this again."
  read -r -p "Press Enter to close."
  exit 1
fi
echo "Checking for updates..."
docker compose pull
docker compose up -d --remove-orphans
echo "Opening 37Studio..."
for _ in $(seq 30); do curl -s -o /dev/null http://localhost:8080/ && break; sleep 1; done
URL=http://localhost:8080
# Its own window, like an app, when Chrome is there; otherwise the default browser.
if [ -d "/Applications/Google Chrome.app" ]; then open -na "Google Chrome" --args --app=$URL
elif command -v open >/dev/null; then open $URL
else xdg-open $URL; fi
