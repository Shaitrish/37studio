# Meta App Review for 37Studio

What to fill in and submit so customers (not just app testers) can connect their Pages, publish, and use the Inbox.
App: **37Studio**, ID 2057436148296558, business portfolio **Shaitrish**.

## 0. Before you submit

1. **Business Verification** of the Shaitrish portfolio: Business settings → Security centre → Start verification. Use the
   trade licence (TRAD/DNCC/022960/2025), the business address and a phone number or email on the shaitrish.com domain.
   Meta requires a verified business for Advanced Access to the messaging permissions ("Your app must be owned by a verified
   business"). Usually a few working days.
2. **Basic settings** (App settings → Basic):

   | Field | Value |
   | --- | --- |
   | App domains | `studio.shaitrish.com` |
   | Contact email | `jahangir@shaitrish.com` |
   | Privacy policy URL | `https://studio.shaitrish.com/privacy` |
   | Terms of Service URL | `https://studio.shaitrish.com/terms` |
   | User data deletion | Data deletion instructions URL: `https://studio.shaitrish.com/privacy#delete` |
   | App icon | `brand/37studio-icon-1024.png` (1024 × 1024) |
   | Category | Business and pages |

3. **Platform**: Add platform → Website → `https://studio.shaitrish.com`.
4. **Redirect before going Live.** The app sends Facebook back to `http://localhost:8080/api/oauth/meta/callback`. That works
   in development mode. With the app Live and Strict Mode on, Meta only accepts redirect URIs saved in Facebook Login for
   Business → Settings, and that field didn't save `localhost` when we tried. Before switching to Live, route the redirect
   through `https://studio.shaitrish.com/oauth/meta/callback` (the Worker forwards the code to the local copy). This is a small change in `internal/social/social.go` and
   `worker/src.js`.

## 1. Permissions to request (Advanced Access)

For each one, paste the description, then attach the screencast named in brackets.

**pages_show_list** [video A]
> 37Studio is a desktop app that a business runs on its own computer to make videos and publish them to its Facebook Pages.
> After the person logs in with Facebook Login for Business, we list the Pages they manage so they can choose which Pages
> belong to each company in the app. Without this permission they cannot pick a Page to publish to.

**business_management** [video A]
> Many of our customers' Pages are owned by a business portfolio. We use this permission only to list the Pages and linked
> Instagram accounts the person manages through their business portfolio, so they can connect them in 37Studio. We don't
> change business settings, assets or people.

**pages_read_engagement** [video A, B]
> We read basic Page information (name, picture) to show the connected Page, and read the Page's published posts so the
> person can see comments on them in 37Studio's Inbox and know which post each comment belongs to.

**pages_manage_posts** [video A]
> The person makes a video in 37Studio and clicks Publish (or approves a scheduled post). We upload that video as a Reel to
> the Page they selected, with the caption they reviewed. Nothing is posted without that action.

**instagram_basic** [video A, B]
> We read the Instagram professional account linked to the person's Page (username, picture, recent media) to show it as a
> publishing target and to list comments on its posts in the Inbox.

**instagram_content_publish** [video A]
> When the person clicks Publish and selects their Instagram account, we upload the video they made in 37Studio as a Reel
> with the caption they reviewed.

**pages_messaging** [video B]
> 37Studio's Inbox shows the Page's Messenger conversations so a business can answer customers in one place. The person
> reads a message, optionally asks Claude (an AI model, under the business's own account) to draft a reply from the
> business's own information, edits it, and clicks Send. Replies are only sent within Messenger's 24-hour standard
> messaging window, in response to the customer.

**pages_manage_metadata** [video B]
> Required by the Conversations API to read the Page's Messenger and Instagram conversations in the Inbox.

**pages_read_user_content** [video B]
> We read comments that people leave on the Page's posts, so the business can see and answer them in the Inbox.

**pages_manage_engagement** [video B]
> When the business answers a comment in the Inbox and clicks Send, we post that reply under the comment as the Page.

**instagram_manage_messages** [video B]
> The Inbox shows Instagram direct messages sent to the business's Instagram professional account, and sends the reply the
> person reviews, within Instagram's 24-hour messaging window.

**instagram_manage_comments** [video B]
> The Inbox shows comments on the business's Instagram posts and posts the reply the person reviews under the comment.

## 2. Screencasts

Record at 1080p with the cursor visible, English interface, no music. Speak or caption each step. Use your own Pages and
a second Facebook/Instagram account as the "customer".

**Video A: connect and publish (about 3 minutes)**
1. Open 37Studio (show the app and that it's running on the computer).
2. Companies → a company → Social → **Connect Facebook & Instagram**.
3. Show the Facebook Login for Business dialog: pick the business and Pages, show the permissions screen, continue.
4. Back in 37Studio, tick the Page and Instagram account → Connect. Show them listed on the Social tab.
5. Open a finished project → **Publish** → select the Page and the Instagram account → show the captions → **Post**.
6. Show the Posts list turning to Posted, then open the Page and Instagram and show the Reel live.

**Video B: inbox (about 3 minutes)**
1. From the "customer" account: send the Page a Messenger message, comment on a Page post, send the Instagram account a
   DM, and comment on an Instagram post.
2. In 37Studio, open **Inbox** → Refresh. Show all four items.
3. Open the Messenger item → **Draft with AI** → edit the draft → **Send**. Show the reply arriving in Messenger.
4. Do the same for the Facebook comment, the Instagram DM and the Instagram comment, showing each reply on the platform.
5. Show the Social tab's **Inbox replies** setting: off, drafts to approve, or replies on their own, plus the reply rules.

## 3. Notes for the reviewer (paste into "App verification details")

> 37Studio is a desktop application (Windows, Mac, Linux) that runs on the customer's own computer; it is not a hosted
> website, so there is no login for you to use. The screencasts show the complete flows. If you'd like to try it, download
> it from https://studio.shaitrish.com (or see the source at https://github.com/Shaitrish/37studio), start it with the
> included launcher, and follow the steps in the videos. Facebook Login uses your own test Page.

## 4. Data handling questions

- **Data processors / third parties:** Yes, one. When the person asks for a drafted reply (or turns on auto-replies),
  the conversation text and the business's own information are sent to **Anthropic (Claude)**, under the business's own
  Anthropic account, to write the draft. Nothing else is shared; nothing is sold.
- **Where platform data is stored:** only on the customer's own computer, in the app's data folder. Incoming WhatsApp
  messages wait up to 30 days on our relay (studio.shaitrish.com, Cloudflare) until the customer's copy collects them.
- **Who can access it:** the customer. Shaitrish has no access to customers' installations.
- **Deletion:** https://studio.shaitrish.com/privacy#delete
- **Responsible entity:** Shaitrish, Shekhertek 12, Mohammadpur, Dhaka 1207, Bangladesh.

## 5. After approval

1. Publish → switch the app to **Live**.
2. People no longer need to be added as testers; anyone can connect with the shared app directly.
3. WhatsApp for customers' own numbers is a separate program: becoming a **Tech Provider** (Embedded Signup), with its own
   App Review for `whatsapp_business_management` and `whatsapp_business_messaging` and two screen recordings (sending a
   message, creating a template).
