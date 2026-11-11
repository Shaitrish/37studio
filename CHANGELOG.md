# Changelog

What changed in each release of 37Studio, newest first. Your copy shows when a newer version is out; close 37Studio and open it again from the launcher to update.

## [Unreleased]

### Added
- **New post**: post a video or photo from your computer to the brand's accounts, without the agent. Click **New post** on the Overview, Projects or a brand's page, drop the file in, and **Write with AI** writes a caption for each platform after looking at the photo or at frames from the video. Post now or schedule it; it shows in Projects with its Posts tab and numbers.
- **Photos on social media**: Publish can post a photo (JPG, PNG, WebP or GIF) to Facebook Pages, X, LinkedIn, Pinterest, Bluesky and Mastodon. Instagram and TikTok only take photos from a public web address and YouTube takes no photos, so Publish greys those accounts out and says why.

## [0.12.1] - 2026-11-11

### Changed
- **The message box works like Claude Code's**, in the editor agent's and the Assistant's chat: the microphone sits in its corner with a বাং / EN switch, **+** attaches screenshots, **/** opens quick actions, and the row below shows how many subagents are working, the model and its effort, the files going with your message, and how long the agent has been at it.
- The version you picked under the preview goes with your message, so "make this one shorter" means that file. Click × on it to leave it out.

### Fixed
- The mic in the chat now works in the desktop app: it records your voice and turns it into text on this computer (English in a few seconds, Bangla takes longer).

## [0.12.0] - 2026-11-11

### Added
- **Windows installer**: download one setup file, open it and 37Studio is installed with Start-menu and desktop shortcuts, no admin rights needed. It shows under Settings → Apps with Uninstall, and running a newer one updates it. The .zip is still there for those who prefer it.
- **Download straight from the website**: the Download buttons on studio.shaitrish.com now download the app for your computer at once (the Windows installer or the Linux .deb) instead of opening GitHub.
- **Chat like Claude Code**: in the editor agent and Assistant chats, press Up for the messages you sent before and Down to come back to what you were typing. Each step reads as one "● Tool(what)" row, with an error the agent got back shown under it.
- **Copy your messages**: hover over one of your messages, or the agent's reply, and click the copy button. Selected text in your own messages now shows as selected, too.
- **Show in folder**: a folder button on each video in the versions row opens its folder on your computer with the video selected, on Linux, Windows and macOS. It shows in the desktop app and when 37Studio runs on this computer without Docker.
- **Website and README**: a 30-second, sped-up look at the real app, from a brand's library to a posted video and its numbers. The README is much shorter, with the Docker and server setup moved to its own page.

### Fixed
- **New version notice**: when an update is out, the sidebar shows **Update to …** with an icon (also with the sidebar collapsed), you get a one-time message when you open 37Studio, and a window left open for days checks again. On Windows, Settings → About now points to the Windows installer, not the Linux .deb.

## [0.11.0] - 2026-11-07

### Added
- **Subagents in the chat**: when the editor agent splits work into helpers that run at the same time, each one shows as its own row, with what it's doing now, its steps and when it's done. The agent now uses them for work that doesn't depend on each other.
- **Chat width**: drag the gap between the preview and the chat to make the chat wider or narrower (double-click resets it), or widen it to the whole page with the button beside **Editor agent**. 37Studio remembers your choice.
- **Connected folder** on a brand: a folder on your computer the agent reads live and can't change, with nothing copied. It can't read `.env` files, keys or raw databases there.
- **Databases the agent may query**: read-only queries on the SQLite files you list in that folder, through the new `37studio sql` command.
- **Performance** in the sidebar: views, how much of each video people watched, average watch time, likes, comments, shares, saves and engagement for every video on your Facebook Pages, Instagram, YouTube and TikTok, sortable, with the most and least viewed on top. 37Studio reads them every 6 hours, or when you click **Refresh**. Each needs one more permission; see the Guide.
- The editor agent now starts each video knowing how the brand's published videos did, and can list them with `37studio stats`. Other AI apps get the same numbers through the `video_stats` tool.
- **Make it the brand icon**: the pin button on a library file puts it first; the first logo is the brand's icon everywhere in the app.
- **Published** in the Library: every video you posted from 37Studio, with its views on each platform. Open one to compare all its numbers side by side and see where it did best. The project's Posts tab shows the same numbers.
- **By platform** on the Performance page compares views per video, how much was watched and engagement across platforms.
- **Strategy** for each brand on the Performance page: Claude turns the numbers into lessons on request, you can edit it, and the editor agent follows it in every new video and adds notes with `37studio strategy add`.

### Changed
- Opening a project shows its layout at once while it loads, instead of a blank page.
- Projects and the Library open several times faster the first time after a restart, and pages you've already visited open instantly.
- The project page checks for new messages less often while the agent is idle, and not at all while the window is hidden, so 37Studio uses less of your computer.
- TikTok's **Send to TikTok drafts** is now **Send to the TikTok app**: the video arrives as a notification in the TikTok phone app's Inbox (it never showed under Drafts in TikTok Studio on the web).

### Fixed
- **Inbox**: the post above a comment no longer gets cut off halfway through a line. It shows three lines, with **Show more** for the rest.
- A brand's website scan no longer leaves its steps in the Assistant's chat.
- **Inbox**: a Facebook or Instagram message older than 24 hours now says so above Send, and if Facebook refuses it you get a plain explanation instead of its error code: answer it from Meta Business Suite (7 days), or from 37Studio once Meta approves your app's Human Agent permission.
- When Instagram's API refuses a Reel (since October 2026 it does this to many longer than about 40 seconds), the post now says so and what to do: post that one from the Instagram app, or cut it under 40 seconds.

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
