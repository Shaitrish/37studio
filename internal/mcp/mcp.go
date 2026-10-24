// Package mcp serves the app's API as MCP tools for Claude Code, Claude Desktop, Cursor and the in-app assistant.
package mcp

import (
	"bytes"
	"cmp"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/keys"
	"github.com/Shaitrish/37studio/internal/remote"
	"github.com/Shaitrish/37studio/internal/social"
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/textutil"
	"github.com/Shaitrish/37studio/internal/update"
)

const Protocol = "2025-06-18"

type mcpTool struct {
	Name, Desc   string
	Method, Path string            // Path may hold {arg} placeholders
	Query        map[string]string // argument -> query parameter
	Rename       map[string]string // argument -> JSON body field
	Props        map[string]any    // JSON Schema properties
	Required     []string
	Do           func(mux http.Handler, args map[string]any) (string, bool) // tools that are more than one API call
}

func pInt(d string) map[string]any { return map[string]any{"type": "integer", "description": d} }

func pStr(d string) map[string]any { return map[string]any{"type": "string", "description": d} }

func pBool(d string) map[string]any { return map[string]any{"type": "boolean", "description": d} }

func pEnum(d string, vals ...string) map[string]any {
	return map[string]any{"type": "string", "description": d, "enum": vals}
}

func pList(d, item string) map[string]any {
	return map[string]any{"type": "array", "description": d, "items": map[string]any{"type": item}}
}

func pObj(d string) map[string]any { return map[string]any{"type": "object", "description": d} }

var brand = pInt("The brand's id (list_companies)")

var project = pInt("The project's id")

var toTenant = map[string]string{"company_id": "tenant_id"}

var byTenant = map[string]string{"company_id": "tenant"}

var companyDoc = map[string]any{
	"name": pStr("Brand name"), "website": pStr("Website URL"), "contact": pStr("Contact person"), "email": pStr("Email"),
	"phone": pStr("Phone"), "bio": pStr("Short description of the business"), "language": pStr("Default video language code, like en or bn"),
	"brand_color": pStr("Main brand colour, #rrggbb"), "accent_color": pStr("Accent colour, #rrggbb"), "font": pStr("Brand font family"),
	"notes": pStr("Notes for the editor agent"), "knowledge": pStr("The whole knowledge.md: what the business does, offers, tone, facts"),
	"design": pStr("The whole design.md: the brand's design system"), "replies": pStr("The whole replies.md: rules for inbox replies"),
	"inbox_mode": pEnum("How inbox replies go out", "off", "draft", "send"),
}

var autopilotDoc = map[string]any{
	"name": pStr("Plan name"), "brief": pStr("What the videos are about"), "kind": pStr("Video kind id"), "aspect": pStr("Format, like 9:16"),
	"seconds": pInt("Length"), "language": pStr("Language code"), "quality": pStr("Quality tier id"), "days": pList("Weekdays to post, 0 = Sunday", "integer"),
	"at": pStr("Time to post, HH:MM"), "tz": pStr("IANA time zone, like Asia/Dhaka"), "accounts": pList("Social account ids to post to", "integer"),
	"approval": pBool("Wait for one-click approval before posting (default true)"), "enabled": pBool("On or off"),
}

