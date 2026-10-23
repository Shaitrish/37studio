# 37Studio

Video and social media studio that runs on your own computer. Long-form footage and brand material go in; hook reels, captioned clips, motion-graphics explainers, promos and thumbnails come out, posted to Facebook, Instagram, TikTok and YouTube, on a schedule or fully on Autopilot. Each company (a "tenant" in the code) has a brand library, a knowledge base and its social accounts; every project shows what it cost. Shaitrish sells it as a one-time purchase to friends and family, who run it in Docker with their own keys.

**Claude Code does the editing. HyperFrames renders. Go serves. React is the UI.**

## How a job flows

1. **Company.** *Add from website or folder* builds a complete knowledge base of the company's existing system in two stages. First `hyperframes capture` (no tokens) grabs pages, assets, fonts, screenshots, CSS variables and computed styles. Then an agent sorts every asset into the brand library and writes:
   - `design.md`: the design system as a HyperFrames design spec (exact colors by role, type scale, fonts, radii, spacing, shadows, button/card/nav styles). It is copied into every project's `comp/`, where HyperFrames treats it as brand truth.
   - `knowledge.md`: what the business does, offers, proof, tone, site structure and video ideas.
   - `assets.json`: a caption for every file in the library (what it shows, which site section it came from, source URL, what it is good for). Editors can fix captions or move files between folders in the UI. If `GEMINI_API_KEY` is set, capture adds Gemini Vision descriptions.
   - `profile.json`, which fills the company form (only fields still empty).

   Brand library folders: `logo`, `people`, `images`, `graphics` (SVG icons and illustrations), `videos`, `fonts`, `audio`, `design` (screenshots, raw tokens), `other`. The UI filters by file type (SVG, PNG, JPG and so on). You can also add a company by hand.

   **From a project folder.** The same dialog takes the company's own folder (a website, app or docs project), with or without a website. The browser uploads a copy to `tenants/<id>/source/`, since Docker can't see the user's disk. Dependencies, build output, hidden files and anything that looks like a secret (`.env`, keys, databases) are left out twice: in the browser, and in `source.go`, because every agent may read the folder and so send it to Anthropic. The scan reads the README, manifests, marketing pages, locale strings, theme and CSS tokens and public assets, and copies real logos, images and fonts into the library. Every project agent also gets the folder as a read-only reference, so a video about a feature can use its real screens, UI text and flows. Choosing the folder again replaces the copy.
