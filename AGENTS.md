# AGENTS.md

Guidance for AI coding agents (Claude Code, Codex, Cursor and others) and for people working on 37Studio. `CLAUDE.md` imports this file, so keep the rules here.

## What this is

37Studio is a free, open-source (MIT) AI video studio and social media desk that runs on the user's own computer or server in Docker. Claude Code, the editor agent, builds videos as HyperFrames compositions from a company's brand library and knowledge base. The app renders them locally, publishes them to nine platforms and drafts inbox replies. Users bring their own keys. Other agents can run the whole app through its MCP server, and so can the in-app Assistant.

## Layout

```
cmd/37studio/        main: subcommands (gen, clip, transcribe), background loops, every HTTP route
internal/            the Go backend, one package per area (map: docs/architecture.md#code-map)
  config store httpapi remote files textutil   low level: these import nothing of ours
  keys agent gen transcribe timeline templates source
  company project social autopilot costs
  mcp assistant update
web/                 React + Vite + TypeScript UI (pages/, ui.tsx, api.ts, styles.css)
worker/              Cloudflare Worker: product website (public/), update file, Meta sign-in relay, WhatsApp relay, donations (D1)
dist/                the launcher bundle people download (docker-compose.yml, start.bat/.command/.sh)
docs/                platforms.md (per-platform setup), architecture.md, meta-app-review.md
```

## Commands

```bash
go build ./... && go vet ./... && go test ./...   # backend: run all three before you finish
gofmt -l .                                       # must print nothing
cd web && npm run build                          # UI: type-checks (tsc) and builds
./dev.sh                                         # backend (air, hot rebuild) + UI (Vite) on :5173
docker compose up --build                        # the whole app on :8080, as users run it
node --check worker/src.js                       # Worker syntax
```

## How the backend is built

- **Dependencies point down.** Low packages never import higher ones, and nothing imports `cmd`. If a low package needs to call up, add a hook the way `agent.AfterTurn` does: `main` registers `autopilot.Advance` and `project.ClipAdvance`, so the agent never imports them. `go build` rejects import cycles, so you'll know.
- **Handlers** have the shape `func(r *http.Request) (any, error)` and are wrapped with `httpapi.JSON` in `cmd/37studio/main.go`. Return `httpapi.Bad("plain message")` for user errors. A `sql.ErrNoRows` becomes a 404 by itself.
- **Database:** `store.Row`, `store.Rows`, `store.Save(table, allowedCols, id, body)`, settings through `store.Setting`. The schema lives in `internal/store/store.go` and only ever grows with `create table if not exists` and `alter table … add column`; never break existing data folders.
- **Files:** use `files.ProjectDir`, `files.TenantDir` and the rest. Join user-supplied paths only with `files.Within`, which can't escape its root.
- **Outbound calls:** use `remote.Call` / `remote.CallWith`, so the platform's own error message reaches the user. Stream big uploads (`remote.Multipart`, `remote.Uploads`); never read a video into memory.
- **Secrets:** keys live encrypted in the database (`internal/keys`). Name platform secrets `…_SECRET`: `agent.Env` strips every `*_SECRET` variable from the agent's environment. The API shows only a key's last four characters.
- **The MCP server is the whole app.** Every API route a person can use should have a tool in `internal/mcp` (`TestMCP` checks that each tool's path is a real route). When you add a route, add its tool. The Assistant (`internal/assistant`) is Claude Code connected only to that server.
- **Long work runs in the background** (agent turns, renders, scans, imports, posts) and reports through project events and status. Requests return at once.

## Conventions

- Go standard library plus SQLite (modernc, no cgo), with no web framework. The UI is React with plain CSS and Phosphor icons, and no UI kit. Don't add a dependency for what a few lines can do.
- Write the least code that works and match the code around it. Comments explain *why*. Mark deliberate shortcuts with `ponytail:` and say what would make them worth upgrading.
- Logic that can break quietly gets a small test next to it (`*_test.go`). Platform APIs are tested against an `httptest` server, never the real thing.
- UI text and error messages say what happened and what to do next, in plain words. Costs are recorded in USD and shown in the user's currency.
- Keep it local-first: no telemetry, no accounts, and nothing that sends user data anywhere new. Footage and messages stay on the user's machine.
- When setup or configuration changes, update `README.md`, `docs/platforms.md` or `docs/architecture.md`.

## Product rules

- It's open source under MIT: no license keys, trials or paid tiers. The project is funded by donations (`worker/public/donate.html`) and paid setup help.
- Never show made-up numbers (donations, users, results).
- Platform limits are real; don't promise around them. Threads only accepts videos from a public URL, Meta removed posting to Groups and personal profiles, and Reddit and Snapchat have no usable video API.
- The agent asks before destructive or public actions: deleting, publishing, approving Autopilot videos, sending inbox replies.

## Releasing (maintainers)

Bump `Version` in `internal/update/update.go`, then run `./release.sh "What changed"`. It pushes the multi-arch image to ghcr.io, zips `dist/`, writes `worker/public/version.json` and deploys the Worker. Never commit `data/`, `.env` or any backup of them.