var Tools = []mcpTool{
	{Name: "get_options", Desc: "Video kinds (presets) with their default format and length, quality tiers, and which AI keys are set. Read this before creating videos.",
		Method: "GET", Path: "/api/meta"},

	// brands
	{Name: "list_companies", Desc: "All brands with their project count and AI spend in USD.", Method: "GET", Path: "/api/tenants"},
	{Name: "get_company", Desc: "One brand: profile, knowledge base, design system, reply rules, brand library files with captions, and its projects.",
		Method: "GET", Path: "/api/tenants/{company_id}", Props: map[string]any{"company_id": brand}, Required: []string{"company_id"}},
	{Name: "create_company_from_website", Desc: "Add a brand by scanning its website: the brand library, design system and knowledge base fill in over 5-10 minutes (watch scan_status with get_company).",
		Method: "POST", Path: "/api/tenants/scan", Props: map[string]any{"url": pStr("The brand's website"), "notes": pStr("Anything the scan should know")}, Required: []string{"url"}},
	{Name: "create_company", Desc: "Add a brand by hand.", Method: "POST", Path: "/api/tenants", Props: companyDoc, Required: []string{"name"}},
	{Name: "update_company", Desc: "Change a brand's profile, knowledge base (knowledge), design system (design), reply rules (replies) or inbox mode. Only the fields you pass change; text documents are replaced whole.",
		Method: "PUT", Path: "/api/tenants/{company_id}", Props: merge(companyDoc, map[string]any{"company_id": brand}), Required: []string{"company_id"}},
	{Name: "rescan_company", Desc: "Scan the brand's website again to refresh its brand library and knowledge base.",
		Method: "POST", Path: "/api/tenants/{company_id}/scan", Props: map[string]any{"company_id": brand, "url": pStr("Website, if different"), "notes": pStr("Notes for the scan")}, Required: []string{"company_id"}},
	{Name: "delete_company", Desc: "Delete a brand and its brand library. Refused while it still has projects. Ask the user first.",
		Method: "DELETE", Path: "/api/tenants/{company_id}", Props: map[string]any{"company_id": brand}, Required: []string{"company_id"}},
	{Name: "add_company_asset", Desc: "Add a file to a brand's library, from a URL or as base64.",
		Props: map[string]any{"company_id": brand, "folder": pEnum("Library folder", files.AssetDirs...), "filename": pStr("File name with extension"),
			"url": pStr("Download the file from here"), "content_base64": pStr("Or the file itself, base64-encoded (small files)")},
		Required: []string{"company_id", "folder", "filename"}, Do: addCompanyAsset},
	{Name: "update_company_asset", Desc: "Describe a brand library file so the editor agent picks it well (caption, use, context, section, source), or move it to another folder.",
		Method: "PUT", Path: "/api/tenants/{company_id}/assets", Props: map[string]any{"company_id": brand, "path": pStr("Library path, like people/jane.png"),
			"caption": pStr("What is on screen: who, which product or feature"), "use": pStr("Good for: hero shot, testimonial, background…"), "context": pStr("Nearby heading or text"),
			"section": pStr("Where on the site it appeared"), "source": pStr("Page URL"), "folder": pEnum("Move it to this folder", files.AssetDirs...)}, Required: []string{"company_id", "path"}},
	{Name: "delete_company_asset", Desc: "Remove a file from a brand's library.", Method: "DELETE", Path: "/api/tenants/{company_id}/assets",
		Query: map[string]string{"path": "path"}, Props: map[string]any{"company_id": brand, "path": pStr("Library path, like logo/mark.svg")}, Required: []string{"company_id", "path"}},

	// campaigns
	{Name: "list_campaigns", Desc: "Campaigns, optionally for one brand.", Method: "GET", Path: "/api/campaigns", Query: byTenant, Props: map[string]any{"company_id": brand}},
	{Name: "create_campaign", Desc: "Group a brand's videos around one launch, offer or product. The brief reaches every video's agent.",
		Method: "POST", Path: "/api/campaigns", Rename: toTenant, Props: map[string]any{"company_id": brand, "name": pStr("Campaign name"), "brief": pStr("What the campaign is about")}, Required: []string{"company_id", "name"}},
	{Name: "get_campaign", Desc: "One campaign with its videos.", Method: "GET", Path: "/api/campaigns/{campaign_id}", Props: map[string]any{"campaign_id": pInt("The campaign's id")}, Required: []string{"campaign_id"}},
	{Name: "update_campaign", Desc: "Rename a campaign or change its brief.", Method: "PUT", Path: "/api/campaigns/{campaign_id}",
		Props: map[string]any{"campaign_id": pInt("The campaign's id"), "name": pStr("Name"), "brief": pStr("Brief")}, Required: []string{"campaign_id"}},
	{Name: "delete_campaign", Desc: "Delete a campaign; its videos stay.", Method: "DELETE", Path: "/api/campaigns/{campaign_id}",
		Props: map[string]any{"campaign_id": pInt("The campaign's id")}, Required: []string{"campaign_id"}},

	// projects and videos
	{Name: "list_projects", Desc: "Projects (videos, posts, thumbnails), newest first, optionally for one brand or campaign.", Method: "GET", Path: "/api/projects",
		Query: map[string]string{"company_id": "tenant", "campaign_id": "campaign"}, Props: map[string]any{"company_id": brand, "campaign_id": pInt("Campaign id")}},
	{Name: "get_project", Desc: "One project: brief, status (running, review, …), finished files in out/, drafts, posts, cost, and render or import progress.",
		Method: "GET", Path: "/api/projects/{project_id}", Props: map[string]any{"project_id": project}, Required: []string{"project_id"}},
	{Name: "create_video", Desc: "Start a new video (or post, or thumbnail) for a brand. The editor agent starts at once and works in the background for several minutes: follow it with get_project_messages and get_project. Footage is optional; with footage_url (a YouTube or Google Drive link) it downloads first.",
		Props: map[string]any{"company_id": brand, "title": pStr("Project title"), "kind": pStr("Video kind id from get_options, like hook-reel or explainer"),
			"brief": pStr("What the video should say and show"), "aspect": pEnum("Format", "9:16", "16:9", "1:1", "4:5"), "seconds": pInt("Length in seconds"),
			"language": pStr("Language code, like en or bn"), "quality": pStr("Quality tier id from get_options"), "focus": pStr("A page URL or feature the video is about"),
			"campaign_id": pInt("Campaign to add it to"), "template_id": pInt("Template to start from (list_templates)"), "footage_url": pStr("YouTube or Google Drive link to import"),
			"start": pBool("Start the agent now (default true)")},
		Required: []string{"company_id", "title", "brief"}, Do: createVideo},
	{Name: "update_project", Desc: "Change a project's title, brief, focus, format, length, language, quality or campaign. The agent sees the new brief on its next turn.",
		Method: "PUT", Path: "/api/projects/{project_id}", Props: map[string]any{"project_id": project, "title": pStr("Title"), "brief": pStr("Brief"), "focus": pStr("Page or feature"),
			"aspect": pStr("Format"), "seconds": pInt("Length"), "language": pStr("Language code"), "quality": pStr("Quality tier id"), "campaign_id": pInt("Campaign id")}, Required: []string{"project_id"}},
	{Name: "send_message", Desc: "Send the project's editor agent a message: feedback, changes, \"render the final\". If it is busy, the message waits for its turn.",
		Method: "POST", Path: "/api/projects/{project_id}/messages", Props: map[string]any{"project_id": project, "text": pStr("The message"), "quality": pStr("Quality tier for this turn (optional)")}, Required: []string{"project_id", "text"}},
	{Name: "get_project_messages", Desc: "The project's chat with its editor agent: messages, tool steps, results and errors. Pass after (the last id you saw) to get only new ones.",
		Method: "GET", Path: "/api/projects/{project_id}/events", Query: map[string]string{"after": "after"}, Props: map[string]any{"project_id": project, "after": pInt("Only events after this id")}, Required: []string{"project_id"}},
	{Name: "stop_agent", Desc: "Stop the project's editor agent or render right now.", Method: "POST", Path: "/api/projects/{project_id}/stop", Props: map[string]any{"project_id": project}, Required: []string{"project_id"}},
	{Name: "export_video", Desc: "Render the final video into out/ (runs in the background; get_project shows progress).",
		Method: "POST", Path: "/api/projects/{project_id}/render", Props: map[string]any{"project_id": project, "name": pStr("Output file name (optional)"), "quality": pEnum("draft renders faster at a lower frame rate; leave empty for the final", "draft")}, Required: []string{"project_id"}},
	{Name: "import_footage", Desc: "Download a YouTube or Google Drive video into the project's footage.", Method: "POST", Path: "/api/projects/{project_id}/import",
		Props: map[string]any{"project_id": project, "url": pStr("YouTube or Google Drive link"), "start": pBool("Start the agent when the download finishes")}, Required: []string{"project_id", "url"}},
	{Name: "cancel_import", Desc: "Stop a footage download that is still running.", Method: "DELETE", Path: "/api/projects/{project_id}/import", Props: map[string]any{"project_id": project}, Required: []string{"project_id"}},
	{Name: "set_transcription", Desc: "Switch a project's transcription off (skip: stops a running one) or back on (resume).", Method: "POST", Path: "/api/projects/{project_id}/transcript",
		Props: map[string]any{"project_id": project, "action": pEnum("What to do", "skip", "resume")}, Required: []string{"project_id", "action"}},
	{Name: "generate_media", Desc: "Generate an image or video clip (Gemini) or a voiceover, sound effect or music (ElevenLabs) into the project's drafts. Billed to those keys.",
		Method: "POST", Path: "/api/projects/{project_id}/generate", Props: map[string]any{"project_id": project, "kind": pEnum("What to make", "image", "video", "voice", "sfx", "music"),
			"prompt": pStr("What it should be, or the words to speak"), "aspect": pStr("Format, like 9:16"), "seconds": pInt("Length, for clips, sound and music")}, Required: []string{"project_id", "kind", "prompt"}},
	{Name: "delete_project_file", Desc: "Delete a file from a project, like an old draft or an unwanted upload.", Method: "DELETE", Path: "/api/projects/{project_id}/files",
		Query: map[string]string{"path": "path"}, Props: map[string]any{"project_id": project, "path": pStr("Path inside the project, like out/drafts/take1.mp4")}, Required: []string{"project_id", "path"}},
	{Name: "open_timeline", Desc: "Open the project's timeline editor (HyperFrames Studio) and get its port; the user edits there by hand at http://localhost:<port>.",
		Method: "POST", Path: "/api/projects/{project_id}/studio", Props: map[string]any{"project_id": project}, Required: []string{"project_id"}},
	{Name: "close_timeline", Desc: "Close the project's timeline editor.", Method: "DELETE", Path: "/api/projects/{project_id}/studio", Props: map[string]any{"project_id": project}, Required: []string{"project_id"}},
	{Name: "add_project_file", Desc: "Add footage or any other file to a project's input folder, from a URL or as base64.",
		Props:    map[string]any{"project_id": project, "filename": pStr("File name with extension"), "url": pStr("Download the file from here"), "content_base64": pStr("Or the file itself, base64-encoded (small files)")},
		Required: []string{"project_id", "filename"}, Do: addProjectFile},
	{Name: "save_template", Desc: "Save a finished project's look as a template for future videos.", Method: "POST", Path: "/api/projects/{project_id}/template",
		Props: map[string]any{"project_id": project, "name": pStr("Template name"), "notes": pStr("When to use it"), "shared": pBool("Offer it to every brand")}, Required: []string{"project_id"}},
	{Name: "list_templates", Desc: "Saved templates, optionally for one brand.", Method: "GET", Path: "/api/templates", Query: byTenant, Props: map[string]any{"company_id": brand}},
	{Name: "delete_template", Desc: "Delete a template; videos made from it stay.", Method: "DELETE", Path: "/api/templates/{template_id}", Props: map[string]any{"template_id": pInt("Template id")}, Required: []string{"template_id"}},
	{Name: "list_library", Desc: "Every finished video and image across projects.", Method: "GET", Path: "/api/library"},
	{Name: "delete_project", Desc: "Delete a project and all its files. Ask the user first.", Method: "DELETE", Path: "/api/projects/{project_id}", Props: map[string]any{"project_id": project}, Required: []string{"project_id"}},

	// publishing and autopilot
	{Name: "list_social_accounts", Desc: "A brand's connected social accounts (ids to publish to), which platforms are set up, and its inbox mode.",
		Method: "GET", Path: "/api/tenants/{company_id}/social", Props: map[string]any{"company_id": brand}, Required: []string{"company_id"}},
	{Name: "write_captions", Desc: "Have Claude write a caption for every platform from the finished video and the brand's knowledge base.",
		Method: "POST", Path: "/api/projects/{project_id}/captions", Props: map[string]any{"project_id": project}, Required: []string{"project_id"}},
	{Name: "publish_video", Desc: "Post a finished video to some of the brand's accounts, now or at a time. Confirm the accounts, captions and time with the user first.",
		Method: "POST", Path: "/api/projects/{project_id}/posts",
		Props: map[string]any{"project_id": project, "file": pStr("The video, like out/final.mp4"), "accounts": pList("Social account ids (list_social_accounts)", "integer"),
			"captions": pObj("Caption per platform: facebook, instagram, tiktok, youtube, youtube_title, linkedin, x, pinterest, pinterest_title, bluesky, mastodon"),
			"privacy":  pObj("Optional: {youtube: public|unlisted|private, tiktok: PUBLIC_TO_EVERYONE|SELF_ONLY|INBOX|…}"), "at": pStr("RFC 3339 time to post at; empty posts now")},
		Required: []string{"project_id", "file", "accounts", "captions"}},
	{Name: "list_autopilots", Desc: "A brand's Autopilot content plans.", Method: "GET", Path: "/api/autopilots", Query: byTenant, Props: map[string]any{"company_id": brand}, Required: []string{"company_id"}},
	{Name: "create_autopilot", Desc: "Create an Autopilot plan: videos made on a schedule and posted after one-click approval.",
		Method: "POST", Path: "/api/autopilots", Rename: toTenant, Props: merge(autopilotDoc, map[string]any{"company_id": brand}), Required: []string{"company_id", "brief"}},
	{Name: "update_autopilot", Desc: "Change an Autopilot plan, or turn it on or off. Only the fields you pass change.",
		Method: "PUT", Path: "/api/autopilots/{autopilot_id}", Props: merge(autopilotDoc, map[string]any{"autopilot_id": pInt("Plan id")}), Required: []string{"autopilot_id"}},
	{Name: "delete_autopilot", Desc: "Delete an Autopilot plan; its videos stay.", Method: "DELETE", Path: "/api/autopilots/{autopilot_id}", Props: map[string]any{"autopilot_id": pInt("Plan id")}, Required: []string{"autopilot_id"}},
	{Name: "run_autopilot", Desc: "Make the plan's next video now.", Method: "POST", Path: "/api/autopilots/{autopilot_id}/run", Props: map[string]any{"autopilot_id": pInt("Plan id")}, Required: []string{"autopilot_id"}},
	{Name: "skip_video", Desc: "Skip an Autopilot video instead of posting it.", Method: "POST", Path: "/api/projects/{project_id}/skip", Props: map[string]any{"project_id": project}, Required: []string{"project_id"}},
	{Name: "retry_post", Desc: "Try a failed post again now.", Method: "POST", Path: "/api/posts/{post_id}/retry", Props: map[string]any{"post_id": pInt("Post id from get_project")}, Required: []string{"post_id"}},
	{Name: "delete_post", Desc: "Cancel a scheduled post, or remove a finished one from the list (it stays on the platform).", Method: "DELETE", Path: "/api/posts/{post_id}", Props: map[string]any{"post_id": pInt("Post id")}, Required: []string{"post_id"}},
	{Name: "approve_video", Desc: "Approve an Autopilot video so its posts go out. Only when the user says so.", Method: "POST", Path: "/api/projects/{project_id}/approve", Props: map[string]any{"project_id": project}, Required: []string{"project_id"}},

	// inbox and costs
	{Name: "list_inbox", Desc: "Recent Messenger, Instagram and WhatsApp messages and comments, optionally for one brand.", Method: "GET", Path: "/api/inbox", Query: byTenant, Props: map[string]any{"company_id": brand}},
	{Name: "get_inbox_thread", Desc: "The whole conversation or comment thread behind an inbox item.", Method: "GET", Path: "/api/inbox/thread",
		Query: map[string]string{"account": "account", "kind": "kind", "id": "id"}, Props: map[string]any{"account": pInt("Account id"), "kind": pEnum("Item kind", "dm", "comment"), "id": pStr("Item id")}, Required: []string{"account", "kind", "id"}},
	{Name: "draft_reply", Desc: "Have Claude draft a reply to an inbox item from the brand's knowledge base.", Method: "POST", Path: "/api/inbox/draft",
		Props: map[string]any{"account": pInt("Account id from the inbox item"), "kind": pEnum("Item kind", "dm", "comment"), "id": pStr("Item id"), "context": pStr("Extra instructions")}, Required: []string{"account", "kind", "id"}},
	{Name: "send_reply", Desc: "Send a reply to a customer. Only with text the user has approved.", Method: "POST", Path: "/api/inbox/reply",
		Props: map[string]any{"account": pInt("Account id"), "kind": pEnum("Item kind", "dm", "comment"), "id": pStr("Item id"), "to": pStr("Recipient id, for messages"), "text": pStr("The reply")}, Required: []string{"account", "kind", "id", "text"}},
	{Name: "get_costs", Desc: "AI spend in USD by provider, brand and month.", Method: "GET", Path: "/api/costs"},

	// setup: settings, keys and account connections
	{Name: "get_settings", Desc: "App settings: AI models and unit prices, per-turn budget, voices, rendering, display currency and exchange rate.", Method: "GET", Path: "/api/settings"},
	{Name: "update_settings", Desc: "Change app settings. Pass only the ones to change, with the names get_settings returns.",
		Props: map[string]any{"settings": pObj("Setting name -> new value")}, Required: []string{"settings"},
		Do: func(mux http.Handler, a map[string]any) (string, bool) {
			return mcpDo(mux, "PUT", "/api/settings", a["settings"])
		}},
	{Name: "list_voices", Desc: "ElevenLabs voices to choose for voiceovers (set one with update_settings).", Method: "GET", Path: "/api/voices"},
	{Name: "check_update", Desc: "This copy's version and whether a newer one is out.", Method: "GET", Path: "/api/update"},
	{Name: "list_keys", Desc: "Which API keys and platform app credentials are set (only the last 4 characters are ever shown).", Method: "GET", Path: "/api/keys"},
	{Name: "set_keys", Desc: "Save API keys or platform app credentials, encrypted on this computer; an empty value removes one. Names: " + keyList() + ". Only with values the user gave you.",
		Props: map[string]any{"keys": pObj("Key name -> value")}, Required: []string{"keys"},
		Do: func(mux http.Handler, a map[string]any) (string, bool) {
			return mcpDo(mux, "PUT", "/api/keys", a["keys"])
		}},
	{Name: "connect_link", Desc: "A sign-in link that connects a brand's accounts on a platform. The user opens it in their browser, signs in on the platform and ticks the accounts; the app's own credentials for that platform must be set first (set_keys).",
		Props:    map[string]any{"company_id": brand, "platform": pEnum("meta = Facebook & Instagram, google = YouTube", "meta", "google", "tiktok", "linkedin", "x", "pinterest")},
		Required: []string{"company_id", "platform"}, Do: connectLink},
	{Name: "connect_bluesky", Desc: "Connect a Bluesky account to a brand with its handle and an app password (bsky.app → Settings → Privacy and security → App passwords).",
		Method: "POST", Path: "/api/tenants/{company_id}/bluesky", Props: map[string]any{"company_id": brand, "handle": pStr("Like name.bsky.social"), "password": pStr("An app password"), "service": pStr("Only for a self-hosted PDS")}, Required: []string{"company_id", "handle", "password"}},
	{Name: "connect_mastodon", Desc: "Connect a Mastodon account to a brand with its server and an access token (Preferences → Development → New application, scopes read:accounts write:media write:statuses).",
		Method: "POST", Path: "/api/tenants/{company_id}/mastodon", Props: map[string]any{"company_id": brand, "server": pStr("Like mastodon.social"), "token": pStr("Access token")}, Required: []string{"company_id", "server", "token"}},
	{Name: "connect_whatsapp", Desc: "Connect a WhatsApp Business API number to a brand's inbox, with the values from Meta's WhatsApp → API Setup page and a system-user token.",
		Method: "POST", Path: "/api/tenants/{company_id}/whatsapp", Props: map[string]any{"company_id": brand, "phone_id": pStr("Phone number ID"), "waba_id": pStr("WhatsApp Business Account ID"), "token": pStr("Access token")}, Required: []string{"company_id", "phone_id", "waba_id", "token"}},
	{Name: "disconnect_account", Desc: "Disconnect a social account from its brand. Posts scheduled to it will fail. Ask the user first.",
		Method: "DELETE", Path: "/api/social/{account_id}", Props: map[string]any{"account_id": pInt("Account id from list_social_accounts")}, Required: []string{"account_id"}},
}

