# Docker and servers

37Studio also runs as one Docker container, with every tool the agent uses bundled in: on your own computer (the
only way on a Mac today), from source, or on a server for a team and for replies and Autopilot around the clock. On
Windows and Linux most people want the [desktop app](desktop.md) instead.

- [On your computer: the ready-made app](#on-your-computer-the-ready-made-app)
- [From source](#from-source)
- [Run it on a server](#run-it-on-a-server)
- [Configuration](#configuration)

You need [Docker Desktop](https://www.docker.com/products/docker-desktop/) (Windows 10 or 11 with WSL 2, macOS on Intel or Apple Silicon) or Docker Engine (Linux), 16 GB of RAM and about 20 GB of free disk, plus a Claude plan or an Anthropic API key.

## On your computer: the ready-made app

1. Install Docker Desktop and open it once.
2. Download [37studio.zip](https://studio.shaitrish.com/37studio.zip) and unzip it into a folder you'll keep, for example `Documents/37Studio`.
3. Double-click `start.bat` (Windows) or `start.command` (Mac; the first time, right-click → Open), or run `./start.sh` (Linux).

The launcher starts Docker, pulls the latest image (about 3 GB the first time) and opens the app at http://localhost:8080, which only accepts connections from your own computer. Every later start checks for updates first. Your work lives in the `data` folder next to the launcher; back it up. The [setup guide](https://studio.shaitrish.com/setup) walks through every step for Windows, Mac and Linux.

## From source

```bash
git clone https://github.com/Shaitrish/37studio.git && cd 37studio
cp .env.example .env      # optional: keys can also be added later in Settings
docker compose up --build
# open http://localhost:8080
```

- **Agent permissions.** From source, the agent uses an allowlist (`safe`: file edits, ffmpeg, `npx hyperframes`, `37studio` and a few shell basics) unless you set `STUDIO37_AGENT_PERMISSIONS=bypass` in `.env`, which lets it run any command inside the container. The launcher and the cloud file set `bypass`, because the container is the sandbox. `safe` keeps the agent from running arbitrary code by accident, but it is not a security boundary: the agent can read the AI keys in its environment and the files in `/data`, and ffmpeg and WebFetch can reach the internet. The allowlist covers Claude Code runs; OpenCode runs follow OpenCode's own permission settings.
- **Windows.** Use Docker Desktop with the WSL 2 backend. Footage under `./data` is a bind mount, so for multi-GB files keep the repo inside the WSL filesystem (`\\wsl$\...`) rather than on `C:\`.

To work on the code with hot reload, see [CONTRIBUTING.md](../CONTRIBUTING.md#running-it-from-source).

## Run it on a server

Run it on a server when a team shares it, or when Autopilot and auto-replies should keep going while your laptop sleeps. [`docker-compose.cloud.yml`](../docker-compose.cloud.yml) runs the app behind [Caddy](https://caddyserver.com), which gets and renews an HTTPS certificate for your domain by itself.

**Server.** A Linux VPS with 16 GB of RAM (recommended) and 4 or more vCPUs: rendering and transcription run on the CPU, so more cores make them faster. Give it 50 GB or more of disk for the image, footage and renders. Both amd64 and arm64 servers work.

1. Point a DNS **A** record (and **AAAA**, if the server has IPv6) for your domain, for example `studio.example.com`, at the server.
2. Install Docker: `curl -fsSL https://get.docker.com | sh`.
3. Open TCP ports **80**, **443** and **3100–3109** in the server's firewall (and in your cloud provider's firewall, if it has one).
4. Get the compose file and create `.env` next to it:

   ```bash
   mkdir 37studio && cd 37studio
   curl -fsSLO https://raw.githubusercontent.com/Shaitrish/37studio/main/docker-compose.cloud.yml
   printf 'STUDIO37_DOMAIN=studio.example.com\nSTUDIO37_PASSWORD=%s\n' "$(openssl rand -base64 24)" > .env
   cat .env                                  # note the password
   mkdir -p data && sudo chown 1000:1000 data  # the app runs as uid 1000 and writes here
   docker compose -f docker-compose.cloud.yml up -d
   ```

5. Open `https://studio.example.com` and sign in with any username and the password. Add your Claude key in **Settings → AI services** (for a subscription token, run `docker compose -f docker-compose.cloud.yml exec studio claude setup-token`).

What is different on a server:

- **Password.** The cloud file won't start without `STUDIO37_PASSWORD`. It is HTTP basic auth on every page, API call and timeline editor, over HTTPS. Use a long random one and share it only with people you trust with the agent (next point).
- **Agent permissions.** The cloud file sets `STUDIO37_AGENT_PERMISSIONS=bypass`, like the launcher: the agent can run any command inside the container. Anyone who can sign in can therefore have it run commands there, read every brand's files in `/data`, and reach the internet. The container runs as an unprivileged user (uid 1000) with no access to the Docker socket or the host's files beyond `./data`. For the allowlist instead, change that line to `safe` in the compose file; commands outside the allowlist are then refused, which can block some tasks. It narrows what the agent runs, but whoever can sign in can still steer the agent, so treat the password as access to everything in the container either way.
- **Sign-in redirects.** The cloud file sets `STUDIO37_PUBLIC_URL=https://<your domain>`, so register `https://<your domain>/api/oauth/<platform>/callback` with each platform. Use the **Web** platform for TikTok.
- **Timeline editor.** The UI opens each project's editor at the page's own address on a port from 3100 to 3109 (`https://studio.example.com:3100`, ...). Caddy serves HTTPS on those ports with the same certificate and passes each one to the same port on the app, so the editor works once the ports are open. Your browser asks for the password once more the first time, because a different port counts as a different site. Up to ten editors can be open at once. If you'd rather not open those ports, everything else still works; make changes by chatting with the agent.
- **Updates.** The sidebar says when a new version is out. Then: `docker compose -f docker-compose.cloud.yml pull && docker compose -f docker-compose.cloud.yml up -d`.
- **Backups.** Everything lives in `./data`: the database, `secret.key` (which decrypts the keys in it), brand files, footage and videos. The database is SQLite in WAL mode, so copy it while the app is stopped, or snapshot the whole disk:

  ```bash
  docker compose -f docker-compose.cloud.yml stop studio
  tar czf 37studio-$(date +%F).tgz data
  docker compose -f docker-compose.cloud.yml start studio
  ```

  The archive holds platform tokens and keys, so store it somewhere private. Agent chat sessions live in the `claude-home` volume; they are only needed to resume old chats.

The launcher and the cloud file use the `latest` image, so a restart can update the app. To stay on a release until you choose to update, replace `latest` with its version (`ghcr.io/shaitrish/37studio:0.5.0`) or its digest.

To build the image from your own checkout on the server, clone the repo and replace the `image:` line in `docker-compose.cloud.yml` with `build: .`.

## Configuration

Settings are environment variables: in `.env` when running from source, under `environment:` in the launcher's `docker-compose.yml`, or in `.env` next to `docker-compose.cloud.yml`. Variables marked **Keys** can also be entered in the app under Settings → AI services or Settings → Social apps, where they are encrypted at rest. AI model IDs and unit prices, the per-turn budget, the voice, and the display currency (USD by default) and its exchange rate are under Settings too.

| Variable | Default | Keys | What |
| --- | --- | --- | --- |
| `ANTHROPIC_API_KEY` or `CLAUDE_CODE_OAUTH_TOKEN` | unset | yes | Claude for the editor agent, captions and replies |
| `GEMINI_API_KEY`, `ELEVENLABS_API_KEY` | unset | yes | Generated images and video; voice, sound effects and music |
| `FAL_KEY` | unset | yes | fal.ai video models (Seedance, Kling, Veo, Wan…); pick one in Settings → AI services → Video clips |
| `OPENCODE_API_KEY`, `OPENAI_API_KEY`, `XAI_API_KEY`, `OPENROUTER_API_KEY` | unset | yes | More models for the editor agent, run through OpenCode: each adds its provider's newest models to the model menus |
| `META_APP_ID`, `META_APP_SECRET`, `META_CONFIG_ID` | unset | yes | Your own Meta app for Facebook Pages + Instagram |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | unset | yes | YouTube |
| `TIKTOK_CLIENT_KEY`, `TIKTOK_CLIENT_SECRET` | unset | yes | TikTok |
| `LINKEDIN_CLIENT_ID`, `LINKEDIN_CLIENT_SECRET` | unset | yes | LinkedIn |
| `LINKEDIN_PAGES` | unset | | `1` lists the LinkedIn company Pages you administer (needs the Community Management API) |
| `X_CLIENT_ID`, `X_CLIENT_SECRET` | unset | yes | X |
| `PINTEREST_APP_ID`, `PINTEREST_APP_SECRET` | unset | yes | Pinterest |
| `PINTEREST_API` | `https://api.pinterest.com` | | `https://api-sandbox.pinterest.com` while your Pinterest app has Trial access |
| `STUDIO37_PASSWORD` | unset | | Basic-auth password for the whole app (any username). Required by the cloud file |
| `STUDIO37_PUBLIC_URL` | `http://localhost:8080` | | Where the platforms send the browser back after sign-in; must match the redirect URIs you register |
| `STUDIO37_DOMAIN` | | | Cloud file only: your domain, for Caddy's certificate and `STUDIO37_PUBLIC_URL` |
| `STUDIO37_AGENT_PERMISSIONS` | `safe` | | `safe` (allowlist) or `bypass` (any command; the launcher and cloud file set it) |
| `STUDIO37_RELAY_URL` | unset | | Your own WhatsApp webhook relay ([deploy it](platforms.md#run-your-own-relay-whatsapp)), if you don't use a [tunnel](platforms.md#whatsapp-through-a-cloudflare-tunnel). `STUDIO37_AUTH_URL` is still read if this is unset |
| `STUDIO37_ALLOW_PRIVATE_URLS` | unset | | `1` lets website scans, video imports, Bluesky/Mastodon servers and every other outbound connection use addresses on your own network (localhost, LAN, `10.x`, `192.168.x`); refused by default. Set it too if you reach the internet through a proxy on your network |
| `STUDIO37_UPDATE_URL` | `https://api.github.com/repos/Shaitrish/37studio/releases/latest` | | Where the update check looks for new releases (GitHub Releases, every 6 hours); a fork can point at its own `…/releases/latest` |
| `STUDIO37_DATA` | `./data` | | Everything lives here: `37studio.db`, `secret.key`, `tenants/`, `projects/`, `templates/` |
| `STUDIO37_ADDR` | `127.0.0.1:8080` | | Listen address (the image sets `:8080`; the compose files publish it on `127.0.0.1` only or behind Caddy) |
| `STUDIO37_STUDIO_PORT` | `3100` | | First port for timeline editors, one per open project (Docker publishes 3100–3109) |
| `STUDIO37_WEB` | `web/dist` | | Built UI |
| `STUDIO37_CLAUDE` | `claude` | | Claude Code binary |
| `HF_VERSION` | `0.8.186` | | Pinned HyperFrames CLI and player runtime |

Environment-only settings (`LINKEDIN_PAGES`, `PINTEREST_API`, `STUDIO37_PUBLIC_URL`, `STUDIO37_RELAY_URL`, `STUDIO37_PASSWORD`) go in `.env` when running from source, or under `environment:` in the launcher's `docker-compose.yml`. Then run the start file again (from source: `docker compose up -d`), which recreates the container with the new settings; a plain restart doesn't pick them up. Keys saved in Settings apply at once.

```yaml
    environment:
      STUDIO37_AGENT_PERMISSIONS: bypass     # already there
      PINTEREST_API: https://api-sandbox.pinterest.com
```
