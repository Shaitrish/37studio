#!/usr/bin/env bash
# Dev mode: backend via air (rebuilds on .go changes) + frontend via Vite (hot reload).
# Open http://localhost:5173. Ctrl+C stops both.
cd "$(dirname "$0")"
set -a; [ -f .env ] && . ./.env; set +a
[ -d web/node_modules ] || (cd web && npm install)
trap 'kill 0' EXIT
air &
cd web && npm run dev
