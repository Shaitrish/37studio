<h1 align="center">37Studio</h1>

<p align="center"><b>The open-source AI video studio that also posts your videos and answers your comments.</b><br>
Brief a reel, an ad or an explainer. An AI agent builds it in your brand, renders it on your computer, posts it to nine platforms and drafts the replies.<br>
An open-source alternative to Opus Clip, Buffer and Claude Motion in one app, with your own keys and no monthly bill.</p>

<p align="center">
  <a href="https://studio.shaitrish.com/desktop"><b>Download for Windows &amp; Linux</b></a> ·
  <a href="https://studio.shaitrish.com">Website</a> ·
  <a href="docs/guide.md">User guide</a> ·
  <a href="CHANGELOG.md">Changelog</a> ·
  <a href="https://studio.shaitrish.com/donate">Donate</a>
</p>

<p align="center">
  <a href="https://github.com/Shaitrish/37studio/stargazers"><img src="https://img.shields.io/github/stars/Shaitrish/37studio?style=flat&color=d4502a" alt="GitHub stars"></a>
  <a href="https://github.com/Shaitrish/37studio/releases"><img src="https://img.shields.io/github/v/release/Shaitrish/37studio?color=555" alt="Latest release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-d4502a" alt="MIT License"></a>
  <img src="https://img.shields.io/badge/runs%20on-Windows%20%C2%B7%20Linux%20%C2%B7%20macOS%20%C2%B7%20server-555" alt="Windows, Linux, macOS, server">
  <a href="https://skillsllm.com/skill/37studio"><img src="https://skillsllm.com/badges/security.svg?slug=37studio" alt="SkillsLLM Automated Security Scan"></a>
</p>

<p align="center">
  <a href="#made-with-37studio">Made with 37Studio</a> ·
  <a href="#install">Install</a> ·
  <a href="#use-it-from-your-ai-agent">Use it from your agent</a> ·
  <a href="#why-37studio">Why 37Studio</a> ·
  <a href="#what-it-does">What it does</a> ·
  <a href="#platforms">Platforms</a>
</p>

