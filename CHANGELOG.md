# Changelog

What changed in each release of 37Studio, newest first. Your copy shows when a newer version is out; close 37Studio and open it again from the launcher to update.

## [Unreleased]

## [0.10.0] - 2026-11-06

### Added
- **Guide** in the sidebar: how to set up and use every part of 37Studio, searchable and readable offline, with buttons that open the page it talks about. The same text is in `docs/guide.md`.
- **Help** at the top of every page and in most windows opens the part of the Guide about that screen, beside the page, so you keep your place. Press **?** for the same thing.
- **Learn 37Studio** on the Overview page links the first steps, and **Finish setup** has a **Setup guide** button and buttons that open AI services.

### Changed
- The browser tab now shows which page you're on.
- **Costs** moved out of the sidebar's Social group, into Studio.
- Messages, help text and docs now name the Settings sections the app really has (AI services, Social apps) instead of the old "Settings → Keys", and say "brand" where they still said client or company.

## [0.9.1] - 2026-11-04

### Fixed
- **Desktop app on Ubuntu**: the window now opens from the app menu. The .deb adds the AppArmor profile Ubuntu 23.10 and newer require before WebKit can draw the window; without it 37Studio closed straight away unless started from a terminal.

## [0.9.0] - 2026-11-04

### Added
- **Desktop app for Linux and Windows (preview)**: 37Studio in its own window, without Docker. Each release has a .deb for Ubuntu 24.04+ and Debian 13+ and a plain program for other Linux, for amd64 and arm64, and a .zip for Windows 10 and 11 on Intel and AMD. On Windows, unpack it and run `install-windows.ps1` to put 37Studio in the Start menu; your work is kept in `%AppData%\37Studio`. Download it from [studio.shaitrish.com/desktop](https://studio.shaitrish.com/desktop). Links, the timeline editor and social sign-ins open in your browser, and downloads and backups ask where to save. The Overview page lists the tools still missing on your computer, with the command to install them — `winget` commands on Windows, including Git for Windows, whose bash is what the agent runs its commands through there. See docs/desktop.md.
- **Brand voices** (a brand's Voices tab): clone a person's voice from a short recording, or add any voice from your ElevenLabs account, and pick a default for each language. The agent speaks voiceovers in the brand's default, or in any of its voices you name in chat.
- **Change a video's voice**: ask the agent to "use Rahim's voice", even on a finished video, and it speaks the existing voiceover again in that voice with the same words and timing, so captions stay in sync. Then press Export again.

### Changed
- `docker stop` and restarts now let 37Studio finish what it's doing and stop running agent turns and renders, instead of leaving them running in the background.

## [0.8.0] - 2026-10-27

### Added
- **Storage page** (Settings → Storage): how much space each project and brand library takes, the free space left, and a button to clear out drafts older than 7, 30 or 90 days.
- **Backup**: download everything 37Studio keeps as one zip, with or without videos and footage. The zip holds your keys and account logins, so keep it private.
- **Delete several at once** on the Projects page and in the Library.
- Delete finished videos and images from the Library, and generated media from a project's Generate tab.
- The Assistant can run on OpenCode's models too.
- A knowledge base preview, a sidebar toggle (Ctrl+B), and a new 2-minute intro in English and Bangla.

### Changed
- Deleting a brand now takes its projects, campaigns, templates, Autopilot plans and connected accounts with it, after a confirm that says how many projects go. Before, it refused until every project was deleted by hand.
- The per-turn spending limit now stops OpenCode runs too, for the Assistant and the editor agent.
- Messages typed while the agent is working survive a restart; they go out with your next message.
- The app loads faster: each page loads when you first open it.

### Fixed
- A file a scheduled or awaiting-approval post needs can't be deleted, so the post can't fail with a missing file.
- A brand with a campaign couldn't be deleted at all.
- Outbound connections check the address they actually reach, so a link can't be pointed at your own network after it was checked. The Assistant's "add a file from a URL" had no check before.

## [0.7.0] - 2026-10-25

### Added
- Codex with your ChatGPT plan, no API key: Settings → AI services → Codex → Sign in with ChatGPT.
- GPT, Grok, Kimi, GLM, DeepSeek and other models through OpenCode.
- Video clips from Seedance, Kling, Veo or Wan through fal.ai.
- The agent asks before media spending on a project passes your limit.

### Changed
- Codex and OpenCode install on first use, so the download is smaller.
- Settings shows Claude as connected when Claude Code is already signed in on this computer.

### Fixed
- Assistant voice input and model choice, project media tabs, brand logos, and the currency picker.

## [0.6.0] - 2026-10-25

### Changed
- 37Studio no longer uses any server of ours. Facebook and Instagram sign in through your own Meta app, WhatsApp comes straight to your copy through a Cloudflare Tunnel (or your own relay), and the update check asks GitHub.
- A redesigned Settings page, with keys grouped by provider and setup links.
- Project media grouped into voiceover, music, sound effects, images and clips.

### Added
- A plain-language guide for connecting each platform: [studio.shaitrish.com/connect](https://studio.shaitrish.com/connect).
- Kimi K3, Qwen 3.8 Max and GLM 5.3 as OpenCode models.

### Fixed
- The issues found in the first security review (see SECURITY.md for how to report one).

## [0.5.0] - 2026-10-23

### Added
- 37Studio is open source under the MIT License: no license keys, trials or paid tiers.
- The Assistant: run the whole studio by chatting.
- An MCP server, so other AI apps (Claude Code, Claude Desktop, Cursor and others) can run 37Studio.
- Posting to LinkedIn, X, Pinterest, Bluesky and Mastodon, for nine platforms in all.
- Templates: start a new video from an earlier video's look.
- Several videos from one source, and a Library of every finished video.

[Unreleased]: https://github.com/Shaitrish/37studio/compare/v0.9.1...HEAD
[0.9.1]: https://github.com/Shaitrish/37studio/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/Shaitrish/37studio/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/Shaitrish/37studio/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/Shaitrish/37studio/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/Shaitrish/37studio/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/Shaitrish/37studio/releases/tag/v0.5.0