func keyList() string {
	var names []string
	for _, k := range keys.Names {
		names = append(names, k.Name)
	}
	return strings.Join(names, ", ")
}

// connectLink can't sign in for the user (the platforms want a person in a browser), so it hands them the link.
func connectLink(mux http.Handler, a map[string]any) (string, bool) {
	p := store.Str(a["platform"])
	if !social.ProviderReady(p) {
		return p + " isn't set up in this copy yet: save its app credentials with set_keys first (docs/platforms.md has the steps for each platform).", true
	}
	return fmt.Sprintf("Open this link in the browser where 37Studio is open, sign in, then tick the accounts that belong to the brand:\n%s/api/oauth/%s/start?tenant=%v&return=%s",
		config.PublicURL, p, a["company_id"], url.QueryEscape(config.PublicURL+"/")), false
}

func merge(a, b map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

const mcpInstructions = `37Studio makes branded videos for brands with an AI editor agent and posts them to social media. Brands hold a brand library, design system and knowledge base; projects are videos built by the agent in the background.
Video work is slow: after create_video or send_message, check get_project_messages and get_project every minute or so until the status is "review", then look at the files in out/. Ask the user before deleting anything, publishing, approving Autopilot videos or sending inbox replies.`

// Handler answers JSON-RPC over POST. Browsers can reach localhost too, so a request from another site's page is
// refused (the Origin check the MCP spec asks for, against DNS rebinding).
func Handler(mux http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !localOrigin(r.Header.Get("Origin")) {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
		if r.Method != "POST" {
			w.Header().Set("Allow", "POST")
			http.Error(w, "this MCP server answers POST only (no event stream)", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			mcpReply(w, nil, nil, &mcpError{-32700, "parse error"})
			return
		}
		if req.ID == nil { // a notification: nothing to answer
			w.WriteHeader(http.StatusAccepted)
			return
		}
		switch req.Method {
		case "initialize":
			mcpReply(w, req.ID, map[string]any{"protocolVersion": Protocol, "capabilities": map[string]any{"tools": map[string]any{}},
				"serverInfo": map[string]any{"name": "37studio", "version": update.Version}, "instructions": mcpInstructions}, nil)
		case "ping":
			mcpReply(w, req.ID, map[string]any{}, nil)
		case "tools/list":
			tools := []map[string]any{}
			for _, t := range Tools {
				schema := map[string]any{"type": "object", "properties": orEmpty(t.Props)}
				if len(t.Required) > 0 {
					schema["required"] = t.Required
				}
				tools = append(tools, map[string]any{"name": t.Name, "description": t.Desc, "inputSchema": schema})
			}
			mcpReply(w, req.ID, map[string]any{"tools": tools}, nil)
		case "tools/call":
			var p struct {
				Name string         `json:"name"`
				Args map[string]any `json:"arguments"`
			}
			json.Unmarshal(req.Params, &p)
			t := findTool(p.Name)
			if t == nil {
				mcpReply(w, req.ID, nil, &mcpError{-32602, "unknown tool " + p.Name})
				return
			}
			text, isErr := t.call(mux, orEmpty(p.Args))
			mcpReply(w, req.ID, map[string]any{"content": []map[string]any{{"type": "text", "text": textutil.Head(text, 60000)}}, "isError": isErr}, nil)
		default:
			mcpReply(w, req.ID, nil, &mcpError{-32601, "method not found: " + req.Method})
		}
	}
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func mcpReply(w http.ResponseWriter, id json.RawMessage, result any, e *mcpError) {
	w.Header().Set("Content-Type", "application/json")
	out := map[string]any{"jsonrpc": "2.0", "id": id}
	if e != nil {
		out["error"] = e
	} else {
		out["result"] = result
	}
	json.NewEncoder(w).Encode(out)
}

func orEmpty(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

func findTool(name string) *mcpTool {
	for i := range Tools {
		if Tools[i].Name == name {
			return &Tools[i]
		}
	}
	return nil
}

// localOrigin: MCP clients send no Origin; a browser page may only be this app itself.
func localOrigin(o string) bool {
	if o == "" {
		return true
	}
	u, err := url.Parse(o)
	if err != nil {
		return false
	}
	pu, _ := url.Parse(config.PublicURL)
	h := u.Hostname()
	return h == "localhost" || h == "127.0.0.1" || h == "::1" || (pu != nil && h == pu.Hostname())
}

// call maps the arguments onto the tool's API request: {placeholders} in the path, query parameters, and the rest
// as the JSON body.
func (t mcpTool) call(mux http.Handler, args map[string]any) (string, bool) {
	if t.Do != nil {
		return t.Do(mux, args)
	}
	path, q, b := t.Path, url.Values{}, map[string]any{}
	for k, v := range args {
		switch {
		case strings.Contains(path, "{"+k+"}"):
			path = strings.ReplaceAll(path, "{"+k+"}", url.PathEscape(fmt.Sprint(v)))
		case t.Query[k] != "":
			q.Set(t.Query[k], fmt.Sprint(v))
		default:
			b[cmp.Or(t.Rename[k], k)] = v
		}
	}
	if strings.Contains(path, "{") {
		return "missing a required id in " + path, true
	}
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	return mcpDo(mux, t.Method, path, b)
}

// mcpDo runs one request through the app's router, as if the UI had sent it.
func mcpDo(mux http.Handler, method, target string, body any) (string, bool) {
	var rd io.Reader
	if method != "GET" && body != nil {
		raw, _ := json.Marshal(body)
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, target, rd)
	req.Header.Set("Content-Type", "application/json")
	return mcpServe(mux, req)
}

func mcpServe(mux http.Handler, req *http.Request) (string, bool) {
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	out := strings.TrimSpace(rec.Body.String())
	if out == "" || out == "null" {
		out = http.StatusText(rec.Code)
	}
	return out, rec.Code >= 400
}

func createVideo(mux http.Handler, a map[string]any) (string, bool) {
	b := map[string]any{"tenant_id": a["company_id"]}
	for _, k := range []string{"title", "kind", "brief", "aspect", "seconds", "language", "quality", "focus", "campaign_id", "template_id"} {
		if v, ok := a[k]; ok {
			b[k] = v
		}
	}
	out, isErr := mcpDo(mux, "POST", "/api/projects", b)
	if isErr {
		return out, true
	}
	var p struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal([]byte(out), &p)
	switch link := store.Str(a["footage_url"]); {
	case link != "":
		if msg, bad := mcpDo(mux, "POST", fmt.Sprintf("/api/projects/%d/import", p.ID), map[string]any{"url": link, "start": a["start"] != false}); bad {
			return fmt.Sprintf("Project %d was created, but the footage import failed: %s", p.ID, msg), true
		}
		return fmt.Sprintf("Project %d created; the footage is downloading and the agent starts when it's done. Follow it with get_project.", p.ID), false
	case a["start"] != false:
		if msg, bad := mcpDo(mux, "POST", fmt.Sprintf("/api/projects/%d/messages", p.ID), map[string]any{"text": ""}); bad {
			return fmt.Sprintf("Project %d was created, but the agent didn't start: %s", p.ID, msg), true
		}
		return fmt.Sprintf("Project %d created and the editor agent is working on it. Follow it with get_project_messages.", p.ID), false
	}
	return out, false
}

func addCompanyAsset(mux http.Handler, a map[string]any) (string, bool) {
	return mcpUpload(mux, fmt.Sprintf("/api/tenants/%v/assets?folder=%s", a["company_id"], url.QueryEscape(store.Str(a["folder"]))), a)
}

func addProjectFile(mux http.Handler, a map[string]any) (string, bool) {
	return mcpUpload(mux, fmt.Sprintf("/api/projects/%v/inputs", a["project_id"]), a)
}

// mcpUpload streams a file from a URL (or decodes a small base64 one) into the same multipart upload the UI uses.
func mcpUpload(mux http.Handler, target string, a map[string]any) (string, bool) {
	var src io.Reader
	switch {
	case store.Str(a["url"]) != "":
		res, err := remote.Uploads.Get(store.Str(a["url"]))
		if err != nil {
			return "download failed: " + err.Error(), true
		}
		defer res.Body.Close()
		if res.StatusCode >= 300 {
			return "download failed: " + res.Status, true
		}
		src = res.Body
	case store.Str(a["content_base64"]) != "":
		raw, err := base64.StdEncoding.DecodeString(store.Str(a["content_base64"]))
		if err != nil {
			return "content_base64 isn't valid base64", true
		}
		src = bytes.NewReader(raw)
	default:
		return "pass url or content_base64", true
	}
	req := remote.Multipart("http://37studio"+target, nil, "file", files.SafeName(store.Str(a["filename"])), src, "")
	return mcpServe(mux, req)
}