2. **Campaign (optional).** Groups a company's videos around one launch, offer or product. Its brief reaches every video's agent.
3. **Project.** *New project* is a short chat: company, campaign (or start a new one), what to make, which page or feature it is about (a URL, or a feature name the agent finds on their site), where it runs, length, brief, footage (optional) and quality. No footage is fine: the agent builds from the page, the knowledge base and the brand library.
4. **Agent.** The app runs `claude -p` in the project folder. The agent probes and transcribes the footage, asks up to 3 questions if it is blocked, cuts with ffmpeg, and builds a HyperFrames composition in `comp/`.
5. **Preview and edit.** The project page plays `comp/index.html` live in `<hyperframes-player>`, with no render needed. Chat with the agent to iterate, or click **Edit timeline** to open HyperFrames Studio and edit by hand: drag, trim and split clips, change text, timing and audio. Studio saves to the same files; on its next turn the agent reads your edits from `hyperframes history`, builds on them, and can ask Studio which element you selected ("make *this* bigger"). Studio is proxied on ports 3100+ (`STUDIO37_STUDIO_PORT`); Docker publishes 3100-3109.
6. **Export.** *Export final* runs `hyperframes render` into `out/`. Anything in `out/` is a finished video or image.
7. **Publish.** *Publish* posts a final video to any mix of the company's connected Facebook Pages, Instagram, TikTok and YouTube accounts, now or on a schedule, with a caption per platform (written by Claude on request, or by the agent when you ask in chat). See [Social publishing](#social-publishing).
8. **Autopilot (optional).** A content plan per company ("3 reels a week with one exam tip each, 8pm, to our Page and Instagram") makes, renders, captions and schedules videos on its own; you approve each with one click, or let it post. See [Autopilot](#autopilot).

## Why HyperFrames, not Remotion

- **Agent-native.** A composition is plain HTML with `data-*` timing attributes, which Claude writes reliably. HyperFrames also ships Claude Code skills for compositions, captions, keyframes and media.
- **Live preview for free.** `@hyperframes/player` plays the agent's HTML directly. Remotion would need every agent edit bundled into a React project before it could preview.
- **Built-in tools for this work.** It includes `transcribe` (local Whisper, word-level, with Bangla support via `--language bn`), `remove-background` for teacher cut-outs, `snapshot` for thumbnails, and `check` for linting.
- **License.** HyperFrames is open source. Remotion needs a paid company license once a team has more than 3 people.

## Transcription

The agent transcribes speech with `37studio transcribe <file> --language bn --out work/transcript`. It runs faster-whisper ([asr/transcribe.py](asr/transcribe.py), embedded in the binary) on the user's computer, so it costs nothing and the audio stays local; paid APIs (ElevenLabs, Gemini) are used only when the editor asks for them.

- **Model by hardware.** With an NVIDIA GPU visible it uses `large-v3` (float16). On a CPU it uses `large-v3-turbo` (int8), which is several times faster for a small accuracy loss and still handles Bangla. `--quality best` forces `large-v3`; `--quality fast`, or less than 4 GB of memory, uses `small`, which is weak on Bangla.
- **Inside Docker every OS is the same Linux CPU path.** Docker Desktop on a Mac can't reach the Apple GPU, and the shipped compose file passes no NVIDIA GPU (the image also lacks the CUDA libraries), so the GPU branch only applies when the binary runs outside Docker. The model and thread count still follow the container's memory and cores. On a 12-core laptop it ran at about 1.3× the recording's length (a 59-second Bangla clip took 80 seconds), so a 2-hour class takes 2–3 hours. The tool prints progress and an estimate. It runs detached and the command waits at most 9 minutes, so the job outlives one Bash call and the agent's turn; running the same command again joins it or reuses the finished transcript.
- **Why not batched mode.** faster-whisper's batched pipeline was no faster on a CPU at beam 5, and at beam 1 it dropped sentences and slipped Devanagari letters into Bangla.
- **Long-class settings.** Voice activity detection skips silences, `condition_on_previous_text` is off so hours of audio don't loop on one phrase, and word timestamps give exact cut points for reels.
- **Outputs.** `transcript.json` (segments and words), `.srt`, and `.txt` with timestamps. A second run with the same file and settings reuses the result.
- **Setup.** The image ships the Python environment (`/opt/asr`, `STUDIO37_ASR_PYTHON`). The model (about 1.6 GB for turbo) downloads into the cache volume the first time. Outside Docker the command makes its own venv under the user cache.

## Run it

### Docker (Linux, macOS, Windows)

```bash
cp .env.example .env      # set CLAUDE_CODE_OAUTH_TOKEN or ANTHROPIC_API_KEY, and optionally GEMINI_API_KEY
docker compose up --build
# open http://localhost:8080
```

- **Claude login.** Paste an Anthropic API key, or a Claude subscription token from `claude setup-token`, in Settings → Keys (or set `ANTHROPIC_API_KEY` / `CLAUDE_CODE_OAUTH_TOKEN` in `.env`).
- **Agent permissions.** Inside Docker, `STUDIO37_AGENT_PERMISSIONS=bypass` lets the agent run any command. The container is the sandbox. The default, `safe`, is an allowlist: ffmpeg, `npx hyperframes`, file edits, python and node.
- **Windows.** Use Docker Desktop with the WSL2 backend. Footage under `./data` is a bind mount. For multi-GB files, keep the repo inside the WSL filesystem (`\\wsl$\...`) rather than on `C:\`, because it is much faster.

### Local development

Prerequisites: Go 1.26+, Node 22+, ffmpeg, and the Claude Code CLI (logged in).

```bash
cd web && npm install && npm run build && cd ..
./dev.sh                  # both at once: backend + frontend, Ctrl+C stops both. Or separately:
set -a; . ./.env; set +a
air                       # API on :8080, rebuilds and restarts on every .go change (go install github.com/air-verse/air@latest)
cd web && npm run dev     # UI with hot reload on :5173, proxied to :8080
```

## Configuration

| Env var | Default | |
| --- | --- | --- |
| `STUDIO37_DATA` | `./data` | Everything lives here: `37studio.db`, `tenants/`, `projects/` |
| `STUDIO37_ADDR` | `:8080` | Listen address |
| `STUDIO37_WEB` | `web/dist` | Built UI |
| `STUDIO37_CLAUDE` | `claude` | Claude Code binary |
| `STUDIO37_AGENT_PERMISSIONS` | `safe` | `safe` or `bypass` |
| `STUDIO37_PASSWORD` | unset | Basic-auth password for the whole app |
| `GEMINI_API_KEY` | unset | Enables image and video generation |
| `HF_VERSION` | `0.8.186` | Pinned HyperFrames CLI and player runtime |
| `STUDIO37_STUDIO_PORT` | `3100` | First port for timeline editors (one per open project) |
| `STUDIO37_PUBLIC_URL` | `http://localhost:8080` | Where Facebook, Google and TikTok send the browser back after sign-in; must match the redirect URIs you register |
| `META_APP_ID`, `META_APP_SECRET`, `META_CONFIG_ID` | unset | Facebook Pages + Instagram publishing |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | unset | YouTube publishing |
| `TIKTOK_CLIENT_KEY`, `TIKTOK_CLIENT_SECRET` | unset | TikTok publishing |

Provider keys, AI model IDs and unit prices, currency, exchange rate and the license are edited in the app under **Settings**.

## Social publishing

Each client's **Social** tab connects accounts: you sign in once per platform with an account that manages the client's Pages or channels, then tick which of the returned accounts belong to that client. A client can have any number of Facebook Pages, Instagram, YouTube and TikTok accounts. On a project, **Publish** sends one final video to any mix of them at once, now or at a scheduled time; the **Posts** tab shows each upload's status, a link to the live post, and the platform's own error if one fails (with Retry). Tokens are stored in `37studio.db`; nothing goes through a third party.

Facebook **Groups** and personal **profiles** can't be posted to: Meta removed group posting for all apps in April 2024 and profile posting in 2018.

Scheduled posts go out only while 37Studio is running; a post more than 12 hours overdue is marked missed instead of going out late.

Register these redirect URIs (replace the host if you set `STUDIO37_PUBLIC_URL`):

| Platform | Redirect URI |
| --- | --- |
| Facebook + Instagram | `http://localhost:8080/api/oauth/meta/callback` |
| YouTube | `http://localhost:8080/api/oauth/google/callback` |
| TikTok | `http://localhost:8080/api/oauth/tiktok/callback` |

**Facebook Pages + Instagram (one Meta app).**
1. At developers.facebook.com create a **Business** app and add **Facebook Login for Business**. Add the redirect URI above.
2. Create a Login configuration with a *User access token* and these permissions: `pages_show_list`, `pages_read_engagement`, `pages_manage_posts`, `business_management`, `instagram_basic`, `instagram_content_publish`. Put its ID in `META_CONFIG_ID` (Instagram's video upload requires Facebook Login for Business), and the app ID and secret in `META_APP_ID` and `META_APP_SECRET`.
3. Leave the app in **Development** mode and give your team a role on it (Admin, Developer or Tester): no App Review is needed for people with a role. The client gives your Business Portfolio access to their Page (or makes you a Page admin). Their Instagram must be a Business or Creator account linked to that Page.
4. Vertical videos of 3-90 seconds post as Reels; anything else posts as a regular Page video. Meta allows 30 API-published Reels per Page per 24 hours.

**YouTube (one Google Cloud project).**
1. In console.cloud.google.com create a project and enable **YouTube Data API v3**.
2. OAuth consent screen: External, with the `youtube.upload` and `youtube.readonly` scopes. Set it to **In production**: in Testing, Google expires the login after 7 days. Unverified apps show a warning screen during sign-in; continue past it.
3. Credentials → OAuth client ID → *Web application*, with the redirect URI above. Put the ID and secret in `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET`.
4. Until the project passes YouTube's free API audit, YouTube keeps every upload **private**. Apply via the *YouTube API Services audit and quota extension* form. The default daily quota also allows only a handful of uploads.

**TikTok.**
1. At developers.tiktok.com create an app with the **Desktop** platform (the only kind TikTok lets redirect to `http://localhost`; 37Studio uses PKCE for it) and the redirect URI above. Running on a real HTTPS domain instead? Use the Web platform with an `https://` redirect and set `STUDIO37_PUBLIC_URL`.
2. Add **Login Kit** and **Content Posting API** (turn on Direct Post) with the scopes `user.info.basic`, `video.publish`, `video.upload`. Put the client key and secret in `TIKTOK_CLIENT_KEY` and `TIKTOK_CLIENT_SECRET`.
3. Until TikTok reviews the app, use its Sandbox and add each client's TikTok account as a target user. Until TikTok audits the app, direct posts must be **Only me**; or choose **Send to TikTok drafts** and the client taps Post in the app. Each client signs in with their own TikTok account, since TikTok has no agency access.

## Social inbox

**Inbox** in the sidebar shows Messenger, Instagram and WhatsApp messages plus Facebook and Instagram comments for every connected account, newest first, with a "needs reply" filter. Pick one, click **Draft with AI** (Claude writes a reply from that client's `knowledge.md`, in the customer's language, and says a team member will follow up when the answer isn't there), edit, and **Send**. Nothing goes out without a person clicking Send. Drafts cost about $0.05 each and count toward the client's costs.

It reads the Graph API live while the page is open (refreshing every minute), so Pages and Instagram need no public address.

**Auto-replies** (Company → Social → Inbox replies): *Off*, *Claude drafts, you approve* (drafts wait in the Inbox marked "Draft ready", prefilled for one click), or *Claude replies on its own*. **How Claude should reply** on the same card holds the company's own rules (sign-off, language, what never to promise, when to hand over to a person), saved as `replies.md` next to `knowledge.md` and added to every draft; they override the default style but never the "facts only from the knowledge base" rule. A background loop checks every 2 minutes while 37Studio runs. Whatever arrived while the computer was off is answered when it's back, if the platform still allows it: messages within 24 hours, comments up to 2 days old. At most 20 replies per round, so a flood can't run up the Claude bill. For round-the-clock replies, keep it on a computer that doesn't sleep (a Mac mini, or a laptop set to never sleep on power).

**WhatsApp** (Company → Social → Connect WhatsApp). Meta delivers WhatsApp messages only by webhook, so the Worker on studio.shaitrish.com receives them (`/webhooks/meta`, signed with the app secret), holds them 30 days in D1, and each copy pulls its own numbers' messages (`/whatsapp/messages`, authorised by its WhatsApp token) into `wa_messages`, the lasting history. Replies go straight to the Graph API. Setup:
1. In the Meta app add the **Connect with customers through WhatsApp** use case. The app's WhatsApp webhook already points at `https://studio.shaitrish.com/webhooks/meta` with the `messages` field (verify token: Worker secret `META_VERIFY_TOKEN`).
2. WhatsApp → API Setup gives a test number, its **Phone number ID** and **WhatsApp Business Account ID**. Add a real number there when ready; a number on the API can't be used in the WhatsApp Business phone app at the same time.
3. Business settings → Users → System users: add one, assign the WhatsApp account, **Generate token** for 37Studio with `whatsapp_business_messaging` and `whatsapp_business_management`. It doesn't expire.
4. Paste the three into Connect WhatsApp. 37Studio subscribes the app to that account's messages.

Numbers owned by other businesses (friends' and clients' own WhatsApp) need Meta's Embedded Signup, which needs the 37Studio app to be a Meta **Tech Provider**: Business Verification, then App Review with two screen recordings (sending a message, creating a template), then Access Verification. The same is required to keep a number on the WhatsApp Business phone app alongside the API (coexistence). Once approved, Connect WhatsApp becomes a sign-in button; the relay and inbox stay as they are.

Meta setup, on top of Social publishing:
1. In the Meta app, add the **Engage with customers on Messenger** use case (`pages_messaging`). In *Manage everything on your Page* add `pages_manage_engagement`, `pages_manage_metadata` and `pages_read_user_content`. In the Instagram use case, under *API setup with Facebook login*, add the messaging permissions (`instagram_manage_messages`) and `instagram_manage_comments`.
2. Create a Login configuration with those six plus the six publishing permissions, put its ID in `META_CONFIG_ID`, and reconnect each client on its Social tab.
3. In the Instagram app: Settings → Messages and story replies → Message controls → Connected tools → **Allow access to messages**.
4. Until Meta grants Advanced Access through App Review and Business Verification, the inbox only shows messages from people with a role on the app or the Page. Test with a team account; real customers appear after approval.

Messenger and Instagram allow free-form replies within 24 hours of the customer's last message; comment replies have no time limit.

## Autopilot

Each company can have content plans: what the videos are about, video type, length, format, language, quality, which days and time to post, which accounts, and whether to wait for approval. Three hours before each slot the scheduler starts a project; the agent picks a topic the company hasn't covered yet, builds and checks the video in one turn without asking questions, and writes the captions. The app renders it and queues the posts. With approval on (the default) the video waits on its project page and on Overview for one click; otherwise it posts at the slot.

It runs one video at a time and only while 37Studio is running. A slot that passed while the computer was off is skipped, not posted late. Times are in the browser's time zone, saved with the plan.

## Distribution (friends and family)

Each buyer runs their own copy in Docker on their own computer and pays for their own AI usage:

- **Website:** `worker/public/` on studio.shaitrish.com: `index.html` (product page; licenses ৳5,000 for 6 months or ৳8,000 for a year), `buy.html` (checkout and bank transfer), `setup.html` (step-by-step setup for Windows, Mac and Linux), `privacy.html` (what the app sends and where), `terms.html`, shared `site.css`, and screenshots in `shots/` taken from a demo copy (`STUDIO37_GRAPH_URL` / `STUDIO37_AUTH_URL` pointed at a stand-in server).
- **Download page:** `worker/public/index.html` on studio.shaitrish.com links `37studio.zip`, which is `dist/`: `docker-compose.yml`, `start.bat` (Windows), `start.command` (Mac), `start.sh` (Linux) and a README. The launcher starts Docker Desktop if it isn't running, pulls the latest image, starts it on `127.0.0.1:8080` only, and opens it in its own app window (Edge on Windows, Chrome on Mac when installed), so every start is also an update. The web app has a manifest, so Chrome/Edge "Install 37Studio" and Safari "Add to Dock" keep it in the taskbar or Dock.
- **Intel and Apple Silicon:** the image is built for `linux/amd64` and `linux/arm64`. Chrome for Testing has no Linux arm64 build, so on arm64 HyperFrames renders with Playwright's pinned arm64 headless shell (`HYPERFRAMES_BROWSER_PATH`), as HyperFrames' own `Dockerfile.render` does.
- **Keys:** Settings → Keys takes an Anthropic API key or a Claude subscription token (`claude setup-token`), plus optional Gemini and ElevenLabs keys. They are encrypted in the database with a key file next to it and applied to the agent's environment; nothing needs `.env`. Anthropic's terms don't let third-party products run on Claude subscription logins without its approval, so the supported setup for sold copies is each buyer's own API key.
- **License:** Ed25519-signed keys, ৳5,000 for 6 months or ৳8,000 for a year. Issue one with `37studio license --name "Buyer" --email buyer@example.com --months 6` (or `--months 12`; no `--months` = never ends, for your own copies) on the machine that holds `~/.config/37studio/license-private.key`. Back that key up; without it no new licenses can be issued. Without a valid key a copy makes two trial videos; new projects, Autopilot, publishing and auto-replies answer 402 and the app opens an Activate dialog. The Worker checks the same key (signature, end date, `REVOKED_LICENSES`) before finishing a Facebook login or serving WhatsApp messages, so patching the app doesn't unlock those.
- **Buying:** `/buy` on studio.shaitrish.com. In Bangladesh the form posts to `/pay/start` (worker/pay.js), which opens an SSLCommerz checkout (bKash, Nagad, Rocket, cards); the return and IPN are validated with SSLCommerz's validation API (order, amount, currency) and the Worker signs the key itself with the `LICENSE_PRIVATE_KEY` secret, shown on `/pay/order?t=<tran_id>`. `SSLCZ_MODE` in wrangler.toml must be `live` for real money; in sandbox only our own test purchases (`test=1`) get through, and their keys carry `"test": true`, which the app and the Worker refuse. Orders: `npx wrangler d1 execute studio37 --remote --command "select tran_id,plan,amount,name,email,phone,status,datetime(created_at/1000,'unixepoch') from orders order by created_at desc limit 20"`. From abroad buyers pay by bank transfer ($40 / $65) and email the receipt; issue their key with `37studio license --months 6|12`.
- **Updates:** each copy reads `https://studio.shaitrish.com/version.json` at most every 6 hours and shows "Version X available" in the sidebar.
- **Facebook login:** every copy uses Shaitrish's 37Studio Meta app (ID and configuration are built in). Copies don't carry the app secret; the Worker finishes the login. Until Meta approves the app, add each buyer as a Tester (App roles → Roles) before they connect. Or the buyer uses their own Meta app: Company → Social → **Use your own Meta app** saves its ID, secret and login configuration as keys (`META_APP_ID`, `META_APP_SECRET`, `META_CONFIG_ID`, also under Settings → Keys). The app ID only takes effect together with its secret. Logins then run locally, and the buyer, as the app's admin, can post and read comments on their own Pages in Development mode. Agents never get the secret (`agentEnv`).

Publishing a release (bump `version` in `edition.go` first): `./release.sh "What changed"` builds and pushes `ghcr.io/shaitrish/37studio:<version>` and `:latest` for both architectures (arm64 under QEMU, through a `studio37` buildx builder it creates; needs ~15 GB free disk), zips `dist/`, writes `version.json`, and deploys the Worker. Set `IMAGE=` for another registry; log in with `docker login ghcr.io` first, and after the first push make the package public on GitHub (Package settings → Change visibility) so buyers can pull it without logging in.

The Worker, once: `cd worker && npx wrangler login && npx wrangler secret put META_APP_SECRET && npx wrangler secret put META_VERIFY_TOKEN && npx wrangler deploy`, and `npx wrangler d1 execute studio37 --remote --file schema.sql` for the WhatsApp table. It serves studio.shaitrish.com as a custom domain on the shaitrish.com zone.

## Costs

```
cost = Claude agent cost      (total_cost_usd reported by the CLI for every turn, plus captions and inbox drafts)
     + Gemini generations     (per image / per second of video, unit prices in Settings)
     + ElevenLabs generations (per 1,000 characters, second or minute, unit prices in Settings)
```

Rendering and storage run on the user's own computer, so they cost nothing. Every project, campaign and company shows what it has cost; the **Costs** page breaks spend down by provider, company and month. There is no invoicing in the app.

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
| `db.go` | SQLite schema, row helpers, settings |
| `handlers.go` | Clients, projects, uploads (streamed), file serving + player runtime injection |
| `agent.go` | Presets, quality tiers, harness prompt, Claude runs (stream-json to chat events), render, generate |
| `studio.go` | Timeline editor: starts HyperFrames Studio per project, proxies it on its own port |
| `scan.go` | Company scan: website capture and/or project folder, then agent sorts assets and writes `design.md`, `knowledge.md`, profile |
| `source.go` | Company project folder: upload into `source/`, skip rules for dependencies and secrets |
| `transcribe.go`, `asr/` | `37studio transcribe`: local faster-whisper, detached long runs |
| `campaigns.go` | Campaigns: CRUD; the brief reaches each video's harness |
| `costs.go` | Project cost and the spend report |
| `keys.go` | Provider keys from Settings, encrypted at rest (AES-GCM, key file in the data folder) |
| `edition.go` | Version, update check, license keys (`37studio license --name ...` issues them) |
| `autopilot.go` | Content plans, scheduler and the build → render → captions → approve pipeline |
| `dist/`, `release.sh` | The launcher bundle friends download, and the script that publishes a release |
| `worker/` | Cloudflare Worker on studio.shaitrish.com: download page, `version.json`, Meta login exchange, WhatsApp webhook relay (D1) |
| `gemini.go` | Gemini Interactions API (Nano Banana images, Omni video) and the `37studio gen` CLI |
| `social.go` | Social publishing: OAuth for Meta, Google and TikTok, AI captions, post queue and per-platform uploads |
| `inbox.go` | Social inbox: Messenger, Instagram DMs and comments read live, Claude-drafted replies, sending, auto-replies |
| `whatsapp.go` | WhatsApp: connect a number, pull its messages from the Worker relay, send replies |
| `web/src` | React UI: `pages/Project.tsx` is the editor workspace |

## Not built yet

- **Object storage.** Footage stays on local disk. Add R2/S3 behind `projectDir` when it outgrows one disk.
- **Image posts.** Publishing sends videos only; Instagram needs a public URL for images, which a local-first app doesn't have.
- **YouTube and TikTok for buyers.** The code is there, but the Google and TikTok apps aren't registered or audited yet, so the shared copies show them as not set up.
- **Meta for everyone.** Until App Review passes, buyers must be added as app testers. The submission pack is `docs/meta-app-review.md`; Business Verification comes first, and the Facebook redirect must go through the Worker before the app goes Live.
- **WhatsApp on customers' own numbers** (Embedded Signup, coexistence with the phone app) needs Meta Tech Provider approval.
- **Sale notifications.** Paid orders only land in D1; nothing emails you yet.