[![37Studio at work, sped up: a brand's library, a brief to the editor agent, the agent building the video, the preview, publishing and the results](https://studio.shaitrish.com/demo.gif)](https://studio.shaitrish.com/#demo)

## Made with 37Studio

Reels the editor agent made for BCS Mate, a Bangla exam-prep brand, and posted from the app. Views are Instagram's and TikTok's own numbers, read by 37Studio's Performance page on 22 November 2026. Click one to watch it.

<table>
  <tr>
    <td align="center" width="33%"><a href="https://www.instagram.com/reel/Df7CEWxCM-N/"><img src="docs/readme/mouth.webp" width="220" alt="Reel: the alphabet is a map of your mouth, with an animated 3D head"></a><br><b>The alphabet is a map of your mouth</b><br><sub>211,529 views · Instagram + TikTok</sub></td>
    <td align="center" width="33%"><a href="https://www.instagram.com/reel/DfRSnntChEJ/"><img src="docs/readme/quake.webp" width="220" alt="Reel: why Dhaka shakes, with a spinning globe and maps"></a><br><b>Why Dhaka shakes</b><br><sub>210,764 views · Instagram</sub></td>
    <td align="center" width="33%"><a href="https://www.instagram.com/reel/DfRVGGJCm9m/"><img src="docs/readme/letter.webp" width="220" alt="Reel: this plus sign is a Bangla letter, with old inscriptions"></a><br><b>This plus sign is a Bangla letter</b><br><sub>181,902 views · Instagram</sub></td>
  </tr>
  <tr>
    <td align="center" width="33%"><a href="https://www.instagram.com/reel/DfeM3_xC0TH/"><img src="docs/readme/whale.webp" width="220" alt="Reel: whales in Bangladesh's sea, with maps and archive photos"></a><br><b>Whales in Bangladesh's sea</b><br><sub>114,903 views · Instagram + TikTok</sub></td>
    <td align="center" width="33%"><a href="https://www.instagram.com/reel/DfZrGEeiIx8/"><img src="docs/readme/radcliffe.webp" width="220" alt="Reel: the man who drew the border, with archive maps"></a><br><b>The man who drew the border</b><br><sub>77,863 views · Instagram</sub></td>
    <td align="center" width="33%"><a href="https://www.instagram.com/reel/DfPS-LmiEqh/"><img src="docs/readme/time.webp" width="220" alt="Reel: what time is it 45 degrees east, with a globe and a quiz"></a><br><b>What time is it 45° east?</b><br><sub>36,638 views · Instagram</sub></td>
  </tr>
</table>

**3D scenes.** The agent builds these in code too: a rainy 1757 battlefield, a night sky where each weekday is a planet, a cyclone over the Bay of Bengal. Watch them on [YouTube](https://www.youtube.com/@bcsmate/shorts).

<table>
  <tr>
    <td align="center" width="33%"><a href="https://youtu.be/iaKL8c785gU"><img src="docs/readme/plassey.webp" width="220" alt="3D reel: a rainy 1757 battlefield with cannons"></a><br><b>The rain at Plassey</b><br><sub>3D battlefield</sub></td>
    <td align="center" width="33%"><a href="https://youtu.be/2Oopkd-f6zE"><img src="docs/readme/week.webp" width="220" alt="3D reel: the seven weekdays as planets in the night sky"></a><br><b>Why Monday comes after Sunday</b><br><sub>3D night sky</sub></td>
    <td align="center" width="33%"><a href="https://youtu.be/MWf6_kbW7mI"><img src="docs/readme/cyclone.webp" width="220" alt="3D reel: a cyclone spinning over the Bay of Bengal on a globe"></a><br><b>Why Bay of Bengal cyclones are so deadly</b><br><sub>3D globe</sub></td>
  </tr>
</table>

Each one started as a short brief. The agent wrote it, built the maps and motion from real pictures, checked its own frames and rendered it on the brand's own computer.

## Install

**Desktop app (Windows 10/11 and Linux).** Download it from [studio.shaitrish.com/desktop](https://studio.shaitrish.com/desktop):

- **Windows:** open the installer. It needs no admin rights and adds 37Studio to the Start menu and desktop. Windows may say it protected your PC, because the app isn't signed yet: **More info** → **Run anyway**.
- **Ubuntu 24.04+ / Debian 13+:** `sudo apt install ./37studio_*_amd64.deb` (it brings ffmpeg and Python).

Then install the tools the agent uses. Overview → **Finish setup** in the app lists anything missing:

```bash
winget install Git.Git Gyan.FFmpeg OpenJS.NodeJS.LTS Python.Python.3.13   # Windows only
npm install -g @anthropic-ai/claude-code    # Node.js 22+; on Linux with sudo
```

[docs/desktop.md](docs/desktop.md) covers other Linux, the .zip, building it yourself and troubleshooting.

**Docker (macOS, or a server).** Every tool comes bundled. On your computer, install Docker Desktop, unzip [37studio.zip](https://studio.shaitrish.com/37studio.zip) and double-click `start.bat`, `start.command` or `start.sh`. For a team, or replies and Autopilot around the clock, run it on a Linux server with HTTPS and a password. [docs/docker.md](docs/docker.md) has both, plus building from source and every setting.

## Use it from your AI agent

Already working in Claude Code, Codex, Cursor or another agent? Give it the 37Studio skill and it can make, post and answer for you:

```bash
npx skills add Shaitrish/37studio
```

Or connect the MCP server directly. Every screen in the app is a tool: brands, videos, publishing, Autopilot, inbox, stats and settings.

```bash
claude mcp add --transport http 37studio http://localhost:8080/mcp
```

The editor agent inside the app can be Claude Code (a Claude plan or API key), Codex (a ChatGPT plan) or OpenCode (any model on OpenRouter, xAI, OpenAI, Kimi, DeepSeek, Qwen and more).

## Why 37Studio

Most tools do one step. 37Studio does the whole job, from the brief to the reply under the post.

| | 37Studio | AI video agents | Clip tools | Schedulers |
| --- | :---: | :---: | :---: | :---: |
| Builds a new video from a brief, in your brand | ✅ | ✅ | — | — |
| Turns a long recording into many reels | ✅ | — | ✅ | — |
| Copies the style of a reel you like | ✅ | some | — | — |
| Posts to 9 platforms, now or on a schedule | ✅ | — | some | ✅ |
| Autopilot: a fresh video before every slot | ✅ | — | — | — |
| Drafts replies to messages and comments | ✅ | — | — | some |
| Reads each video's results before planning the next | ✅ | — | — | some |
| Runs on your own computer, your own keys | ✅ | some | — | — |
| Price | Free, MIT | varies | monthly | monthly |

## One-Click Deploy

[![Deploy on RepoCloud](https://d16t0pc4846x52.cloudfront.net/deploylobe.svg)](https://repocloud.io/details/37Studio/)

## First steps

1. **Add your Claude key** in Settings → AI services (an Anthropic API key, or a Claude plan token from `claude setup-token`).
2. **Add a brand** from its website or a folder. In 5 to 10 minutes its library, design system and knowledge base are ready.
3. **Make a video.** **New video** asks what to make; the agent builds it while you watch, and takes changes in plain words.
4. **Connect accounts** on the brand's **Social** tab ([how](docs/platforms.md)), then **Publish** now, on a schedule, or with Autopilot.

The [user guide](docs/guide.md), also inside the app as **Guide**, covers every screen.

## What it does

- **Brand library from a website or folder.** Paste a URL, or pick the brand's own project folder, and get its logos, people, photos, fonts and exact colours sorted and captioned, plus a design system (`design.md`) and a knowledge base (`knowledge.md`).
- **An editor agent.** Claude Opus 5.5 (or Codex, or any model through OpenCode) builds each video as an animated HyperFrames composition, checks its own frames, sound and pacing, and takes changes in plain words. Edit by hand on a real timeline when you want to.
- **Remake a video you like.** Drop in or link a reel or ad, and the agent studies its shots, then rebuilds its pacing, structure and motion as an original video in your brand.
- **Many kinds of output.** Hook reels, captioned clips, motion-graphics explainers, remakes, podcast clips, promo ads, square and portrait posts, and thumbnails. Save any finished video as a template for the next one.
- **Long recordings into many videos.** A 2-hour class becomes ten reels. Transcription runs locally with faster-whisper: free, private, about 100 languages. YouTube and Google Drive links import directly.
- **Generated media.** Gemini images; video clips from Gemini or, through fal.ai, Seedance, Kling, Veo, Wan and others; ElevenLabs voiceover, sound effects and music. Each brand can keep its own voices, cloned from a short recording of the founder or presenter, with a default per language; ask the agent for a voice by name, or to switch a finished video to it. The agent asks before media on one project costs more than the limit in Settings → Spending, and keeps a person or product consistent by reusing one reference image across shots. The agent uses them only when the footage and brand library can't do the job, and says what it made.
- **Publishing and Autopilot.** Post to any mix of a brand's accounts on nine platforms now or on a schedule, or let a content plan make a fresh video before every slot and wait for one-click approval.
- **One inbox.** Messenger, Instagram and WhatsApp messages and Facebook and Instagram comments, with replies drafted in the customer's language, following the brand's own rules. Off, drafts to approve, or fully automatic, per brand.
- **Performance.** Views, how much of each video people watched, likes, comments, shares, saves and engagement for every video on the brand's Facebook, Instagram, YouTube and TikTok, best and worst on top. The editor agent reads the same numbers before it plans the next video.
- **Assistant and MCP.** Run everything by chatting with the Assistant in the sidebar, or from your own AI agent (Claude Code, Claude Desktop, Cursor) through 37Studio's MCP server: brands, campaigns, videos, publishing, Autopilot, settings and connections.
- **Real costs.** Every project, campaign and brand shows its AI spend in your currency. No markup: you pay the AI providers directly.

## Platforms

| Platform | Publishes | Inbox | You connect with |
| --- | --- | --- | --- |
| Facebook Pages | Reels and Page videos | Messenger, comments | Facebook Login through your own Meta app |
| Instagram (Business or Creator) | Reels | Messages, comments | The same sign-in as Facebook |
| YouTube | Videos | | Your own Google OAuth client |
| TikTok | Videos, or to the creator's drafts | | Your own TikTok app |
| LinkedIn | Videos to your profile; company Pages with LinkedIn's approval | | Your own LinkedIn app |
| X | Video posts | | Your own X app |
| Pinterest | Video Pins, on the boards you pick | | Your own Pinterest app |
| Bluesky | Video posts | | Handle and an app password |
| Mastodon | Video posts | | Server URL and an access token |
| WhatsApp | | Messages | A WhatsApp Business API number and a Cloudflare Tunnel to your copy (or your own relay) |

Not possible: **Threads** (its API only takes a video from a public URL, which a local-first app doesn't have), **Facebook Groups and personal profiles** (Meta removed it), **Reddit and Snapchat** (no usable public video-posting API). Setup for each one: [docs/platforms.md](docs/platforms.md).

## How it works

```
Browser ──► 37studio (Go, one binary) ──► SQLite + data/ (brands, projects, encrypted tokens and keys)
             ├─ claude -p, the editor agent ──► ffmpeg · npx hyperframes · 37studio gen / transcribe / clip / check
             ├─ hyperframes render (Chrome + ffmpeg) ──► out/*.mp4, *.png
             ├─ post queue ──► Facebook · Instagram · YouTube · TikTok · LinkedIn · X · Pinterest · Bluesky · Mastodon
             └─ inbox loop ──► Messenger, Instagram (Graph API) · WhatsApp (webhook through your tunnel, or your own relay)
```

Claude Code does the editing, HyperFrames renders, Go serves and React is the UI. Any MCP client (Claude Code, Claude Desktop, Cursor) can run the whole app through its MCP server at `http://localhost:8080/mcp`: `claude mcp add --transport http 37studio http://localhost:8080/mcp`. [docs/architecture.md](docs/architecture.md) has the job flow, the agent harness, a code map and the release process.

**What leaves my computer?** Calls to the AI providers you set up, uploads to the platforms you connect, and the update check (GitHub Releases). Nothing is sent to Shaitrish. Questions and problems: the guide's [Troubleshooting](docs/guide.md#troubleshooting), or [open an issue](https://github.com/Shaitrish/37studio/issues).

## Support the project

37Studio is free and stays free. If it saves you time, you can [donate](https://studio.shaitrish.com/donate) by US bank transfer or bKash, star the repo, or share what you made with it. Want it set up for you? Shaitrish does paid setup over a screen share: [message us on WhatsApp](https://wa.me/8801635191148?text=Hi%2C%20I%27d%20like%20help%20setting%20up%2037Studio).

## Star history

[![Star history](https://api.star-history.com/svg?repos=Shaitrish/37studio&type=Date)](https://star-history.com/#Shaitrish/37studio&Date)

## Contributing

Bug reports, ideas and pull requests are welcome: [CONTRIBUTING.md](CONTRIBUTING.md) covers running it from source and the checks to run. Report security problems privately, as [SECURITY.md](SECURITY.md) describes.

## License

[MIT](LICENSE) © Shaitrish. The open-source tools 37Studio bundles (HyperFrames, ffmpeg, faster-whisper and others) keep their own licenses. Claude Code is Anthropic's software, used under Anthropic's terms.
