# 37Studio guide

How to set up 37Studio and use every part of it. This same guide is inside the app: click **Guide** in the sidebar, or **Help** at the top of any page to read the part about that page beside it.

## Start here

37Studio makes videos for a business and runs its social accounts, from one app on your own computer. You bring your own AI keys and pay those services directly. 37Studio itself is free and open source, with no accounts, no subscription and no telemetry.

The whole journey, in order:

1. [Install 37Studio](#install-37studio).
2. [Connect Claude](#connect-ai-services). It's the only service you must have.
3. [Add a brand](#add-a-brand) from its website or folder.
4. [Make a video](#make-a-video) and [refine it](#work-on-a-project) by chatting with the editor agent.
5. [Connect social accounts](#connect-social-accounts), then [publish](#publish).
6. Optional: let [Autopilot](#autopilot) make videos on a schedule, and let the [Inbox](#inbox) draft your replies.
7. Watch [Performance](#performance) to see which videos worked, so the next ones do too.

### Find help in the app

- **Help**, at the top of every page and in most windows, opens the part of this guide about what's on screen. It opens beside the page, so you don't lose your place. In a project or brand it follows the tab you're on.
- Press **?** to do the same from the keyboard, and **Esc** to close it.
- **Learn 37Studio** on the Overview page has the first steps.
- **Guide** in the sidebar is the whole guide, with search.

### Words used in the app

| Word | What it means |
| --- | --- |
| Brand | A business you make videos for. It holds the logos, colours, facts, voices and social accounts. |
| Project | One editing job. It holds the footage, the chat with the agent and the finished files. |
| Editor agent | The AI that builds and changes your video. It is Claude Code unless you pick another model. |
| Assistant | A chat that runs the whole app for you, in the sidebar. |
| Campaign | A group of videos for one launch or offer, sharing one brief. |
| Autopilot | A content plan that makes and schedules videos by itself. |
| Export | Rendering the final video file. |
| Publish | Sending a finished video to social accounts. |

## Install 37Studio

Pick one way. All of them are the same app.

| Way | Good for | Runs on |
| --- | --- | --- |
| Ready-made app | Most people | Windows, Mac, Linux, with Docker |
| Desktop app (preview) | A normal app window with no Docker | Linux and Windows |
| Cloud server | Teams, and running while your laptop sleeps | Any Linux server |
| From source | Developers | Anywhere Docker runs |

### Ready-made app

You need Docker Desktop (or Docker Engine on Linux), 16 GB of RAM and about 20 GB of free disk.

1. Install [Docker Desktop](https://www.docker.com/products/docker-desktop/) and open it once.
2. Download [37studio.zip](https://studio.shaitrish.com/37studio.zip) and unzip it into a folder you'll keep, like Documents/37Studio.
3. Double-click `start.bat` on Windows or `start.command` on a Mac (the first time, right-click → Open). On Linux run `./start.sh`.

The first start downloads about 3 GB, then opens http://localhost:8080. Only your own computer can reach that address. Your work is in the `data` folder next to the launcher.

The website has a [step-by-step setup guide](https://studio.shaitrish.com/setup) for each system.

### Desktop app

Download the file for your computer from [the desktop page](https://studio.shaitrish.com/desktop). On Ubuntu 24.04+ or Debian 13+:

```bash
sudo apt install ./37studio_*_amd64.deb
sudo npm install -g @anthropic-ai/claude-code
```

On Windows 10 or 11, open the installer (if Windows says it protected your PC, click **More info** → **Run anyway**), then install the tools the agent uses:

```powershell
winget install Git.Git Gyan.FFmpeg OpenJS.NodeJS.LTS Python.Python.3.13
npm install -g @anthropic-ai/claude-code
```

Open **37Studio** from your app menu or Start menu. **Finish setup** on the Overview page lists anything still missing, with the command to install it and a **Check again** button.

Things that work differently from the Docker version:

- The agent uses a short list of allowed commands, because no container separates it from your computer. Some tasks may stop and say a command isn't allowed.
- Links, the timeline editor and social sign-ins open in your normal browser.
- Closing the window quits the app, so Autopilot, scheduled posts and auto-replies stop.
- Your work is in `~/.local/share/37studio` on Linux and `%AppData%\37Studio` on Windows, not in the Docker `data` folder.
- If the Docker version is running, it holds port 8080, the desktop app picks another port, and social sign-ins won't work until you stop Docker.

The [desktop guide](https://github.com/Shaitrish/37studio/blob/main/docs/desktop.md) covers other Linux, building it yourself and opening your Docker data.

### Cloud server

Use a Linux server with 16 GB of RAM, 4 or more vCPUs and 50 GB of disk. You need a domain pointing at it, and ports 80, 443 and 3100–3109 open. The app gets its own HTTPS certificate and won't start without a password.

The [server guide](https://github.com/Shaitrish/37studio/blob/main/docs/docker.md#run-it-on-a-server) has the exact commands and explains what anyone with the password can do.

### From source

```bash
git clone https://github.com/Shaitrish/37studio.git && cd 37studio
cp .env.example .env
docker compose up --build
```

Then open http://localhost:8080. To work on the code with hot reload, see [CONTRIBUTING](https://github.com/Shaitrish/37studio/blob/main/CONTRIBUTING.md).

## Connect AI services

Open **Settings → AI services**. Keys are encrypted on your computer and sent only to the service they belong to. **Settings → Get started** shows what's done and what's left.

### Claude (required)

Claude edits your videos, writes captions and drafts replies. One of these is enough:

- **Anthropic API key.** You pay for what you use. Create one at [console.anthropic.com](https://console.anthropic.com/settings/keys). This is the fully supported route.
- **Claude Pro or Max plan.** Make a token, then paste it under **Claude Pro or Max plan**. Anthropic's terms govern using a subscription in other apps.
- **Desktop app only:** if Claude Code is already signed in on this computer, no key is needed.

To make the plan token, run this in the 37Studio folder:

```bash
docker compose exec studio claude setup-token
```

From source the service is called `37studio`, and in the desktop app the command is just `claude setup-token`.

### Optional services

| Service | What it adds |
| --- | --- |
| Gemini | Generated images and video clips |
| fal.ai | More video models: Seedance, Kling, Veo, Wan and others |
| ElevenLabs | Voiceover, sound effects, music and cloned voices |
| Codex | Sign in with a ChatGPT plan, no API key. Runs count against your plan |
| OpenCode, OpenAI, xAI, OpenRouter | More models for the agent. Each adds its newest models to the model menus |

## Add a brand

Everything you make belongs to a brand, so add one first.

1. Open **Brands** and click **Add from website or folder**.
2. Paste the website, choose the brand's project folder, or both.
3. Under **Anything we already know**, add what the scan can't guess, like "focus on the paid product, not the blog".
4. Click **Scan**. It takes 5 to 10 minutes, and the page updates by itself.
5. Read the **Knowledge** tab and fix anything wrong. Every video and reply uses it.

The folder is copied into 37Studio, without dependencies, build output, hidden files or anything that looks like a secret. Prefer to type it all yourself? Use **Add manually**.

**Re-scan** later to refresh a brand. It only fills fields that are still empty, so your edits are kept.

### Connect a live folder

For a project too big to copy, or one that keeps changing, connect it instead. Click **Edit** on the brand and fill in:

- **Connected folder:** the folder's full path. Nothing is copied. The agent reads the real files, so it sees new work at once, and it can't change them.
- **Databases the agent may query:** SQLite files inside that folder, by their path in it. The agent can only run read-only queries, and only on the files you list, so leave out anything with customer data.

The agent can't read `.env` files, keys or raw database files in that folder. Tell it where things are in the **Knowledge** tab. In the Docker version, the folder must be mounted into the container.

### Brand tabs

| Tab | What it's for |
| --- | --- |
| Brand library | Logos, people, images, icons, videos, fonts, audio and design references. Click a file to describe it, so the agent picks it well. The pin button puts a file first, and the first logo is the brand's icon |
| Campaigns | This brand's campaigns |
| Projects | This brand's projects |
| Voices | Voices for voiceovers. See [Generated media and voices](#generated-media-and-voices) |
| Social | Connected accounts and how inbox replies work |
| Autopilot | Content plans |
| Design system | Colours, type, corner radius and `design.md`, the brand truth for every video |
| Knowledge | `knowledge.md`, the facts the agent and replies are allowed to use |
| Profile | Details, editor notes, and **Delete brand** |

**Edit** at the top changes the brand's name, presenter, contact details, spoken language, font, colours and notes for the editor. Spoken language drives transcription, so set it correctly.

Deleting a brand also deletes its projects, videos, campaigns, templates, Autopilot plans and connected accounts. It can't be undone.

## Make a video

Click **New video** on the Overview, Projects, brand or campaign page. A few questions come one at a time, and you can click any answer to change it.

1. **Brand.** Skipped if you have only one.
2. **Template.** Only asked if you've saved one.
3. **What are we making?** See the table below.
4. **Shape, length, number of videos and language.** The defaults are good, so most people just press **Continue**.
5. **What it's about.** Write the topic, the hook, the call to action and anything to avoid, or let the agent decide from the knowledge base. You can name one page or feature to focus on.
6. **Footage.** Drop files, paste a YouTube or Google Drive link, or use only brand assets.
7. **Name, campaign and quality.** Then click **Create and start**.

| Type | Shape | What you get |
| --- | --- | --- |
| Hook reel | 9:16 | A long video turned into a reel, opening on its most gripping moment |
| Captioned clip | 9:16 | A clean cut with accurate, styled subtitles |
| Motion-graphics explainer | 16:9 | The speaker on screen with graphics that show the idea |
| Promo ad | 9:16 | A short ad: hook, problem, payoff, offer, call to action |
| Thumbnails | 16:9 | YouTube, course, product and book thumbnails as PNG |
| Custom | 16:9 | Anything else. The brief decides |

Shapes: **9:16** for Reels, TikTok and Shorts, **16:9** for YouTube and websites, **4:5** for Instagram and Facebook feeds, **1:1** for square feed posts.

Languages in the app: Bangla, English, Hindi, Urdu, Arabic and Spanish.

### Quality

| Tier | Model | Use it for |
| --- | --- | --- |
| Quick | Haiku 5.5 | Small fixes like text, colours and timing |
| Draft | Sonnet 5.5 | Simple cuts and captions |
| Standard | Opus 5.5 | The default. Reels and motion graphics |
| Premium | Opus 5.5, extra effort | Flagship ads |

Other models you've connected appear in the same menus.

## Work on a project

The project page has a preview, a row of versions, some tabs and the agent's chat.

### Chat with the editor agent

Ask for changes in plain words, like "make the hook shorter" or "use the second logo". You can:

- Attach screenshots by button, paste or drop, to show what you mean.
- Change the model for one message, for example a cheap one for a small fix.
- Dictate instead of typing: click the microphone in the corner of the box, and the switch beside it for Bangla or English. In the desktop app it records you and turns it into text on this computer, free (needs Python 3 and ffmpeg). The first time it downloads the speech model; English takes a few seconds, Bangla about 15.
- Type **/** or click the **/** button for quick actions such as "Looks good, render the final".
- Pick a version under the preview and it goes with your message, so the agent knows which one "this" means. Click × on it to leave it out.
- Keep typing while the agent works. Your message is marked as sent while working.
- Press **Up** in the message box to bring back what you sent before, and **Down** to come back to what you were typing. **Enter** sends, **Shift+Enter** starts a new line.
- Press **Esc** to stop the agent.
- Hover over a message and click the copy button to copy it.
- Drag the gap between the preview and the chat to resize the chat. Double-click it to reset.
- Click the arrows beside **Editor agent** to widen the chat to the whole page, and again to bring the preview back.

When jobs don't depend on each other, the agent hands them to subagents that work at the same time, such as finding hook moments while another picks brand assets. Each shows as a row in the chat with a spinner while it works, its latest step, and a tick when it's done. Click a row to see everything it did.

### Preview, drafts and export

- **Live preview** plays the current edit instantly, with no rendering. When the agent makes a new version, click **New version ready, refresh preview**.
- **Draft render** makes a quick low-quality file for sharing or checking timing.
- **Export** renders the final video. It appears in the versions row and in the Library.
- To find an exported video on your computer, point at it in the versions row and click the folder button. It opens the folder with the video selected. This works in the desktop app and when 37Studio runs on this computer without Docker; elsewhere, use the download button.
- **Mark delivered** tells the Overview page this one is done.

### Edit by hand

**Edit timeline** opens a real timeline editor in a browser tab, where you can drag, trim and split clips and change text and audio. The agent picks up your hand edits on its next turn. Close it with the **X** beside the button.

### Project tabs

| Tab | What's there |
| --- | --- |
| Videos | The videos cut from this source. Only shows when there are some |
| Brief | The brief, focus page and quality, plus **Delete project** |
| Media | Your sources and everything the video uses. Add footage or subtitles here |
| Generate | Make images, clips, voiceover, sound effects and music by hand |
| Posts | Every post of this video and where it stands |
| Cost | What this project has cost so far |

## Long recordings

One long recording can become many videos. A 2-hour class can become ten reels.

1. Start a **New video** and set **Videos** to 3, 5, 10 or **As many as it has**.
2. Upload the recording, or paste a YouTube or Google Drive link.
3. The agent plans the videos from the transcript and queues one project for each.
4. The app makes and exports them one at a time. Follow them on the source project's **Videos** tab. Finished ones also appear in the Library.

Transcription runs on your own computer, so it is free and private, and takes about as long as the recording.

To start sooner, either untick **Transcribe what's said** or add subtitles. A `.srt` file or a YouTube link's own subtitles are used instantly.

## Generated media and voices

The agent only generates media when your footage and brand library can't do the job, because each generation is billed. It says what it made, and the cost shows on the project.

To generate by hand, use the project's **Generate** tab: **Image**, **Video clip**, **Voiceover**, **Sound effect** or **Music**. Results land in the project's drafts.

The agent asks before media on one project goes past the limit in **Settings → Spending**.

### Brand voices

On a brand's **Voices** tab (needs an ElevenLabs key):

- **Clone a voice** from 1–3 minutes of one person speaking clearly with no music. Audio or video files both work. Cloning needs a paid ElevenLabs plan, and you need the speaker's permission.
- **Add an ElevenLabs voice** from your own ElevenLabs account.
- Click the star to make a voice the default for its language.

The agent uses the default for the video's language. Name another in chat, like "use Rahim's voice", and it can even re-speak a finished video's voiceover with the same words and timing. Then press **Export** again.

Brands with no voices use the voices in **Settings → AI services**.

## Library and templates

**Library** in the sidebar has every finished video and image from every project. Drafts stay in their projects. Filter by brand or type, search titles, or use **Select** to delete several at once. Deleting here removes the file, not the project.

### Templates

A template keeps a video's look: layout, fonts, colours, motion, caption style, intro and outro. The agent swaps in the new content.

1. Click **Save as template** on a project, or on a video in the Library.
2. Write what new videos should keep. Tick **Use it for every brand** to share it.
3. New videos now ask **Start from a template?**

Find your templates under the Library's **Templates** tab.

## Campaigns

A campaign groups videos around one launch, offer or product, so they share a message. Its brief goes to the agent on every video in it: what it promotes, the audience, the call to action, dates and tone.

Click **New campaign** on the Campaigns page, or choose one in the last step of **New video**. The campaign page shows its videos and what they cost. Deleting a campaign keeps its videos as one-off projects.

## Connect social accounts

Connecting takes two steps: make a free developer app once in **Settings → Social apps**, then connect accounts on each brand's **Social** tab. 37Studio has no built-in app, so yours stays yours.

| Platform | Publishes | Inbox | You connect with |
| --- | --- | --- | --- |
| Facebook Pages | Reels and Page videos | Messenger, comments | Your own Meta app |
| Instagram (Business or Creator) | Reels | Messages, comments | The same sign-in as Facebook |
| YouTube | Videos | No | Your own Google OAuth client |
| TikTok | Videos, or sent to the creator's TikTok app | No | Your own TikTok app |
| LinkedIn | Videos to your profile | No | Your own LinkedIn app |
| X | Video posts | No | Your own X app |
| Pinterest | Video Pins on boards you pick | No | Your own Pinterest app |
| Bluesky | Video posts | No | Handle and an app password |
| Mastodon | Video posts | No | Server URL and an access token |
| WhatsApp | No | Messages | A Business API number and a tunnel or relay |

Then, on the brand's **Social** tab:

1. Click **Connect** for the platform and sign in. Facebook and Instagram start with **Set up your Meta app**, about 15 minutes.
2. Tick which of the returned accounts belong to this brand.

Follow the [step-by-step platform guide](https://studio.shaitrish.com/connect). It has a plain-text copy you can paste into an AI chat to be walked through it.

When you register a redirect address with a platform, use `http://localhost:8080/api/oauth/<platform>/callback`. It must match exactly. X refuses `localhost`, so use `http://127.0.0.1:8080/api/oauth/x/callback` there.

Facebook, Instagram, YouTube and TikTok also report how each video did. See [Performance](#performance).

Unaudited apps are restricted by the platforms: YouTube uploads stay private and TikTok posts stay **Only me** or go to the creator's TikTok phone app (a notification in its Inbox) until those platforms approve your app. LinkedIn sign-ins last 60 days, then connect again.

## Publish

Export a final video first.

1. Click **Publish** on the project.
2. Pick the video and the accounts.
3. Click **Write with AI** for a caption for each platform, or write your own. Every caption can be edited.
4. Choose **Post now** or **Schedule**.

Follow it on the project's **Posts** tab, which has each upload's status and a link to the live post. A failed post has a **Retry** button.

Scheduled posts go out only while 37Studio is running. A post more than 12 hours late shows as **Failed** with the note "Missed" instead of going out late, and **Retry** posts it now.

## Post from your computer

Post a video or photo you already have, made in another app or on your phone, without the agent.

1. Click **New post** on the Overview, Projects or a brand's page.
2. Pick the brand and drop the video or photo in. Add a line about what it is, if you like: it helps the captions.
3. Click **Next**. The file uploads and **Publish** opens.
4. Pick the accounts, click **Write with AI** (it looks at the photo, or at frames from the video), then post now or schedule it.

The post shows in **Projects** like a video, with its **Posts** tab and numbers.

Photos post to Facebook Pages, X, LinkedIn, Pinterest, Bluesky and Mastodon. Instagram and TikTok only take photos from a public web address, which a copy on your computer doesn't have, and YouTube takes no photos, so those accounts can't be picked for a photo. Videos go to every platform.

## Make other sizes

One video can go out in the shape each platform shows best: 9:16 for Reels, TikTok and YouTube Shorts, 16:9 for Facebook and YouTube, 4:5 or 1:1 for feeds.

1. Under the preview, hover over a final video and click the arrows icon (**Make another size**).
2. Pick the shape, then how to fill it:
- **Fit** keeps the whole picture over a blurred copy of itself, so nothing is cut off.
- **Crop** fills the frame from the middle and cuts the edges.
- **Ask the agent** lays the video out again for the new shape, so titles and the logo sit right. It looks best and costs an agent turn.
3. The copy shows up in Versions as a new final, named like promo-9x16.mp4. Fit and Crop are free and take about as long as the video.

In **Publish**, when a project has more than one shape, **Size for each platform** sends each platform its best one: vertical to TikTok, Instagram and Pinterest, wide to Facebook, YouTube, LinkedIn, X, Bluesky and Mastodon. Change any of them there; for YouTube Shorts, pick the 9:16 copy.

## Autopilot

Autopilot makes a fresh video before each time slot. The agent picks a topic from your brief and knowledge base, builds and checks the video, renders it and writes the captions.

1. Open a brand's **Autopilot** tab and click **New plan**.
2. Describe what the videos are about, for example: "3 reels a week with one exam tip each, in Bangla".
3. Pick the days, the time, the format, the length, the language, the quality and the accounts.
4. Under **Before posting**, choose **Wait for my approval** or **Post automatically**.

Each video starts three hours before its slot. Videos waiting for you appear on the Overview page and on the plan: watch one, check its captions, then click **Approve** or **Skip**.

**Run now** makes one video straight away. Set a plan to **Paused** to stop it. Deleting a plan keeps the videos it made.

Autopilot only runs while 37Studio is running. Keep the computer awake, or run it on a server.

## Inbox

**Inbox** in the sidebar shows Messenger, Instagram and WhatsApp messages plus Facebook and Instagram comments for every connected account.

1. Pick a conversation. **Needs reply** hides what's already answered.
2. Click **Draft with AI**. Claude writes a reply from the brand's knowledge base, in the customer's language.
3. Edit it and click **Send**, or press **Ctrl+Enter**.

Comment replies are public. Messages can only be answered within 24 hours of the customer's last message; for older ones the reply box says so: answer them from the Page's inbox in Meta Business Suite (7 days), or from 37Studio once Meta approves your app's Human Agent permission.

### Auto-replies

Set this on the brand's **Social** tab, under **Inbox replies**:

- **Off.** Nothing happens unless you draft a reply yourself.
- **Claude drafts, you approve.** Drafts wait for you, marked **Draft ready**.
- **Claude replies on its own.** Replies go out by themselves.

**How Claude should reply** holds the brand's own rules: tone, sign-off, what never to promise, when to hand over to a person. Facts only come from the knowledge base, and when the answer isn't there, Claude says a team member will reply.

The app checks every 2 minutes while it's running. It catches up on what arrived while it was off if the platform still allows a reply (24 hours for messages, 2 days for comments).

The inbox needs extra Meta permissions on top of publishing, and until Meta grants your app Advanced Access, only messages from people with a role on the app or Page appear. WhatsApp also needs a public address for Meta's webhook. See the [inbox setup](https://github.com/Shaitrish/37studio/blob/main/docs/platforms.md#inbox-messenger-instagram-and-comments).

## Performance

**Performance** in the sidebar shows how each published video did on Facebook, Instagram, YouTube and TikTok, including videos posted outside 37Studio.

- **Views, likes, comments, shares and saves**, as each platform counts them.
- **Watched:** the average share of the video people watched. Facebook and YouTube report it. Instagram reports how many people left in the first 3 seconds instead; hover the number to see it.
- **Avg watch:** how long each view lasted.
- **Engagement:** likes, comments, shares and saves per 100 views.

Click a column to sort by it, and click again for the lowest first. **Most viewed** and **Least viewed** sit above the list; the least viewed only counts videos more than 3 days old. A video made in 37Studio links back to its project.

**By platform** puts the platforms side by side: views per video, how much was watched and engagement, with a star on the best of each.

Every video you post from 37Studio is remembered on every platform it went to:

- **Library → Published** lists them, with each platform's views under the video. Open one to compare all its numbers side by side, with the best platform marked.
- The project's **Posts** tab shows each post's views, how much was watched and engagement, and where it did best.

### Strategy

Pick a brand on the Performance page to see its **Strategy**: what the numbers taught you, kept in the brand's `strategy.md`.

- **Update from the numbers** has Claude read every video's numbers and rewrite the "What the numbers say" part, about $0.05. Your own parts are kept.
- **Edit** lets you write your own plan or fix a lesson.
- The editor agent follows the strategy in every new video, and adds a dated note when it learns something.

37Studio reads the numbers every 6 hours while it's running; click **Refresh** to read them now. Platforms take up to 2 days to settle their numbers. TikTok reports no watch time.

The editor agent sees the same numbers. Every new video starts with a summary of what worked for the brand, and the agent can look up the full list.

The numbers need one more permission per platform. Meta needs `read_insights` and `instagram_manage_insights`, YouTube needs the YouTube Analytics API, and TikTok needs `video.list`. After adding it, connect the account again on the brand's **Social** tab. See the [platform guide](https://github.com/Shaitrish/37studio/blob/main/docs/platforms.md#performance).

## Assistant

**Assistant** in the sidebar runs the studio by chatting. It can do what the app's buttons do, and nothing else: no shell and no file edits. Try:

- "Add this brand from its website: https://…"
- "Plan three reels a week for a brand at 8 pm with my approval"
- "Publish yesterday's video to all its Pages"
- "What did we spend this month, by brand?"

It asks before it deletes anything, publishes, approves an Autopilot video or replies to a customer. Each reply's cost shows in the chat and on the Costs page. **New conversation** clears the chat.

## Use from other AI apps

Claude Code, Claude Desktop, Cursor and other apps that support MCP can run 37Studio from their own chat, with the same tools as the Assistant. **Settings → Other AI apps** shows your address and command.

```bash
claude mcp add --transport http 37studio http://localhost:8080/mcp
```

Claude Desktop connects through `mcp-remote`, in `claude_desktop_config.json`:

```json
{ "mcpServers": { "37studio": { "command": "npx", "args": ["-y", "mcp-remote", "http://localhost:8080/mcp"] } } }
```

Cursor, in `.cursor/mcp.json`:

```json
{ "mcpServers": { "37studio": { "url": "http://localhost:8080/mcp" } } }
```

With a password set (on a server), add `--header "Authorization: Basic $(printf '37studio:%s' "$PASSWORD" | base64)"` to the Claude Code command.

Connected apps can spend on your AI keys and post to your accounts, so connect only ones you trust.

## Costs and spending

Every project, campaign and brand shows what it cost in AI usage, and the **Costs** page breaks it down by service, brand and month. There's no markup, because you pay the AI services directly. Rendering, transcription and storage run on your machine, so they are free.

Two limits are in **Settings → Spending**:

| Setting | Default | What it does |
| --- | --- | --- |
| Spending limit per agent turn | $25 | The agent stops if one turn would cost more |
| Ask before media costs more than | $5 per project | The agent asks before images, clips, voice and music go past it. 0 means always ask |

The per-turn limit covers the agent's own model use. Generated media is billed on top, so check the Costs page if you run Autopilot.

Costs are recorded in US dollars and shown in your currency, which you set in the same place with its exchange rate.

## Settings

| Section | What's there |
| --- | --- |
| Get started | A checklist of what's set up |
| AI services | Claude, other agent models, and the image, video, voice and music services, with their models and prices |
| Social apps | Your developer app for each platform |
| Spending | Spending limits, your currency and exchange rate |
| Performance | Export speed: parallel workers, draft frame rate, graphics card encoding |
| Storage | What fills your disk, clearing old drafts, and backups |
| Other AI apps | Your MCP address, for connecting other AI apps |
| About | Version, updates and a system check |

The theme button at the bottom of the sidebar switches between system, light and dark.

## Back up and update

### Back up

**Settings → Storage → Back up** downloads one zip, with or without videos and footage. It holds your API keys and account logins, so keep it private. To restore it, close 37Studio, unzip it into an empty data folder, and open 37Studio again.

The same section shows what fills your disk and can clear out old drafts. Final videos, sources and files a waiting post needs are kept.

### Update

The sidebar says when a new version is out, and **Settings → About** tells you how to get it.

- **Ready-made app:** close 37Studio and open it again from the launcher.
- **Desktop app:** download the next version from [the desktop page](https://studio.shaitrish.com/desktop) and install it over the old one. On Windows, close 37Studio and run the new installer.
- **Server:** run the command below in the app's folder.

```bash
docker compose -f docker-compose.cloud.yml pull && docker compose -f docker-compose.cloud.yml up -d
```

## Keyboard shortcuts

| Keys | Where | What it does |
| --- | --- | --- |
| ? | Anywhere | Open help for this page |
| Esc | Help | Close help |
| Ctrl+B | Anywhere | Collapse or expand the sidebar |
| Enter | Chats | Send |
| Shift+Enter | Chats | New line |
| Esc | Chats | Stop the agent or the Assistant |
| F | Project | Preview full screen |
| Ctrl+Enter | Inbox | Send the reply |

On a Mac, use Cmd where this says Ctrl.

## Troubleshooting

**The page doesn't open, or port 8080 is in use.** Another program has port 8080. Close it and run the start file again. `docker compose logs` shows what the app is doing.

**Linux: the logs say permission denied on `/data`.** Run `sudo chown -R 1000:1000 data` in the 37Studio folder and start it again.

**The agent says there's no Claude key.** Add one in **Settings → AI services**. A plan token that stopped working can be replaced with a new one from `claude setup-token`.

**A button is greyed out.** **Edit timeline** and **Export** wait for the agent's first version, and **Publish** needs an exported video. **Connect** needs the platform's app keys in **Settings → Social apps**.

**Connect says a platform is not configured.** Its app keys are missing. Add them in **Settings → Social apps**.

**The platform says the redirect address doesn't match.** It must match exactly: protocol, host, port and no trailing slash. See [Connect social accounts](#connect-social-accounts).

**A YouTube upload is private, or TikTok only offers Only me.** Both platforms restrict apps they haven't audited.

**LinkedIn posts started failing.** LinkedIn sign-ins last 60 days. Connect it again.

**A scheduled post failed with the note "Missed".** 37Studio wasn't running then. Click **Retry** to post now.

**The Inbox says some accounts can't be read.** Open **Details** on that notice. It's usually missing Meta permissions, or Instagram's **Allow access to messages** setting being off.

**The timeline editor doesn't open on a server.** Check that ports 3100–3109 are open in every firewall, and enter the password again when the browser asks.

**Rendering is slow.** Docker Desktop gets only a share of your machine: give it more memory and CPU under its Settings → Resources. **Settings → Performance** in 37Studio has more options.

**The desktop window doesn't open on Ubuntu.** Ubuntu needs an AppArmor profile. The .deb from 0.9.1 on adds it. For anything else, see the [desktop troubleshooting](https://github.com/Shaitrish/37studio/blob/main/docs/desktop.md#troubleshooting).

**The desktop app can't find Claude or ffmpeg.** Install them, then click **Check again** under **Finish setup** on the Overview page.

## What it can't do

These are limits of the platforms, not settings you can change.

- **Threads:** its API only takes videos from a public web address, which a copy on your computer doesn't have.
- **Facebook Groups and personal profiles:** Meta removed posting to them.
- **Reddit and Snapchat:** no usable video API.
- **Image posts:** publishing sends videos only for now.
- **Your Runway, Kling or similar subscription:** those plans work only on their own sites. Make the clip there and upload it, or add a Gemini or fal.ai key to generate inside 37Studio.

Everything scheduled (Autopilot, posts, auto-replies) needs 37Studio to be running, so keep the computer awake or use a server.

## Get more help

- [Connect your social accounts](https://studio.shaitrish.com/connect), step by step
- [Platform setup in detail](https://github.com/Shaitrish/37studio/blob/main/docs/platforms.md)
- [Desktop app guide](https://github.com/Shaitrish/37studio/blob/main/docs/desktop.md)
- [How 37Studio works](https://github.com/Shaitrish/37studio/blob/main/docs/architecture.md)
- [What changed in each version](https://github.com/Shaitrish/37studio/blob/main/CHANGELOG.md)
- [Report a bug or ask a question](https://github.com/Shaitrish/37studio/issues)

Prefer it done for you? Shaitrish does paid setup over a screen share. [Message on WhatsApp](https://wa.me/8801635191148?text=Hi%2C%20I%27d%20like%20help%20setting%20up%2037Studio).
