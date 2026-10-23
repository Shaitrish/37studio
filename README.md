<p align="center"><img src="brand/37studio-mark.svg" width="72" alt="37Studio"></p>

<h1 align="center">37Studio</h1>

<p align="center"><b>An AI video studio and social inbox that runs on your own computer.</b><br>
Videos made, posted and answered, with your own keys and no monthly bill.</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-d4502a" alt="MIT License"></a>
  <a href="https://studio.shaitrish.com"><img src="https://img.shields.io/badge/website-studio.shaitrish.com-1f1c27" alt="Website"></a>
  <img src="https://img.shields.io/badge/runs%20on-Windows%20%C2%B7%20macOS%20%C2%B7%20Linux-555" alt="Windows, macOS, Linux">
</p>

![The project page: the editor agent's chat beside a live preview of a reel](worker/public/shots/project.webp)

37Studio turns long footage, a website or a one-line brief into hook reels, captioned clips, motion-graphics explainers, promos, square posts and thumbnails in a company's own brand. It posts them to Facebook and Instagram (YouTube and TikTok are built, waiting on platform review), on a schedule or on Autopilot, and drafts a reply to every message and comment from the company's knowledge base. Every project shows exactly what it cost in AI usage.

**Claude Code does the editing. HyperFrames renders. Go serves. React is the UI.** Everything runs in one Docker container on your computer. Footage, brands and messages stay there; API keys are encrypted on disk.

## Features

- **Brand library from a website or folder.** Paste a URL (or pick the company's own project folder) and get its logos, people, photos, fonts and exact colours sorted and captioned, plus a design system (`design.md`) and a knowledge base (`knowledge.md`).
- **An editor agent.** Claude Opus 5.5 builds each video as an animated [HyperFrames](https://github.com/heygen-com/hyperframes) composition, checks its own frames and takes changes in plain words. Edit by hand on a real timeline when you want to.
- **Long recordings into many videos.** A 2-hour class becomes ten reels. Transcription runs locally with faster-whisper: free, private, about 100 languages.
- **Generated media.** Gemini images and video clips; ElevenLabs voiceover, sound effects and music.
- **Publishing and Autopilot.** Post to any mix of a company's accounts now or on a schedule, or let a content plan make a fresh video before every slot and wait for one-click approval.
- **One inbox.** Messenger, Instagram and WhatsApp messages and comments, with replies drafted in the customer's language, following the company's own rules. Off, drafts to approve, or fully automatic, per company.
- **Real costs.** Every project, campaign and company shows its AI spend in your currency. No markup: you pay Anthropic, Google and ElevenLabs directly.

## Quick start

You need [Docker Desktop](https://www.docker.com/products/docker-desktop/) (Windows, macOS on Intel or Apple Silicon) or Docker Engine (Linux), 16 GB of RAM and about 20 GB of free disk, plus a Claude plan or an Anthropic API key.

**The ready-made app.** Download [37studio.zip](https://studio.shaitrish.com/37studio.zip), unzip it, and double-click `start.bat` (Windows) or `start.command` (Mac), or run `./start.sh` (Linux). The launcher starts Docker, pulls the latest image and opens the app at http://localhost:8080. The [setup guide](https://studio.shaitrish.com/setup) walks through every step.

**From source.**

```bash
git clone https://github.com/Shaitrish/37studio.git && cd 37studio
cp .env.example .env      # optional: keys can also be added later in Settings → Keys
docker compose up --build
# open http://localhost:8080
```

Then add a Claude key in **Settings → Keys** (an Anthropic API key, or a Claude subscription token from `claude setup-token`) and your first company.

- **Agent permissions.** Inside Docker, `STUDIO37_AGENT_PERMISSIONS=bypass` lets the agent run any command; the container is its sandbox. The default, `safe`, is an allowlist: ffmpeg, `npx hyperframes`, file edits, python and node.
- **Windows.** Use Docker Desktop with the WSL2 backend. Footage under `./data` is a bind mount, so for multi-GB files keep the repo inside the WSL filesystem (`\\wsl$\...`) rather than on `C:\`.
- **Claude subscriptions.** Anthropic's terms govern using a Claude subscription in other apps; an API key is the fully supported route.

### Local development

Prerequisites: Go 1.26+, Node 22+, ffmpeg, and the Claude Code CLI (logged in).

```bash
cd web && npm install && npm run build && cd ..
./dev.sh                  # backend + frontend together; Ctrl+C stops both. Or separately:
set -a; . ./.env; set +a
air                       # API on :8080, rebuilds on every .go change (go install github.com/air-verse/air@latest)
cd web && npm run dev     # UI with hot reload on :5173, proxied to :8080
```

Tests: `go test ./...` for the backend and `cd web && npm run build` (which type-checks) for the UI.

## Configuration

| Env var | Default | |
| --- | --- | --- |
| `STUDIO37_DATA` | `./data` | Everything lives here: `37studio.db`, `tenants/`, `projects/` |
| `STUDIO37_ADDR` | `127.0.0.1:8080` | Listen address (Docker sets `:8080` and publishes it on `127.0.0.1` only) |
| `STUDIO37_WEB` | `web/dist` | Built UI |
| `STUDIO37_CLAUDE` | `claude` | Claude Code binary |
| `STUDIO37_AGENT_PERMISSIONS` | `safe` | `safe` or `bypass` |
| `STUDIO37_PASSWORD` | unset | Basic-auth password for the whole app |
| `HF_VERSION` | `0.8.186` | Pinned HyperFrames CLI and player runtime |
| `STUDIO37_STUDIO_PORT` | `3100` | First port for timeline editors (one per open project) |
| `STUDIO37_PUBLIC_URL` | `http://localhost:8080` | Where Facebook, Google and TikTok send the browser back after sign-in; must match the redirect URIs you register |
| `STUDIO37_AUTH_URL` | `https://studio.shaitrish.com` | The relay for the shared Meta app's sign-in and for WhatsApp messages. See [Run your own relay](#run-your-own-relay) |
| `STUDIO37_UPDATE_URL` | `https://studio.shaitrish.com/version.json` | Where the update check looks for new releases |
| `ANTHROPIC_API_KEY` or `CLAUDE_CODE_OAUTH_TOKEN` | unset | Claude for the editor agent, captions and replies |
| `GEMINI_API_KEY`, `ELEVENLABS_API_KEY` | unset | Generated images and video; voice, sound effects and music |
| `META_APP_ID`, `META_APP_SECRET`, `META_CONFIG_ID` | unset | Your own Meta app for Facebook Pages + Instagram |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | unset | YouTube publishing |
| `TIKTOK_CLIENT_KEY`, `TIKTOK_CLIENT_SECRET` | unset | TikTok publishing |

Provider keys (including the Meta app) can also be entered in the app under **Settings → Keys**, where they are encrypted at rest. AI model IDs and unit prices, the display currency (USD by default) and its exchange rate are under **Settings** too.

## How a job flows

1. **Company.** *Add from website or folder* builds a complete knowledge base of the company's existing system in two stages. First `hyperframes capture` (no tokens) grabs pages, assets, fonts, screenshots, CSS variables and computed styles. Then an agent sorts every asset into the brand library and writes:
   - `design.md`: the design system as a HyperFrames design spec (exact colors by role, type scale, fonts, radii, spacing, shadows, button/card/nav styles). It is copied into every project's `comp/`, where HyperFrames treats it as brand truth.
   - `knowledge.md`: what the business does, offers, proof, tone, site structure and video ideas.
   - `assets.json`: a caption for every file in the library (what it shows, which site section it came from, source URL, what it is good for). Editors can fix captions or move files between folders in the UI. If `GEMINI_API_KEY` is set, capture adds Gemini Vision descriptions.
   - `profile.json`, which fills the company form (only fields still empty).

   Brand library folders: `logo`, `people`, `images`, `graphics` (SVG icons and illustrations), `videos`, `fonts`, `audio`, `design` (screenshots, raw tokens), `other`. The UI filters by file type (SVG, PNG, JPG and so on). You can also add a company by hand. Companies are called tenants in the code.

   **From a project folder.** The same dialog takes the company's own folder (a website, app or docs project), with or without a website. The browser uploads a copy to `tenants/<id>/source/`, since Docker can't see the user's disk. Dependencies, build output, hidden files and anything that looks like a secret (`.env`, keys, databases) are left out twice: in the browser, and in `source.go`, because every agent may read the folder and so send it to Anthropic. The scan reads the README, manifests, marketing pages, locale strings, theme and CSS tokens and public assets, and copies real logos, images and fonts into the library. Every project agent also gets the folder as a read-only reference, so a video about a feature can use its real screens, UI text and flows. Choosing the folder again replaces the copy.
2. **Campaign (optional).** Groups a company's videos around one launch, offer or product. Its brief reaches every video's agent.
3. **Project.** *New project* is a short chat: company, campaign (or start a new one), what to make, which page or feature it is about (a URL, or a feature name the agent finds on their site), where it runs, length, brief, footage (optional) and quality. No footage is fine: the agent builds from the page, the knowledge base and the brand library.
4. **Agent.** The app runs `claude -p` in the project folder. The agent probes and transcribes the footage, asks up to 3 questions if it is blocked, cuts with ffmpeg, and builds a HyperFrames composition in `comp/`.
5. **Preview and edit.** The project page plays `comp/index.html` live in `<hyperframes-player>`, with no render needed. Chat with the agent to iterate, or click **Edit timeline** to open HyperFrames Studio and edit by hand: drag, trim and split clips, change text, timing and audio. Studio saves to the same files; on its next turn the agent reads your edits from `hyperframes history`, builds on them, and can ask Studio which element you selected ("make *this* bigger"). Studio is proxied on ports 3100+ (`STUDIO37_STUDIO_PORT`); Docker publishes 3100-3109.
6. **Export.** *Export final* runs `hyperframes render` into `out/`. Anything in `out/` is a finished video or image, and the **Library** page lists every project's finished files in one place, filterable by company and type.
   **Templates.** *Save as template* (on a project, or on a video in the Library) copies the project's `comp/` into `templates/<id>/`, with a thumbnail from its export (`template.go`). Dot folders such as HyperFrames history are left out. A template belongs to one company, or to every company. New project asks "Start from a template?" and puts a copy in the new project's `comp/`; the first prompt tells the agent to keep the design (layout, fonts, motion, captions, intro and outro) and swap in the new content. Videos queued from a templated source inherit it. The Library's *Templates* view lists and deletes them.
   **Several videos from one source.** *New project* asks "one video, or several?". A 2-hour class with ten solved questions can become ten videos. The agent transcribes the source, plans the videos (asking the editor to confirm unless the brief says go), and queues each with `37studio clip --title … --from … --to … --brief …` (`clip.go`). Each video becomes its own project with the source's company, campaign, format, length and quality. It shares the recording by hard link and gets a copy of the transcript, so nothing is copied or transcribed twice. `clipLoop` builds and exports the queued videos one at a time, after any autopilot video. The source project lists them on its *Videos* tab.
7. **Publish.** *Publish* posts a final video to any mix of the company's connected Facebook Pages, Instagram, TikTok and YouTube accounts, now or on a schedule, with a caption per platform (written by Claude on request, or by the agent when you ask in chat). See [Social publishing](#social-publishing).
8. **Autopilot (optional).** A content plan per company ("3 reels a week with one product tip each, 8pm, to our Page and Instagram") makes, renders, captions and schedules videos on its own; you approve each with one click, or let it post. See [Autopilot](#autopilot).

### Why HyperFrames, not Remotion

- **Agent-native.** A composition is plain HTML with `data-*` timing attributes, which Claude writes reliably. HyperFrames also ships Claude Code skills for compositions, captions, keyframes and media.
- **Live preview for free.** `@hyperframes/player` plays the agent's HTML directly. Remotion would need every agent edit bundled into a React project before it could preview.
- **Built-in tools for this work.** It includes `transcribe`, `remove-background` for cut-outs of people, `snapshot` for thumbnails, and `check` for linting.
- **License.** HyperFrames is open source. Remotion needs a paid company license once a team has more than 3 people.

## Transcription

The agent transcribes speech with `37studio transcribe <file> --language <code> --out work/transcript`. It runs faster-whisper ([asr/transcribe.py](asr/transcribe.py), embedded in the binary) on the user's computer, so it costs nothing and the audio stays local; paid APIs (ElevenLabs, Gemini) are used only when the editor asks for them.

- **Model by hardware.** With an NVIDIA GPU visible it uses `large-v3` (float16). On a CPU it uses `large-v3-turbo` (int8), which is several times faster for a small accuracy loss. `--quality best` forces `large-v3`; `--quality fast`, or less than 4 GB of memory, uses `small`, which is weak on less common languages.
- **Inside Docker every OS is the same Linux CPU path.** Docker Desktop on a Mac can't reach the Apple GPU, and the shipped compose file passes no NVIDIA GPU (the image also lacks the CUDA libraries), so the GPU branch only applies when the binary runs outside Docker. The model and thread count still follow the container's memory and cores. On a 12-core laptop it ran at about 1.3× the recording's length (a 59-second Bangla clip took 80 seconds), so a 2-hour class takes 2–3 hours. The tool prints progress and an estimate. It runs detached and the command waits at most 9 minutes, so the job outlives one Bash call and the agent's turn; running the same command again joins it or reuses the finished transcript.
- **Why not batched mode.** faster-whisper's batched pipeline was no faster on a CPU at beam 5, and at beam 1 it dropped sentences and slipped Devanagari letters into Bangla.
- **Long-class settings.** Voice activity detection skips silences, `condition_on_previous_text` is off so hours of audio don't loop on one phrase, and word timestamps give exact cut points for reels.
- **Outputs.** `transcript.json` (segments and words), `.srt`, and `.txt` with timestamps. A second run with the same file and settings reuses the result.
- **Subtitles instead.** If an `.srt` or `.vtt` sits next to the video, the command converts it at once instead of transcribing (segment timings, no word timings). It matches by file name, or uses a lone subtitle file beside a lone video. Editors add one from the Media tab or the New project footage step; YouTube imports bring their own.
- **Skip or stop.** The New project flow asks whether to transcribe. The project page shows progress with a **Cancel** button, and the Media tab can switch transcription off or back on. Both write `work/transcript/skip`, which stops a running job (it is killed as a process group) and makes `37studio transcribe` tell the agent to work without a transcript until it runs with `--force`.
- **Import from a link.** A YouTube or Google Drive link (in New project or the Media tab) downloads into `input/` with yt-dlp (`import.go`, in the same venv; `--js-runtimes node` for YouTube's player challenges). Video is capped at 1080p and merged to MP4, and YouTube's subtitles come along in the video's language. Progress shows on the project page and can be cancelled. Intermediate files stay in `work/download/`, which is removed afterwards. From New project, the agent starts once the download finishes. Google Drive files must be shared as "Anyone with the link".
- **Setup.** The image ships the Python environment (`/opt/asr`, `STUDIO37_ASR_PYTHON`). The model (about 1.6 GB for turbo) downloads into the cache volume the first time. Outside Docker the command makes its own venv under the user cache.

## Social publishing

Each company's **Social** tab connects accounts: you sign in once per platform with an account that manages the company's Pages or channels, then tick which of the returned accounts belong to that company. A company can have any number of Facebook Pages, Instagram, YouTube and TikTok accounts. On a project, **Publish** sends one final video to any mix of them at once, now or at a scheduled time; the **Posts** tab shows each upload's status, a link to the live post, and the platform's own error if one fails (with Retry). Tokens are stored in `37studio.db`.

Facebook **Groups** and personal **profiles** can't be posted to: Meta removed group posting for all apps in April 2024 and profile posting in 2018.

Scheduled posts go out only while 37Studio is running; a post more than 12 hours overdue is marked missed instead of going out late.

Register these redirect URIs (replace the host if you set `STUDIO37_PUBLIC_URL`):

| Platform | Redirect URI |
| --- | --- |
| Facebook + Instagram | `http://localhost:8080/api/oauth/meta/callback` |
| YouTube | `http://localhost:8080/api/oauth/google/callback` |
| TikTok | `http://localhost:8080/api/oauth/tiktok/callback` |

**Facebook Pages + Instagram: your own Meta app (recommended).** Company → Social → **Use your own Meta app** walks through this in the app, or:
1. At developers.facebook.com create a **Business** app and add **Facebook Login for Business**. Add the redirect URI above.
2. Create a Login configuration with a *User access token* and these permissions: `pages_show_list`, `pages_read_engagement`, `pages_manage_posts`, `business_management`, `instagram_basic`, `instagram_content_publish` (plus the Inbox permissions below). Put its ID in `META_CONFIG_ID` (Instagram's video upload requires Facebook Login for Business), and the app ID and secret in `META_APP_ID` and `META_APP_SECRET`, in Settings → Keys or `.env`. The ID only takes effect together with its secret, and agents never get the secret (`agentEnv`).
3. Leave the app in **Development** mode and give your team a role on it (Admin, Developer or Tester): no App Review is needed for people with a role. Each company gives your Business Portfolio access to its Page (or makes you a Page admin). Its Instagram must be a Business or Creator account linked to that Page.
4. Vertical videos of 3-90 seconds post as Reels; anything else posts as a regular Page video. Meta allows 30 API-published Reels per Page per 24 hours.

**The shared 37Studio Meta app.** Without your own app, logins use Shaitrish's 37Studio app (its ID and login configuration are built in, `social.go`). Copies don't carry its secret; the relay at `STUDIO37_AUTH_URL` finishes the login. Until Meta approves that app in App Review, only people with a role on it can sign in, so for now it serves installs the maintainers set up. The submission pack is [docs/meta-app-review.md](docs/meta-app-review.md).

**YouTube (one Google Cloud project).**
1. In console.cloud.google.com create a project and enable **YouTube Data API v3**.
2. OAuth consent screen: External, with the `youtube.upload` and `youtube.readonly` scopes. Set it to **In production**: in Testing, Google expires the login after 7 days. Unverified apps show a warning screen during sign-in; continue past it.
3. Credentials → OAuth client ID → *Web application*, with the redirect URI above. Put the ID and secret in `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET`.
4. Until the project passes YouTube's free API audit, YouTube keeps every upload **private**. Apply via the *YouTube API Services audit and quota extension* form. The default daily quota also allows only a handful of uploads.

**TikTok.**
1. At developers.tiktok.com create an app with the **Desktop** platform (the only kind TikTok lets redirect to `http://localhost`; 37Studio uses PKCE for it) and the redirect URI above. Running on a real HTTPS domain instead? Use the Web platform with an `https://` redirect and set `STUDIO37_PUBLIC_URL`.
2. Add **Login Kit** and **Content Posting API** (turn on Direct Post) with the scopes `user.info.basic`, `video.publish`, `video.upload`. Put the client key and secret in `TIKTOK_CLIENT_KEY` and `TIKTOK_CLIENT_SECRET`.
3. Until TikTok reviews the app, use its Sandbox and add each company's TikTok account as a target user. Until TikTok audits the app, direct posts must be **Only me**; or choose **Send to TikTok drafts** and the account owner taps Post in the app. Each company signs in with its own TikTok account, since TikTok has no agency access.

## Social inbox

**Inbox** in the sidebar shows Messenger, Instagram and WhatsApp messages plus Facebook and Instagram comments for every connected account, newest first, with a "needs reply" filter. Pick one, click **Draft with AI** (Claude writes a reply from that company's `knowledge.md`, in the customer's language, and says a team member will follow up when the answer isn't there), edit, and **Send**. Nothing goes out without a person clicking Send unless you turn on auto-replies. Drafts cost about $0.05 each and count toward the company's costs.

It reads the Graph API live while the page is open (refreshing every minute), so Pages and Instagram need no public address.

**Auto-replies** (Company → Social → Inbox replies): *Off*, *Claude drafts, you approve* (drafts wait in the Inbox marked "Draft ready", prefilled for one click), or *Claude replies on its own*. **How Claude should reply** on the same card holds the company's own rules (sign-off, language, what never to promise, when to hand over to a person), saved as `replies.md` next to `knowledge.md` and added to every draft; they override the default style but never the "facts only from the knowledge base" rule. A background loop checks every 2 minutes while 37Studio runs. Whatever arrived while the computer was off is answered when it's back, if the platform still allows it: messages within 24 hours, comments up to 2 days old. At most 20 replies per round, so a flood can't run up the Claude bill. For round-the-clock replies, keep it on a computer that doesn't sleep.

Meta setup, on top of Social publishing:
1. In the Meta app, add the **Engage with customers on Messenger** use case (`pages_messaging`). In *Manage everything on your Page* add `pages_manage_engagement`, `pages_manage_metadata` and `pages_read_user_content`. In the Instagram use case, under *API setup with Facebook login*, add the messaging permissions (`instagram_manage_messages`) and `instagram_manage_comments`.
2. Create a Login configuration with those six plus the six publishing permissions, put its ID in `META_CONFIG_ID`, and reconnect each company on its Social tab.
3. In the Instagram app: Settings → Messages and story replies → Message controls → Connected tools → **Allow access to messages**.
4. Until Meta grants Advanced Access through App Review and Business Verification, the inbox only shows messages from people with a role on the app or the Page. Test with a team account; real customers appear after approval.

Messenger and Instagram allow free-form replies within 24 hours of the customer's last message; comment replies have no time limit.

### WhatsApp

Company → Social → **Connect WhatsApp**. Meta delivers WhatsApp messages only by webhook, and a copy on someone's computer has no public address, so a small Cloudflare Worker (`worker/`) receives them (`/webhooks/meta`, signed with the app secret), holds them 30 days in D1, and each copy pulls its own numbers' messages (`/whatsapp/messages`, authorised by its WhatsApp token) into `wa_messages`, the lasting history. Replies go straight to the Graph API. Setup:
1. In the Meta app add the **Connect with customers through WhatsApp** use case, and point its WhatsApp webhook at your relay's `/webhooks/meta` with the `messages` field and the relay's `META_VERIFY_TOKEN`.
2. WhatsApp → API Setup gives a test number, its **Phone number ID** and **WhatsApp Business Account ID**. Add a real number there when ready; a number on the API can't be used in the WhatsApp Business phone app at the same time.
3. Business settings → Users → System users: add one, assign the WhatsApp account, **Generate token** for your app with `whatsapp_business_messaging` and `whatsapp_business_management`. It doesn't expire.
4. Paste the three into Connect WhatsApp. 37Studio subscribes the app to that account's messages.

Numbers owned by other businesses, and keeping a number on the WhatsApp Business phone app alongside the API (coexistence), need Meta's Embedded Signup, which needs the app to be a Meta **Tech Provider**: Business Verification, then App Review with two screen recordings (sending a message, creating a template), then Access Verification.

### Run your own relay

`worker/` is one Cloudflare Worker that does three jobs: it finishes Facebook logins for the shared Meta app, relays WhatsApp webhooks, and serves the product website in `worker/public/`. The public one at studio.shaitrish.com only serves the shared 37Studio Meta app. With your own Meta app you need your own relay only for WhatsApp; Cloudflare's free plan is enough.

```bash
cd worker
npx wrangler login
npx wrangler d1 create studio37          # put the database_id it prints into wrangler.toml
# edit wrangler.toml: your own `name`, META_APP_ID, and a route on your domain (or delete `routes` to use *.workers.dev)
npx wrangler secret put META_APP_SECRET  # your Meta app's secret
npx wrangler secret put META_VERIFY_TOKEN # any random string; enter the same one in Meta's webhook settings
npx wrangler d1 execute studio37 --remote --file schema.sql
npx wrangler deploy
```

Then set `STUDIO37_AUTH_URL=https://<your worker>` for each copy: in `.env` when running from source, or under `environment:` in the launcher bundle's `docker-compose.yml`.

## Autopilot

Each company can have content plans: what the videos are about, video type, length, format, language, quality, which days and time to post, which accounts, and whether to wait for approval. Three hours before each slot the scheduler starts a project; the agent picks a topic the company hasn't covered yet, builds and checks the video in one turn without asking questions, and writes the captions. The app renders it and queues the posts. With approval on (the default) the video waits on its project page and on Overview for one click; otherwise it posts at the slot.

It runs one video at a time and only while 37Studio is running. A slot that passed while the computer was off is skipped, not posted late. Times are in the browser's time zone, saved with the plan.

## Costs

```
cost = Claude agent cost      (total_cost_usd reported by the CLI for every turn, plus captions and inbox drafts)
     + Gemini generations     (per image / per second of video, unit prices in Settings)
     + ElevenLabs generations (per 1,000 characters, second or minute, unit prices in Settings)
```

Rendering and storage run on your own computer, so they cost nothing. Costs are recorded in USD, what the providers bill, and shown in the currency set in Settings (USD by default; set any other code and an exchange rate). Every project, campaign and company shows what it has cost; the **Costs** page breaks spend down by provider, company and month.

- **Quality tiers.** The tier picks the model and effort: Draft is Sonnet 5.5 at medium, Standard is Opus 5.5 at high, Premium is Opus 5.5 at xhigh. This is the main lever on cost.
- **Budget cap.** Each agent turn has a hard budget (`--max-budget-usd`, set in Settings).

## Project folder

```
data/projects/<id>/
  input/        raw uploads (the agent may not modify these)
  work/         agent scratch: audio, transcripts, cut ranges
  comp/         HyperFrames composition (live preview)
  out/          finished videos and images
  out/drafts/   draft renders and generated media
```

The agent's instructions (the harness) are in `agent.go`: `harness` holds the system prompt and `presets` the per-job-type briefs. Tune them there.

The agent can generate media by calling `37studio gen image|video --prompt ... --out ...`, which is the same binary. The cost is recorded against the project in `$STUDIO37_PROJECT`.

## Code map

| File | What |
| --- | --- |
| `main.go` | Routes, JSON helpers, basic auth |
| `db.go` | SQLite schema, row helpers, settings and their defaults |
| `handlers.go` | Companies, projects, uploads (streamed), file serving + player runtime injection |
| `agent.go` | Presets, quality tiers, harness prompt, Claude runs (stream-json to chat events), render, generate |
| `opencode.go` | OpenCode as a second harness, for cheaper models on routine edits |
| `studio.go` | Timeline editor: starts HyperFrames Studio per project, proxies it on its own port |
| `scan.go` | Company scan: website capture and/or project folder, then agent sorts assets and writes `design.md`, `knowledge.md`, profile |
| `source.go` | Company project folder: upload into `source/`, skip rules for dependencies and secrets |
| `transcribe.go`, `asr/` | `37studio transcribe`: local faster-whisper, detached long runs |
| `import.go` | YouTube and Google Drive imports with yt-dlp |
| `campaigns.go` | Campaigns: CRUD; the brief reaches each video's harness |
| `costs.go` | Project cost and the spend report |
| `keys.go` | Provider keys from Settings, encrypted at rest (AES-GCM, key file in the data folder) |
| `update.go` | Version and the update check |
| `autopilot.go` | Content plans, scheduler and the build → render → captions → approve pipeline |
| `template.go` | Templates: save a project's composition, list and delete them, and seed a new project's `comp/` |
| `clip.go` | Several videos from one source: `37studio clip`, the one-at-a-time build → export queue, and the Library listing |
| `gemini.go`, `eleven.go` | Gemini (Nano Banana images, Omni video, the `37studio gen` CLI) and ElevenLabs |
| `social.go` | Social publishing: OAuth for Meta, Google and TikTok, AI captions, post queue and per-platform uploads |
| `inbox.go` | Social inbox: Messenger, Instagram DMs and comments read live, Claude-drafted replies, sending, auto-replies |
| `whatsapp.go` | WhatsApp: connect a number, pull its messages from the relay, send replies |
| `web/src` | React UI: `pages/Project.tsx` is the editor workspace |
| `worker/` | Cloudflare Worker: product website, `version.json`, Meta login exchange, WhatsApp webhook relay (D1) |
| `dist/`, `release.sh` | The launcher bundle people download, and the script that publishes a release |

## Releasing

Maintainers bump `version` in `update.go`, then run `./release.sh "What changed"`. It builds and pushes `ghcr.io/shaitrish/37studio:<version>` and `:latest` for `linux/amd64` and `linux/arm64` (arm64 under QEMU, through a `studio37` buildx builder it creates; needs ~15 GB free disk), zips `dist/` into the download, writes `version.json`, and deploys the Worker. Set `IMAGE=` for another registry and log in with `docker login ghcr.io` first. Every copy reads `version.json` at most every 6 hours and shows "Version X available" in the sidebar; the launcher pulls the new image on its next start.

Chrome for Testing has no Linux arm64 build, so on arm64 HyperFrames renders with Playwright's pinned arm64 headless shell (`HYPERFRAMES_BROWSER_PATH`), as HyperFrames' own `Dockerfile.render` does.

## Roadmap

- **Object storage.** Footage stays on local disk. Add R2/S3 behind `projectDir` when it outgrows one disk.
- **Image posts.** Publishing sends videos only; Instagram needs a public URL for images, which a local-first app doesn't have.
- **YouTube and TikTok on the shared apps.** The code is there, but the shared Google and TikTok apps aren't registered or audited yet. Your own apps work today.
- **Meta App Review** for the shared app, so anyone can connect without their own Meta app ([docs/meta-app-review.md](docs/meta-app-review.md)).
- **WhatsApp on existing business numbers** (Embedded Signup, coexistence with the phone app), which needs Meta Tech Provider approval.

## Contributing

Bug reports, ideas and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for how to run it, the code style, and what makes a pull request easy to merge.

## License

[MIT](LICENSE) © Shaitrish. The open-source tools 37Studio bundles (HyperFrames, ffmpeg, faster-whisper and others) keep their own licenses. Claude Code is Anthropic's software, used under Anthropic's terms.
