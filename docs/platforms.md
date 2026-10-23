# Connecting social platforms

How to set up each platform 37Studio posts to, plus the inbox (Messenger, Instagram, WhatsApp). The [README](../README.md#social-publishing) has the short version.

- [How connecting works](#how-connecting-works)
- [Facebook Pages + Instagram](#facebook-pages--instagram)
- [YouTube](#youtube)
- [TikTok](#tiktok)
- [LinkedIn](#linkedin)
- [X](#x)
- [Pinterest](#pinterest)
- [Bluesky](#bluesky)
- [Mastodon](#mastodon)
- [Inbox: Messenger, Instagram and comments](#inbox-messenger-instagram-and-comments)
- [WhatsApp](#whatsapp)
- [Run your own relay](#run-your-own-relay)
- [What can't be posted to](#what-cant-be-posted-to)

## How connecting works

Each company's **Social** tab has a **Connect** button per platform. For the OAuth platforms (Facebook & Instagram, YouTube, TikTok, LinkedIn, X, Pinterest) you sign in once on the platform's own page with an account that manages the company's Pages, channels or boards, then tick which of the returned accounts belong to that company. A company can have any number of accounts on each platform. Bluesky and Mastodon take a handle and app password, or a server and access token, instead.

Every OAuth platform needs a developer app: the platform only lets an app it knows ask for permission to post. 37Studio has no central server, so for every platform except Facebook & Instagram you register your own app (free) and give 37Studio its credentials:

- **Launcher app:** Settings → Keys. The launcher bundle has no `.env`.
- **From source:** Settings → Keys, or the variables in `.env`.

Credentials entered in Settings → Keys are encrypted at rest (AES-GCM, with a key file in the data folder). A few settings exist only as environment variables (`LINKEDIN_PAGES`, `PINTEREST_API`, `STUDIO37_PUBLIC_URL`, `STUDIO37_AUTH_URL`, `STUDIO37_PASSWORD`): put them under `environment:` in the launcher's `docker-compose.yml`, or in `.env` when running from source, then run the start file again (from source: `docker compose up -d`) so the container is recreated with them. Credentials saved in Settings → Keys apply at once.

### Redirect URIs

Each developer app needs the address the platform sends the browser back to after sign-in. It is `STUDIO37_PUBLIC_URL` (default `http://localhost:8080`) followed by:

| Platform | Path | Default full URI |
| --- | --- | --- |
| Facebook + Instagram | `/api/oauth/meta/callback` | `http://localhost:8080/api/oauth/meta/callback` |
| YouTube | `/api/oauth/google/callback` | `http://localhost:8080/api/oauth/google/callback` |
| TikTok | `/api/oauth/tiktok/callback` | `http://localhost:8080/api/oauth/tiktok/callback` |
| LinkedIn | `/api/oauth/linkedin/callback` | `http://localhost:8080/api/oauth/linkedin/callback` |
| X | `/api/oauth/x/callback` | `http://127.0.0.1:8080/api/oauth/x/callback` (X refuses `localhost`) |
| Pinterest | `/api/oauth/pinterest/callback` | `http://localhost:8080/api/oauth/pinterest/callback` |

On a server, `STUDIO37_PUBLIC_URL` is `https://your-domain` (the cloud compose file sets it), so register `https://your-domain/api/oauth/<platform>/callback`. The URI must match exactly, including `http`/`https`, host, port and the absence of a trailing slash.

### Captions and posting

**Publish** on a project sends one final video to any mix of the company's accounts, now or at a scheduled time. Claude writes a caption for each platform from what the video shows and the company's knowledge base: X and Bluesky stay short (their posts are limited to 280 and 300 characters), LinkedIn reads professional, YouTube gets a title and description, and Pinterest gets a title and description. You can edit every caption before posting, or ask the editor agent for them in the project chat.

The **Posts** tab shows each upload's status, a link to the live post, and the platform's own error if one fails, with **Retry**. Scheduled posts go out only while 37Studio is running; a post more than 12 hours overdue is marked missed instead of going out late. Tokens live in `37studio.db` in the data folder and are only sent to the platform they belong to.

## Facebook Pages + Instagram

**Connect Facebook & Instagram** signs in with Facebook Login for Business and lists the Pages you manage, plus the Instagram Business or Creator account linked to each.

### Your own Meta app (recommended)

Company → Social → **Use your own Meta app** walks through this inside the app. Or:

1. At [developers.facebook.com](https://developers.facebook.com/apps) create a **Business** app and add **Facebook Login for Business**. Add the redirect URI above.
2. Create a Login configuration with a *User access token* and these permissions: `pages_show_list`, `pages_read_engagement`, `pages_manage_posts`, `business_management`, `instagram_basic`, `instagram_content_publish` (plus the [inbox permissions](#inbox-messenger-instagram-and-comments) if you want the Inbox). Put its ID in `META_CONFIG_ID` (Instagram's video upload requires Facebook Login for Business), and the app ID and secret in `META_APP_ID` and `META_APP_SECRET`, in Settings → Keys or `.env`. The ID only takes effect together with its secret, and agents never get the secret.
3. Leave the app in **Development** mode and give your team a role on it (Admin, Developer or Tester): no App Review is needed for people with a role. Each company gives your business portfolio access to its Page (or makes you a Page admin). Its Instagram must be a Business or Creator account linked to that Page.

Vertical videos of 3–90 seconds post as Reels; anything else posts as a regular Page video. Meta allows 30 API-published Reels per Page per 24 hours.

### The shared 37Studio Meta app

Without your own app, logins use Shaitrish's 37Studio app (its ID and login configuration are built in, `internal/social/social.go`). Copies don't carry its secret; the relay at `STUDIO37_AUTH_URL` finishes the login. Until Meta approves that app in App Review, only people the maintainers add as testers can sign in with it. It is registered for `http://localhost:8080` only, so on a server use your own Meta app. The App Review submission pack is [meta-app-review.md](meta-app-review.md).

## YouTube

1. In [console.cloud.google.com](https://console.cloud.google.com) create a project and enable **YouTube Data API v3**.
2. OAuth consent screen: External, with the `youtube.upload` and `youtube.readonly` scopes. Set it to **In production**: in Testing, Google expires the login after 7 days. Unverified apps show a warning screen during sign-in; continue past it.
3. Credentials → OAuth client ID → *Web application*, with the redirect URI above. Put the ID and secret in `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` (Settings → Keys or `.env`).
4. Company → Social → **Connect YouTube** and pick the channel.

Until the project passes YouTube's free API audit, YouTube keeps every upload **private**. Apply through the *YouTube API Services audit and quota extension* form. The default daily quota also allows only a handful of uploads.

## TikTok

1. At [developers.tiktok.com](https://developers.tiktok.com) create an app with the **Desktop** platform (the only kind TikTok lets redirect to `http://localhost`; 37Studio uses PKCE for it) and the redirect URI above. On a server with a real HTTPS domain, use the **Web** platform with the `https://` redirect instead.
2. Add **Login Kit** and **Content Posting API** (turn on Direct Post) with the scopes `user.info.basic`, `video.publish`, `video.upload`. Put the client key and secret in `TIKTOK_CLIENT_KEY` and `TIKTOK_CLIENT_SECRET` (Settings → Keys or `.env`).
3. Company → Social → **Connect TikTok**.

Until TikTok reviews the app, use its Sandbox and add each company's TikTok account as a target user. Until TikTok audits the app, direct posts must be **Only me**; or choose **Send to TikTok drafts** in the Publish dialog and the account owner taps Post in the TikTok app. Each company signs in with its own TikTok account, since TikTok has no agency access.

## LinkedIn

Posts videos to the profile of the member who signs in, and, with LinkedIn's approval, to the company Pages they administer.

1. At [linkedin.com/developers](https://www.linkedin.com/developers/apps) create an app. LinkedIn asks for a LinkedIn Page to associate it with.
2. Under **Products**, add **Sign In with LinkedIn using OpenID Connect** and **Share on LinkedIn**. Together they grant the scopes 37Studio asks for: `openid profile w_member_social`.
3. Under **Auth**, add the redirect URI above, and copy the Client ID and Client Secret into `LINKEDIN_CLIENT_ID` and `LINKEDIN_CLIENT_SECRET` (Settings → Keys or `.env`).
4. Company → Social → **Connect LinkedIn**.

**Company Pages** need LinkedIn's **Community Management API**, which LinkedIn approves per app. Once it's approved, set the environment variable `LINKEDIN_PAGES=1`, restart, and connect again: the Pages you administer appear next to your profile.

LinkedIn tokens last 60 days. After that, posts to the account fail with a sign-in error; click **Connect LinkedIn** again to renew it.

## X

1. At [developer.x.com](https://developer.x.com) create a project and an app inside it.
2. In the app's **User authentication settings**, turn on **OAuth 2.0**, choose the type **Web App, Automated App or Bot** (a confidential client), set the app permissions to **Read and write**, and add the redirect URI above as a callback URI. X also asks for a website URL; any page about you works.
3. Under **Keys and tokens**, copy the OAuth 2.0 **Client ID** and **Client Secret** into `X_CLIENT_ID` and `X_CLIENT_SECRET` (Settings → Keys or `.env`).
4. Company → Social → **Connect X**. 37Studio signs in with OAuth 2.0 and PKCE and asks for `tweet.read tweet.write users.read media.write offline.access`.

Posting needs an X API access level that allows creating posts and uploading media; check what your developer account's plan includes before connecting.

**On your own computer** X doesn't accept `localhost` callback URLs, so 37Studio uses `127.0.0.1` for X only: register `http://127.0.0.1:8080/api/oauth/x/callback`. You keep opening 37Studio at `http://localhost:8080` as usual; only the sign-in passes through `127.0.0.1`. On a server, register `https://<your domain>/api/oauth/x/callback`.

## Pinterest

Posts video Pins to the boards you choose. Each board is an account in 37Studio, so a company can post to one board or several.

1. At [developers.pinterest.com](https://developers.pinterest.com) create an app (Pinterest asks for a business account) and add the redirect URI above.
2. Copy the app ID and secret into `PINTEREST_APP_ID` and `PINTEREST_APP_SECRET` (Settings → Keys or `.env`). 37Studio asks for `boards:read pins:read pins:write user_accounts:read`.
3. New apps get **Trial** access, which only allows creating Pins through Pinterest's sandbox API. Until Pinterest grants **Standard** access, set the environment variable `PINTEREST_API=https://api-sandbox.pinterest.com` and restart. Remove it once Standard access is granted.
4. Company → Social → **Connect Pinterest**, then tick the boards that belong to this company.

Each Pin gets a title and a description from the captions.

## Bluesky

No developer app needed.

1. In Bluesky, open **Settings → Privacy and security → App passwords** and create one for 37Studio. An app password can post but can't change the account's password or delete it.
2. Company → Social → **Connect Bluesky**, and enter the handle (for example `yourshop.bsky.social`) and the app password. For an account on a self-hosted PDS, also enter its host.

## Mastodon

No developer app needed.

1. On your Mastodon server, open **Preferences → Development → New application**. Give it a name and the scopes `read:accounts`, `write:media` and `write:statuses`, and save. Open it and copy **Your access token**.
2. Company → Social → **Connect Mastodon**, and enter the server URL (for example `https://mastodon.social`) and the token.

## Inbox: Messenger, Instagram and comments

**Inbox** in the sidebar shows Messenger, Instagram and WhatsApp messages, plus Facebook and Instagram comments, for every connected account, newest first, with a "needs reply" filter. The other platforms are for publishing only. Pick a message, click **Draft with AI** (Claude writes a reply from that company's `knowledge.md`, in the customer's language, and says a team member will follow up when the answer isn't there), edit, and **Send**. Nothing goes out without a person clicking Send unless you turn on auto-replies. A draft costs about $0.05 and counts toward the company's costs.

It reads the Graph API live while the page is open (refreshing every minute), so Pages and Instagram need no public address.

**Auto-replies** (Company → Social → Inbox replies): *Off*, *Claude drafts, you approve* (drafts wait in the Inbox marked "Draft ready", prefilled for one click), or *Claude replies on its own*. **How Claude should reply** on the same card holds the company's own rules (sign-off, language, what never to promise, when to hand over to a person). They are saved as `replies.md` next to `knowledge.md` and added to every draft; they override the default style but never the "facts only from the knowledge base" rule. A background loop checks every 2 minutes while 37Studio runs. Whatever arrived while the computer was off is answered when it's back, if the platform still allows it: messages within 24 hours, comments up to 2 days old. At most 20 replies go out per round, so a flood can't run up the Claude bill.

Meta setup, on top of the Facebook + Instagram steps:

1. In the Meta app, add the **Engage with customers on Messenger** use case (`pages_messaging`). In *Manage everything on your Page* add `pages_manage_engagement`, `pages_manage_metadata` and `pages_read_user_content`. In the Instagram use case, under *API setup with Facebook login*, add the messaging permission (`instagram_manage_messages`) and `instagram_manage_comments`.
2. Create a Login configuration with those six plus the six publishing permissions, put its ID in `META_CONFIG_ID`, and reconnect each company on its Social tab.
3. In the Instagram app: Settings → Messages and story replies → Message controls → Connected tools → **Allow access to messages**.
4. Until Meta grants Advanced Access through App Review and Business Verification, the inbox only shows messages from people with a role on the app or the Page. Test with a team account; real customers appear after approval.

Messenger and Instagram allow free-form replies within 24 hours of the customer's last message; comment replies have no time limit.

## WhatsApp

Inbox only: customers' WhatsApp messages appear in the Inbox; videos are not posted to WhatsApp.

Company → Social → **Connect WhatsApp**. Meta delivers WhatsApp messages only by webhook, and a copy on someone's computer has no public address, so a small Cloudflare Worker (`worker/`) receives them (`/webhooks/meta`, signed with the app secret) and holds them for 30 days in D1. Each copy pulls its own numbers' messages (`/whatsapp/messages`, authorised by its WhatsApp token) into `wa_messages`, the lasting history. Replies go straight to the Graph API. Setup:

1. In the Meta app, add the **Connect with customers through WhatsApp** use case, and point its WhatsApp webhook at your relay's `/webhooks/meta` with the `messages` field and the relay's `META_VERIFY_TOKEN`.
2. WhatsApp → API Setup gives a test number, its **Phone number ID** and **WhatsApp Business Account ID**. Add a real number there when ready; a number on the API can't be used in the WhatsApp Business phone app at the same time.
3. Business settings → Users → System users: add one, assign the WhatsApp account, and **Generate token** for your app with `whatsapp_business_messaging` and `whatsapp_business_management`. It doesn't expire.
4. Paste the three into Connect WhatsApp. 37Studio subscribes the app to that account's messages.

Numbers owned by other businesses, and keeping a number on the WhatsApp Business phone app alongside the API (coexistence), need Meta's Embedded Signup. That needs the app to be a Meta **Tech Provider**: Business Verification, then App Review with two screen recordings (sending a message, creating a template), then Access Verification.

## Run your own relay

`worker/` is one Cloudflare Worker with three jobs: it finishes Facebook logins for the shared Meta app, relays WhatsApp webhooks, and serves the product website in `worker/public/`. The public one at studio.shaitrish.com only serves the shared 37Studio Meta app. With your own Meta app you need your own relay only for WhatsApp; Cloudflare's free plan is enough.

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

## What can't be posted to

- **Threads.** Its API only accepts a video from a public URL. 37Studio keeps videos on your own computer and has no public address to hand it.
- **Facebook Groups and personal profiles.** Meta removed group posting for all apps in April 2024 and profile posting in 2018.
- **Reddit and Snapchat.** Neither has a usable public API for posting videos.
- **Images.** Publishing sends videos only. Thumbnails and square posts download as PNG.
