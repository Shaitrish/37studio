# How 37Studio works

The internals: how a job flows, the agent, HyperFrames, transcription, costs, the project folder, a code map and the release process. The [README](../README.md) covers installing and using it.

- [The pieces](#the-pieces)
- [How a job flows](#how-a-job-flows)
- [The editor agent](#the-editor-agent)
- [Why HyperFrames, not Remotion](#why-hyperframes-not-remotion)
- [Transcription](#transcription)
- [Autopilot](#autopilot)
- [Costs](#costs)
- [Project folder](#project-folder)
- [Code map](#code-map)
- [Releasing](#releasing)

## The pieces

**Claude Code does the editing. HyperFrames renders. Go serves. React is the UI.** Everything runs in one Docker container:

- **`37studio`**, one Go binary: the HTTP API, the web UI's files, SQLite (`data/37studio.db`, pure Go, no cgo), the background loops (post queue, Autopilot, the video queue, inbox auto-replies), and the CLI commands the agent calls (`37studio gen`, `37studio clip`, `37studio transcribe`).
- **Claude Code** (`claude -p`), started per agent turn in the project's folder. OpenCode is a second harness for DeepSeek models.
- **HyperFrames** (`npx hyperframes`), pinned by `HF_VERSION`: compositions, the live preview player, the timeline editor (HyperFrames Studio) and `render`.
- **Chrome** (HyperFrames' own Chrome for Testing on amd64, Playwright's headless shell on arm64), **ffmpeg**, and a Python venv with **faster-whisper** and **yt-dlp**.
- **The web UI** (`web/`), React with plain CSS, built to static files the Go server serves.

Outside the container there is only `worker/`, a Cloudflare Worker: the product website, `version.json`, the shared Meta app's login exchange and the WhatsApp webhook relay (see [platforms.md](platforms.md#run-your-own-relay)).

## How a job flows

1. **Company.** *Add from website or folder* builds a knowledge base of the company in two stages. First `hyperframes capture` (no tokens) grabs pages, assets, fonts, screenshots, CSS variables and computed styles. Then an agent sorts every asset into the brand library and writes:
   - `design.md`: the design system as a HyperFrames design spec (exact colors by role, type scale, fonts, radii, spacing, shadows, button/card/nav styles). It is copied into every project's `comp/`, where HyperFrames treats it as brand truth.
   - `knowledge.md`: what the business does, offers, proof, tone, site structure and video ideas.
   - `assets.json`: a caption for every file in the library (what it shows, which site section it came from, source URL, what it is good for). Editors can fix captions or move files between folders in the UI. If `GEMINI_API_KEY` is set, capture adds Gemini Vision descriptions.
   - `profile.json`, which fills the company form (only fields still empty).

   Brand library folders: `logo`, `people`, `images`, `graphics` (SVG icons and illustrations), `videos`, `fonts`, `audio`, `design` (screenshots, raw tokens), `other`. You can also add a company by hand. Companies are called tenants in the code.

   **From a project folder.** The same dialog takes the company's own folder (a website, app or docs project), with or without a website. The browser uploads a copy to `tenants/<id>/source/`, since Docker can't see the user's disk. Dependencies, build output, hidden files and anything that looks like a secret (`.env`, keys, databases) are left out twice: in the browser, and in `source.go`, because every agent may read the folder and so send it to Anthropic. The scan reads the README, manifests, marketing pages, locale strings, theme and CSS tokens and public assets, and copies real logos, images and fonts into the library. Every project agent also gets the folder as a read-only reference, so a video about a feature can use its real screens, UI text and flows. Choosing the folder again replaces the copy.
2. **Campaign (optional).** Groups a company's videos around one launch, offer or product. Its brief reaches every video's agent.
3. **Project.** *New project* is a short chat: company, campaign (or a new one), what to make, which page or feature it is about (a URL, or a feature name the agent finds on their site), where it runs, length, brief, footage (optional) and quality. No footage is fine: the agent builds from the page, the knowledge base and the brand library.
4. **Agent.** The app runs `claude -p` in the project folder. The agent probes and transcribes the footage, asks up to 3 questions if it is blocked, cuts with ffmpeg, and builds a HyperFrames composition in `comp/`.
5. **Preview and edit.** The project page plays `comp/index.html` live in `<hyperframes-player>`, with no render needed. Chat with the agent to iterate, or click **Edit timeline** to open HyperFrames Studio and edit by hand: drag, trim and split clips, change text, timing and audio. Studio saves to the same files; on its next turn the agent reads your edits from `hyperframes history`, builds on them, and can ask Studio which element you selected ("make *this* bigger").
   Studio only binds `127.0.0.1` and serves from `/`, so `studio.go` gives each open editor its own port, starting at `STUDIO37_STUDIO_PORT` (3100), and reverse-proxies it behind the app's password. The UI opens it at the page's own host and protocol on that port. Docker publishes 3100–3109, so up to ten editors can be open at once.
6. **Export.** *Export final* runs `hyperframes render` into `out/`. Anything in `out/` is a finished video or image, and the **Library** page lists every project's finished files in one place, filterable by company and type.
   **Templates.** *Save as template* (on a project, or on a video in the Library) copies the project's `comp/` into `templates/<id>/`, with a thumbnail from its export (`template.go`). Dot folders such as HyperFrames history are left out. A template belongs to one company, or to every company. New project asks "Start from a template?" and puts a copy in the new project's `comp/`; the first prompt tells the agent to keep the design (layout, fonts, motion, captions, intro and outro) and swap in the new content. Videos queued from a templated source inherit it.
   **Several videos from one source.** *New project* asks "one video, or several?". The agent transcribes the source, plans the videos (asking the editor to confirm unless the brief says go), and queues each with `37studio clip --title … --from … --to … --brief …` (`clip.go`). Each video becomes its own project with the source's company, campaign, format, length and quality. It shares the recording by hard link and gets a copy of the transcript, so nothing is copied or transcribed twice. `clipLoop` builds and exports the queued videos one at a time, after any Autopilot video. The source project lists them on its *Videos* tab.
7. **Publish.** *Publish* posts a final video to any mix of the company's connected accounts, now or on a schedule, with a caption per platform (written by Claude on request, or by the agent when you ask in chat; saved in `work/social.json`). `postLoop` uploads due posts one at a time. See [platforms.md](platforms.md).
8. **Autopilot (optional).** A content plan per company makes, renders, captions and schedules videos on its own. See [Autopilot](#autopilot).

## The editor agent

The agent's instructions (the harness) are in `internal/agent/agent.go`: `harness` holds the system prompt and `presets` the per-job-type briefs (hook reel, captioned clip, explainer, promo, thumbnails, custom). Tune them there.

Each turn is one headless Claude Code run: `--output-format stream-json`, streamed into the project chat; `--max-budget-usd` from Settings caps its spend; the user's own Claude Code hooks are disabled so they can't steer it; the prompt goes over stdin. Sessions are resumed, so the agent keeps its context across turns (they live in the `claude-home` volume).

- **Quality tiers** pick the model and effort: Quick is Haiku 5.5 (low), Draft is Sonnet 5.5 (medium), Standard is Opus 5.5 (high), Premium is Opus 5.5 (xhigh). The OpenCode tiers run DeepSeek models for cheaper routine edits (`opencode.go`); they need an OpenCode login (`docker compose exec studio opencode auth login`). The tier can change per message.
- **Permissions.** `STUDIO37_AGENT_PERMISSIONS=safe` (the default) gives the agent an allowlist: file edits, ffmpeg/ffprobe, `npx`, `37studio`, python3, node and a few shell basics. `bypass` lets it run any command; the launcher and the cloud compose file set it, because the container is the sandbox. Claude Code refuses bypass mode as root, so the image runs as the unprivileged `node` user. Agents don't get the platform apps' secrets (variables ending in `_SECRET`).
- **Generated media.** The harness lists `37studio gen image|video|voice|sfx|music` under "Paid AI media" and tells the agent to use them only when the footage and brand library can't do the job; on Autopilot it decides alone. The agent never picks a provider or model: `genAny` sends images and video to Gemini (`image_model`, `video_model` in Settings) and voice, sound effects and music to ElevenLabs (`voice_model`), so a new provider needs no prompt change (see CONTRIBUTING.md). Without the key the command fails with "GEMINI_API_KEY is not set" and the agent carries on without it. Each generation's cost is recorded against the project in `$STUDIO37_PROJECT`.
- **Captions and replies** use one tool-less Claude turn each (Sonnet, low effort), not the agent.

## Why HyperFrames, not Remotion

- **Agent-native.** A composition is plain HTML with `data-*` timing attributes, which Claude writes reliably. HyperFrames also ships Claude Code skills for compositions, captions, keyframes and media.
- **Live preview for free.** `@hyperframes/player` plays the agent's HTML directly. Remotion would need every agent edit bundled into a React project before it could preview.
- **Built-in tools for this work.** It includes `transcribe`, `remove-background` for cut-outs of people, `snapshot` for thumbnails, and `check` for linting.
- **License.** HyperFrames is open source. Remotion needs a paid company license once a team has more than 3 people.

## Transcription

The agent transcribes speech with `37studio transcribe <file> --language <code> --out work/transcript`. It runs faster-whisper ([internal/transcribe/asr/transcribe.py](../internal/transcribe/asr/transcribe.py), embedded in the binary) on the user's computer, so it costs nothing and the audio stays local; paid APIs (ElevenLabs, Gemini) are used only when the editor asks for them.

- **Model by hardware.** With an NVIDIA GPU visible it uses `large-v3` (float16). On a CPU it uses `large-v3-turbo` (int8), which is several times faster for a small accuracy loss. `--quality best` forces `large-v3`; `--quality fast`, or less than 4 GB of memory, uses `small`, which is weak on less common languages.
- **Inside Docker every OS is the same Linux CPU path.** Docker Desktop on a Mac can't reach the Apple GPU, and the shipped compose files pass no NVIDIA GPU (the image also lacks the CUDA libraries), so the GPU branch only applies when the binary runs outside Docker. The model and thread count follow the container's memory and cores. On a 12-core laptop it ran at about 1.3× the recording's length (a 59-second Bangla clip took 80 seconds), so a 2-hour class takes 2–3 hours. The tool prints progress and an estimate. It runs detached and the command waits at most 9 minutes, so the job outlives one Bash call and the agent's turn; running the same command again joins it or reuses the finished transcript.
- **Why not batched mode.** faster-whisper's batched pipeline was no faster on a CPU at beam 5, and at beam 1 it dropped sentences and slipped Devanagari letters into Bangla.
- **Long-class settings.** Voice activity detection skips silences, `condition_on_previous_text` is off so hours of audio don't loop on one phrase, and word timestamps give exact cut points for reels.
- **Outputs.** `transcript.json` (segments and words), `.srt`, and `.txt` with timestamps. A second run with the same file and settings reuses the result.
- **Subtitles instead.** If an `.srt` or `.vtt` sits next to the video, the command converts it at once instead of transcribing (segment timings, no word timings). It matches by file name, or uses a lone subtitle file beside a lone video. Editors add one from the Media tab or the New project footage step; YouTube imports bring their own.
- **Skip or stop.** The New project flow asks whether to transcribe. The project page shows progress with a **Cancel** button, and the Media tab can switch transcription off or back on. Both write `work/transcript/skip`, which stops a running job (it is killed as a process group) and makes `37studio transcribe` tell the agent to work without a transcript until it runs with `--force`.
- **Import from a link.** A YouTube or Google Drive link (in New project or the Media tab) downloads into `input/` with yt-dlp (`import.go`, in the same venv; `--js-runtimes node` for YouTube's player challenges). Video is capped at 1080p and merged to MP4, and YouTube's subtitles come along in the video's language. Progress shows on the project page and can be cancelled. Intermediate files stay in `work/download/`, which is removed afterwards. From New project, the agent starts once the download finishes. Google Drive files must be shared as "Anyone with the link".
- **Setup.** The image ships the Python environment (`/opt/asr`, `STUDIO37_ASR_PYTHON`). The model (about 1.6 GB for turbo) downloads into the cache volume the first time. Outside Docker the command makes its own venv under the user cache.

## Autopilot

Each company can have content plans: what the videos are about, video type, length, format, language, quality, which days and time to post, which accounts, and whether to wait for approval. Three hours before each slot the scheduler starts a project; the agent picks a topic the company hasn't covered yet, builds and checks the video in one turn without asking questions, and writes the captions. The app renders it and queues the posts. With approval on (the default) the video waits on its project page and on Overview for one click; otherwise it posts at the slot.

It runs one video at a time and only while 37Studio is running. A slot that passed while the computer was off is skipped, not posted late. Times are in the browser's time zone, saved with the plan.

## Costs

```
cost = Claude agent cost      (total_cost_usd reported by the CLI for every turn, plus captions and inbox drafts)
     + OpenCode runs          (as reported by OpenCode)
     + Gemini generations     (per image / per second of video, unit prices in Settings)
     + ElevenLabs generations (per 1,000 characters, second or minute, unit prices in Settings)
```

Rendering and storage run on your own computer, so they cost nothing. Costs are recorded in USD, what the providers bill, and shown in the currency set in Settings (USD by default; set any other code and an exchange rate). Every project, campaign and company shows what it has cost; the **Costs** page breaks spend down by provider, company and month.

- **Quality tiers** are the main lever on cost (see [The editor agent](#the-editor-agent)).
- **Budget cap.** Each agent turn has a hard budget on Claude's spend (`--max-budget-usd`, $25 by default, set in Settings). Gemini and ElevenLabs generations are recorded but not capped.
- **Inbox auto-replies** send at most 20 replies per two-minute round.

## Project folder

```
data/
  37studio.db           SQLite (WAL): companies, projects, posts, accounts and their tokens, settings, encrypted keys
  secret.key            the key that encrypts Settings → Keys; keep it with the database
  tenants/<id>/         a company's brand library (assets/), design.md, knowledge.md, replies.md, source/
  projects/<id>/
    input/              raw uploads (the agent may not modify these)
    work/               agent scratch: audio, transcripts, cut ranges, social.json captions
    comp/               HyperFrames composition (live preview)
    out/                finished videos and images
    out/drafts/         draft renders and generated media
  templates/<id>/       saved templates
```

## Code map

The Go code builds one binary, `37studio`, that serves the app and runs the agent's helper commands (`gen`, `clip`, `transcribe`). `cmd/37studio` wires it together; everything else is in `internal/`, one package per area. Dependencies point downward: the low packages (`config`, `store`, `httpapi`, `remote`, `files`, `textutil`) import nothing of ours, and nothing imports `cmd`. The agent tells Autopilot and the clip queue that a turn ended through `agent.AfterTurn`, so it never imports them.

| Package | What |
| --- | --- |
| `cmd/37studio` | `main`: subcommands, background loops, every HTTP route |
| `internal/config` | Settings from the environment and `.env` |
| `internal/store` | SQLite schema, row helpers, settings and their defaults |
| `internal/httpapi` | JSON handler adapter, errors, request helpers, basic auth |
| `internal/remote` | Outbound API calls (JSON, forms, streamed multipart) and readable platform errors |
| `internal/files` | Data folder layout, uploads, file listing and serving (with the player runtime injected) |
| `internal/textutil` | Small string helpers |
| `internal/keys` | Provider keys and platform credentials, encrypted at rest (AES-GCM, key file in the data folder) |
| `internal/agent` | Presets, quality tiers, harness prompt, Claude Code and OpenCode runs (stream-json to chat events), render, `37studio gen` |
| `internal/gen` | Gemini (Nano Banana images, Omni video) and ElevenLabs (voice, sound effects, music) |
| `internal/company` | Companies and their brand library, the website/folder scan that writes `design.md` and `knowledge.md`, campaigns |
| `internal/source` | A company's project folder: upload into `source/`, skip rules for dependencies and secrets |
| `internal/project` | Projects, uploads, YouTube/Drive imports, several videos from one source (`37studio clip`), the Library |
| `internal/templates` | Save a project's composition as a template and seed new projects from it |
| `internal/transcribe` | `37studio transcribe`: local faster-whisper (`asr/transcribe.py`, embedded), subtitles, detached long runs |
| `internal/timeline` | HyperFrames Studio per project, proxied on its own port behind the password |
| `internal/social` | Connecting accounts, AI captions, the post queue and uploads for every platform; the inbox and WhatsApp |
| `internal/autopilot` | Content plans, scheduler and the build → render → captions → approve pipeline |
| `internal/costs` | Usage records, project cost and the spend report |
| `internal/mcp` | The MCP server: the API as tools for other agents and the Assistant |
| `internal/assistant` | The Assistant chat: Claude Code connected to the MCP server |
| `internal/update` | Version and the update check |
| `web/src` | React UI: `pages/Project.tsx` is the editor workspace, `pages/Social.tsx` the Social tab |
| `worker/` | Cloudflare Worker: product website, `version.json`, Meta login exchange, WhatsApp webhook relay (D1) |
| `dist/`, `release.sh` | The launcher bundle people download, and the script that publishes a release |
| `docker-compose.yml`, `docker-compose.cloud.yml` | Run from source on this computer; run on a server behind Caddy with HTTPS |

## Releasing

Maintainers bump `Version` in `internal/update/update.go`, then run `./release.sh "What changed"`. It builds and pushes `ghcr.io/shaitrish/37studio:<version>` and `:latest` for `linux/amd64` and `linux/arm64` (arm64 under QEMU, through a `studio37` buildx builder it creates; needs about 15 GB of free disk), zips `dist/` into the download, writes `version.json`, and deploys the Worker. Set `IMAGE=` for another registry and log in with `docker login ghcr.io` first. Every copy reads `version.json` at most every 6 hours and shows "Version X available" in the sidebar; the launcher pulls the new image on its next start.

Chrome for Testing has no Linux arm64 build, so on arm64 HyperFrames renders with Playwright's pinned arm64 headless shell (`HYPERFRAMES_BROWSER_PATH`), as HyperFrames' own `Dockerfile.render` does.

### Donations

[studio.shaitrish.com/donate](https://studio.shaitrish.com/donate) (`worker/public/donate.html`) takes donations by bKash Send Money. A personal bKash number has no API, so donors report the TrxID and amount, which the Worker stores in the D1 `donations` table as `pending` (`worker/schema.sql`; apply it with `npx wrangler d1 execute studio37 --remote --file schema.sql`). Only confirmed rows count toward the total on the page. After checking a TrxID in the bKash app:

```bash
cd worker
npx wrangler d1 execute studio37 --remote --command "select trx, amount, name, message, datetime(at/1000,'unixepoch') from donations where status='pending'"
npx wrangler d1 execute studio37 --remote --command "update donations set status='confirmed' where trx='BJK7XY2A4Q'"   # or 'rejected'; fix amount here too if it differs
```

GitHub's Sponsor button points at the same page (`.github/FUNDING.yml`).
