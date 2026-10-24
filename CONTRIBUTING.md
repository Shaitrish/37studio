# Contributing to 37Studio

Thanks for helping. Bug reports, ideas, docs fixes and pull requests are all welcome.

## Reporting bugs and asking for features

Open an [issue](https://github.com/Shaitrish/37studio/issues). For a bug, include your OS, how you run 37Studio (the launcher, from source, or on a server), the version (Settings → Updates), what you did, what you expected, and what happened. A screenshot or the server log (`docker compose logs`) helps a lot. Leave out API keys, tokens, passwords and customer data.

Found a security problem, for example a way to read another brand's files or a leaked token? Don't open a public issue. Email jahangir@shaitrish.com instead.

## Running it from source

The quickest way is the whole thing in Docker:

```bash
git clone https://github.com/Shaitrish/37studio.git && cd 37studio
cp .env.example .env      # optional: keys can also go in Settings → Keys
docker compose up --build # http://localhost:8080
```

For day-to-day work, run the backend and the UI on your machine with hot reload. You need Go 1.26+, Node 22+, ffmpeg and the Claude Code CLI (logged in).

```bash
cd web && npm install && npm run build && cd ..
./dev.sh                  # backend + UI together, open http://localhost:5173; Ctrl+C stops both. Or separately:
set -a; . ./.env; set +a
air                       # API on :8080, rebuilds on every .go change (go install github.com/air-verse/air@latest)
cd web && npm run dev     # UI with hot reload on :5173, proxied to :8080
```

Outside Docker the agent uses the `safe` allowlist unless `.env` says otherwise; keep it that way on your own machine. Transcription makes its own Python venv under your user cache the first time.

[docs/architecture.md](docs/architecture.md) explains how a job flows through the code and has a map of the files.

## Before you open a pull request

```bash
gofmt -l .                  # prints nothing when formatting is clean
go vet ./... && go test ./...
cd web && npm run build     # type-checks and builds the UI
```

If you changed `docker-compose.cloud.yml`, check it with `STUDIO37_DOMAIN=example.com STUDIO37_PASSWORD=x docker compose -f docker-compose.cloud.yml config`.

## Where things are

- `cmd/37studio/`: the Go entry point: subcommands, background loops and every HTTP route.
- `internal/`: the Go backend, one package per area (`agent`, `social`, `project`, `company`, `mcp`, `store`, …). [docs/architecture.md](docs/architecture.md#code-map) maps them. Tests sit next to the code as `*_test.go`.
- `web/`: the React UI (Vite, TypeScript).
- `worker/`: the Cloudflare Worker for studio.shaitrish.com, the static product website. Copies never contact it.
- `relay/`: the optional WhatsApp webhook relay that users deploy to their own Cloudflare account ([docs/platforms.md](docs/platforms.md#run-your-own-relay-whatsapp)).
- `dist/`: the launcher bundle people download. `docs/`: setup and architecture. `brand/`: logos.


## Pull requests

Fork the repo, make your change on a branch, and open a pull request against `main`. The `ci` check runs the commands above and has to pass before merging. We squash-merge, so your pull request's title becomes the commit message: make it say what changed.

- **Small and focused.** One change per pull request is much easier to review. For anything big (a new platform, a new page, a schema change), open an issue first so we can agree on the shape.
- **Match the code around you.** The backend is the Go standard library plus SQLite, with no framework; the UI is React with plain CSS and Phosphor icons. Keep it that way unless there's a strong reason: don't add a dependency for something a few lines can do.
- **Keep it local-first.** 37Studio runs on people's own computers and servers with their own keys. Changes shouldn't send their data anywhere new, add accounts or telemetry, or need a hosted service.
- **Test what can break.** Logic with branches (parsers, schedulers, cost maths, anything touching tokens or files) gets a small test next to it, like the existing `*_test.go` files. Platform calls are tested against an `httptest` stand-in, never the real API.
- **Write for the person using it.** UI text and error messages say what happened and what to do next, in plain words.
- **Update the docs** when you change how something is set up or configured: the [README](README.md), [docs/platforms.md](docs/platforms.md) or [docs/architecture.md](docs/architecture.md).

## Adding a social platform

Each platform follows the same pattern; LinkedIn, X and Pinterest (OAuth) and Bluesky and Mastodon (app password or token) in `internal/social/platforms.go` are good ones to copy.

1. **Credentials.** Add the platform's env names to `Names` in `internal/keys/keys.go`, so they can be entered in Settings → Keys (encrypted at rest), and to `.env.example`. Name the secret `…_SECRET`: `agent.Env` keeps every `*_SECRET` variable away from agents.
2. **Connecting (OAuth).** In `internal/social/social.go`, add the credential pair to `ProviderReady`, a `case` in `OAuthStart` that builds the platform's authorize URL with `redirectURI(prov)` and the state key (`st.pkc` is the PKCE verifier if the platform needs one), and a `case` in `OAuthCallback` that calls your `xxxAccounts(code)`. That function trades the code for tokens and returns `[]socialAccount`: one per account the person can post to, with `token` and `refresh` filled in (they never reach the browser). The redirect path is `/api/oauth/<provider>/callback`.
   **Connecting (no OAuth).** A `connectXxx` handler that checks the credentials with one API call and saves the account, plus its route in `cmd/37studio/main.go`, like `ConnectBluesky`.
3. **Tokens.** If the platform's tokens expire, add a `case` to `accessToken` that refreshes them and stores the new ones.
4. **Posting.** A `case` in `publish()` that calls your `postXxx(…)`. It uploads the file and returns the platform's post ID, a link to the post, and an error. Use `call`/`callWith`, so the platform's own error message shows on the Posts tab, and stream the file instead of reading it into memory (`upClient` has no timeout, for slow uplinks).
5. **Captions.** Add the platform's key (and a title key, if it needs one) to `socialSpec`, with a one-line style hint that Claude follows.
6. **UI.** In `web/src/pages/Social.tsx`, add the platform to `platforms` (label, Phosphor icon, caption length limit) and its Connect button: `providers` for OAuth, or `pasted` (the form's fields) for app-password platforms.
7. **Docs.** The README's platforms table, Social publishing table and configuration table, and a section in `docs/platforms.md` with the developer-app steps, scopes and redirect URI.
8. **A test.** At least the parts that can break quietly: request shape and error handling against an `httptest` server, and any text escaping the platform needs (see `platforms_test.go`).

## Adding a generation provider

The agent only knows `37studio gen image|video|voice|sfx|music`. Which provider answers is the app's choice, so a new one needs no change to the harness prompt.

1. **Key.** Add its env name to `Names` in `internal/keys/keys.go` and to `.env.example`. It has to be an API key: subscriptions to video tools (Runway, Kling, Google AI Pro) don't include API access.
2. **Client.** A small file in `internal/gen/`, like `gemini.go` or `eleven.go`, that sends the request, waits for the result and writes it to the output file.
3. **Routing.** Choose it in `gen.Image` or `gen.Video` from the model setting (`image_model`, `video_model`), for example by a model-name prefix, so people switch providers in Settings.
4. **Cost.** Call `costs.Record` with the provider, the model and a unit price from Settings, so the project and the Costs page show it.
5. **A test.** Request shape and error handling against an `httptest` server.

## Releasing

Maintainers only: bump `Version` in `internal/update/update.go`, then `./release.sh "What changed"`. [docs/architecture.md](docs/architecture.md#releasing) has the details.

## License

By sending a pull request you agree that your contribution is licensed under the project's [MIT License](LICENSE).
