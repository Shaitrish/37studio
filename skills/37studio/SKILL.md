---
name: 37studio
description: Make on-brand videos and run a business's social accounts through 37Studio, the free, open-source AI video studio that runs on the user's own computer. Use when the user wants a reel, ad, explainer, captioned clip, thumbnail or a remake of a video they like for their brand; wants long recordings cut into many short videos; wants to post or schedule videos to Facebook, Instagram, YouTube, TikTok, LinkedIn, X, Pinterest, Bluesky or Mastodon; wants Autopilot (a video made and queued before every posting slot); or wants replies drafted to their Messenger, Instagram and WhatsApp messages and comments.
---

# 37Studio

37Studio runs on the user's machine at `http://localhost:8080` (desktop app or Docker). Everything a person can do in it, you can do through its MCP server at `http://localhost:8080/mcp`.

## 1. Make sure it's running and connected

```bash
curl -s http://localhost:8080/api/meta >/dev/null && echo running || echo "not running"
```

Not running: send the user to https://studio.shaitrish.com/desktop (Windows and Linux app) or https://studio.shaitrish.com/37studio.zip (Docker, for macOS and servers). It is free; they bring their own Claude plan or API key.

If the `37studio` MCP tools aren't available to you yet, connect them, then ask the user to restart this session:

```bash
claude mcp add --transport http 37studio http://localhost:8080/mcp
```

Other apps: Cursor uses `{ "mcpServers": { "37studio": { "url": "http://localhost:8080/mcp" } } }`; Claude Desktop uses `npx -y mcp-remote http://localhost:8080/mcp`. On a server with a password, add a Basic auth header with user `37studio`.

## 2. Work through the tools

- **Start with `get_options`**: the video kinds (hook-reel, captions, explainer, promo, remake, thumbnail, custom), quality tiers and which AI keys are set.
- **A brand first.** `list_companies`; if theirs is missing, `create_company_from_website` with their URL. Its library, design system and knowledge base take 5 to 10 minutes; watch `scan_status` with `get_company`.
- **A video.** `create_video` with the brand, kind, brief, aspect (9:16 for Reels, TikTok and Shorts) and length. Pass `footage_url` for a YouTube or Google Drive link, or the reel to copy for `remake`. The editor agent works in the background for several minutes: follow it with `get_project_messages` and `get_project`, and pass the user's changes in plain words with `send_message`.
- **Finished?** `export_video` renders the final into `out/`. `resize_video` makes other shapes.
- **Many videos from one recording**: create one `hook-reel` with the recording and ask in the brief for "10 separate videos"; the agent plans and queues them.
- **Publishing.** `list_social_accounts`, `write_captions`, then `publish_video` now or at a time. `create_autopilot` makes a fresh video before every slot and waits for approval.
- **Inbox.** `list_inbox`, `get_inbox_thread`, `draft_reply`, `send_reply`.
- **Results and costs.** `video_stats`, `published_videos`, `get_costs`.

## Rules

- Ask the user before anything public or destructive: publishing, approving an Autopilot video, sending an inbox reply, deleting. Show them what will go out.
- Paid media (generated images, video clips, voices) costs the user money on their own keys. Say so before asking for a lot of it.
- Don't promise what the platforms don't allow: no Threads, no Facebook Groups or personal profiles, no Reddit or Snapchat video posting.

Full guide: https://github.com/Shaitrish/37studio/blob/main/docs/guide.md
