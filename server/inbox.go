package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Inbox: Messenger and Instagram DMs plus Facebook and Instagram comments for every connected Page, read live from
// the Graph API, and WhatsApp chats (whatsapp.go), with Claude drafting replies from the company's knowledge base and
// an editor sending them.
// Read on demand while the Inbox page is open, and every two minutes for companies with auto-replies on (inboxLoop).

type inboxItem struct {
	ID       string `json:"id"`      // conversation or comment id
	Account  int64  `json:"account"` // social_accounts.id
	Tenant   int64  `json:"tenant"`
	Client   string `json:"client"`
	Platform string `json:"platform"` // facebook | instagram | whatsapp
	Page     string `json:"page"`
	Kind     string `json:"kind"` // dm | comment
	From     string `json:"from"`
	To       string `json:"to"` // who a DM reply goes to (PSID / IGSID / WhatsApp number)
	Text     string `json:"text"`
	Context  string `json:"context"` // the post a comment is on
	URL      string `json:"url"`
	Time     string `json:"time"`
	Pending  bool   `json:"pending"`         // the customer has the last word
	Draft    string `json:"draft,omitempty"` // Claude's reply waiting for approval (auto-replies in "draft" mode)
}

type inboxMsg struct {
	From string `json:"from"`
	Text string `json:"text"`
	Time string `json:"time"`
	Mine bool   `json:"mine"`
}

type gUser struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
}

func (u gUser) label() string { return or(u.Name, or(u.Username, "Customer")) }

// inboxAccount is a connected Page or Instagram account with what the inbox needs to tell "us" from "them".
type inboxAccount struct {
	id, tenant           int64
	client, platform     string
	accountID, name, tok string
}

func (a inboxAccount) mine(u gUser) bool {
	return u.ID == a.accountID || (u.Username != "" && "@"+u.Username == a.name) || (u.Name != "" && u.Name == a.name)
}

func inboxAccounts(where string, args ...any) ([]inboxAccount, error) {
	rs, err := rows(`select a.id, a.tenant_id, t.name as client, a.platform, a.account_id, a.name, a.token
	  from social_accounts a join tenants t on t.id=a.tenant_id where a.platform in ('facebook','instagram','whatsapp') `+where+` order by t.name, a.name`, args...)
	var out []inboxAccount
	for _, r := range rs {
		out = append(out, inboxAccount{r["id"].(int64), r["tenant_id"].(int64), str(r["client"]), str(r["platform"]), str(r["account_id"]), str(r["name"]), str(r["token"])})
	}
	return out, err
}

func listInbox(r *http.Request) (any, error) {
	where, args := "", []any{}
	if t := r.URL.Query().Get("tenant"); t != "" {
		where, args = "and a.tenant_id=?", append(args, t)
	}
	accts, err := inboxAccounts(where, args...)
	if err != nil {
		return nil, err
	}
	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		items = []inboxItem{}
		errs  = []map[string]string{}
	)
	for _, a := range accts {
		for _, kind := range []string{"dm", "comment"} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				got, err := fetchInbox(a, kind)
				mu.Lock()
				defer mu.Unlock()
				items = append(items, got...)
				if err != nil {
					errs = append(errs, map[string]string{"page": a.name, "platform": a.platform, "client": a.client, "kind": kind, "error": err.Error()})
				}
			}()
		}
	}
	wg.Wait()
	for i, it := range items {
		if it.Pending {
			db.QueryRow("select text from inbox_drafts where account_id=? and item=? and answers=? and status='ready'", it.Account, it.ID, it.Time).Scan(&items[i].Draft)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Time > items[j].Time })
	sort.SliceStable(errs, func(i, j int) bool { return errs[i]["page"]+errs[i]["kind"] < errs[j]["page"]+errs[j]["kind"] })
	return map[string]any{"items": items, "errors": errs}, nil
}

type gMsgs struct {
	Data []struct {
		Message string `json:"message"`
		From    gUser  `json:"from"`
		Created string `json:"created_time"`
	} `json:"data"`
}

func fetchInbox(a inboxAccount, kind string) ([]inboxItem, error) {
	if a.platform == "whatsapp" {
		if kind != "dm" {
			return nil, nil // WhatsApp has no comments
		}
		return waChats(a)
	}
	base := inboxItem{Account: a.id, Tenant: a.tenant, Client: a.client, Platform: a.platform, Page: a.name, Kind: kind}
	var out []inboxItem
	if kind == "dm" {
		var cs struct {
			Data []struct {
				ID           string `json:"id"`
				Updated      string `json:"updated_time"`
				Link         string `json:"link"`
				Participants struct {
					Data []gUser `json:"data"`
				} `json:"participants"`
				Messages gMsgs `json:"messages"`
			} `json:"data"`
		}
		platform := map[string]string{"facebook": "messenger", "instagram": "instagram"}[a.platform]
		// The Page token's "me" is the Page, which also owns its linked Instagram's conversations.
		if err := call(get(graph("/me/conversations", url.Values{"platform": {platform}, "limit": {"25"},
			"fields": {"id,updated_time,link,participants,messages.limit(1){message,from,created_time}"}}), "Bearer "+a.tok), &cs); err != nil {
			return nil, err
		}
		for _, c := range cs.Data {
			it := base
			it.ID, it.Time, it.URL = c.ID, c.Updated, c.Link
			if it.URL != "" && !strings.HasPrefix(it.URL, "http") {
				it.URL = "https://www.facebook.com" + it.URL
			}
			for _, p := range c.Participants.Data {
				if !a.mine(p) {
					it.From, it.To = p.label(), p.ID
				}
			}
			if len(c.Messages.Data) > 0 {
				m := c.Messages.Data[0]
				it.Text, it.Pending = m.Message, !a.mine(m.From)
			}
			out = append(out, it)
		}
		return out, nil
	}
	if a.platform == "facebook" {
		var ps struct {
			Data []struct {
				Message   string `json:"message"`
				Permalink string `json:"permalink_url"`
				Comments  struct {
					Data []struct {
						ID        string `json:"id"`
						Message   string `json:"message"`
						From      gUser  `json:"from"`
						Created   string `json:"created_time"`
						Permalink string `json:"permalink_url"`
						Replies   struct {
							Data []struct {
								From gUser `json:"from"`
							} `json:"data"`
						} `json:"comments"`
					} `json:"data"`
				} `json:"comments"`
			} `json:"data"`
		}
		if err := call(get(graph("/me/published_posts", url.Values{"limit": {"10"},
			"fields": {"message,permalink_url,comments.order(reverse_chronological).limit(15){id,message,from,created_time,permalink_url,comments.limit(20){from}}"}}), "Bearer "+a.tok), &ps); err != nil {
			return nil, err
		}
		for _, p := range ps.Data {
			for _, c := range p.Comments.Data {
				if a.mine(c.From) {
					continue
				}
				it := base
				it.ID, it.From, it.Text, it.Time, it.URL, it.Context, it.Pending = c.ID, or(c.From.Name, "Facebook user"), c.Message, c.Created, or(c.Permalink, p.Permalink), p.Message, true
				for _, rep := range c.Replies.Data {
					if a.mine(rep.From) {
						it.Pending = false
					}
				}
				out = append(out, it)
			}
		}
		return out, nil
	}
	var ms struct {
		Data []struct {
			Caption   string `json:"caption"`
			Permalink string `json:"permalink"`
			Comments  struct {
				Data []struct {
					ID        string `json:"id"`
					Text      string `json:"text"`
					Username  string `json:"username"`
					Timestamp string `json:"timestamp"`
					Replies   struct {
						Data []struct {
							Username string `json:"username"`
						} `json:"data"`
					} `json:"replies"`
				} `json:"data"`
			} `json:"comments"`
		} `json:"data"`
	}
	if err := call(get(graph("/"+a.accountID+"/media", url.Values{"limit": {"10"},
		"fields": {"caption,permalink,comments.limit(15){id,text,username,timestamp,replies{username}}"}}), "Bearer "+a.tok), &ms); err != nil {
		return nil, err
	}
	for _, m := range ms.Data {
		for _, c := range m.Comments.Data {
			if a.mine(gUser{Username: c.Username}) {
				continue
			}
			it := base
			it.ID, it.From, it.Text, it.Time, it.URL, it.Context, it.Pending = c.ID, "@"+c.Username, c.Text, c.Timestamp, m.Permalink, m.Caption, true
			for _, rep := range c.Replies.Data {
				if a.mine(gUser{Username: rep.Username}) {
					it.Pending = false
				}
			}
			out = append(out, it)
		}
	}
	return out, nil
}

// graphID keeps caller-supplied ids to plain Graph ids, so they can't steer a Page token at another endpoint.
var graphID = regexp.MustCompile(`^[\w=.:-]+$`)

func inboxTarget(b map[string]any) (inboxAccount, string, string, error) {
	accts, err := inboxAccounts("and a.id=?", b["account"])
	if err != nil || len(accts) == 0 {
		return inboxAccount{}, "", "", bad("that account is not connected")
	}
	id, kind := str(b["id"]), str(b["kind"])
	if !graphID.MatchString(id) || (kind != "dm" && kind != "comment") {
		return inboxAccount{}, "", "", bad("invalid conversation")
	}
	return accts[0], id, kind, nil
}

func threadMessages(a inboxAccount, kind, id string) ([]inboxMsg, error) {
	var out []inboxMsg
	switch {
	case a.platform == "whatsapp":
		return waThread(a, id)
	case kind == "dm":
		var c struct {
			Messages gMsgs `json:"messages"`
		}
		if err := call(get(graph("/"+id, url.Values{"fields": {"messages.limit(30){message,from,created_time}"}}), "Bearer "+a.tok), &c); err != nil {
			return nil, err
		}
		for i := len(c.Messages.Data) - 1; i >= 0; i-- { // the API returns newest first
			m := c.Messages.Data[i]
			out = append(out, inboxMsg{m.From.label(), m.Message, m.Created, a.mine(m.From)})
		}
	case a.platform == "facebook":
		var c struct {
			Message string `json:"message"`
			From    gUser  `json:"from"`
			Created string `json:"created_time"`
			Replies gMsgs  `json:"comments"`
		}
		if err := call(get(graph("/"+id, url.Values{"fields": {"message,from,created_time,comments.limit(30){message,from,created_time}"}}), "Bearer "+a.tok), &c); err != nil {
			return nil, err
		}
		out = append(out, inboxMsg{or(c.From.Name, "Facebook user"), c.Message, c.Created, false})
		for _, m := range c.Replies.Data {
			out = append(out, inboxMsg{or(m.From.Name, "Facebook user"), m.Message, m.Created, a.mine(m.From)})
		}
	default:
		type igComment struct {
			Text      string `json:"text"`
			Username  string `json:"username"`
			Timestamp string `json:"timestamp"`
		}
		var c struct {
			igComment
			Replies struct {
				Data []igComment `json:"data"`
			} `json:"replies"`
		}
		if err := call(get(graph("/"+id, url.Values{"fields": {"text,username,timestamp,replies{text,username,timestamp}"}}), "Bearer "+a.tok), &c); err != nil {
			return nil, err
		}
		for _, m := range append([]igComment{c.igComment}, c.Replies.Data...) {
			out = append(out, inboxMsg{"@" + m.Username, m.Text, m.Timestamp, a.mine(gUser{Username: m.Username})})
		}
	}
	return out, nil
}

func getThread(r *http.Request) (any, error) {
	q := r.URL.Query()
	a, id, kind, err := inboxTarget(map[string]any{"account": q.Get("account"), "id": q.Get("id"), "kind": q.Get("kind")})
	if err != nil {
		return nil, err
	}
	return threadMessages(a, kind, id)
}

const replyRules = `You answer %s for %s on %s (%s). Write only the reply, ready to send as-is.
- Use only facts from the knowledge base below. Never invent prices, dates, links, phone numbers, offers or promises. If the answer isn't there, say a team member will reply soon%s.
- Reply in the customer's language and script (Bangla stays in Bangla script).
- %s
- No markdown, no greeting boilerplate, at most one emoji.
%s
Knowledge base:
%s
%s
Conversation, oldest first:
%s

Reply as %s:`

// draftReply asks Claude for a reply grounded in the company's knowledge base; the editor reviews it before sending.
func draftReply(r *http.Request) (any, error) {
	b, err := body(r)
	if err != nil {
		return nil, err
	}
	a, id, kind, err := inboxTarget(b)
	if err != nil {
		return nil, err
	}
	text, err := draftFor(a, kind, id, str(b["context"]))
	if err != nil {
		return nil, err
	}
	return map[string]string{"text": text}, nil
}

func draftFor(a inboxAccount, kind, id, context string) (string, error) {
	msgs, err := threadMessages(a, kind, id)
	if err != nil {
		return "", err
	}
	var convo strings.Builder
	for _, m := range msgs {
		who := m.From
		if m.Mine {
			who = a.name + " (us)"
		}
		fmt.Fprintf(&convo, "%s: %s\n", who, m.Text)
	}
	what, style, fallback := "private messages", "Warm and helpful, 1-4 short sentences.", ""
	if kind == "comment" {
		what, style, fallback = "public comments", "Public reply: 1-2 sentences, nothing personal; move orders, payments and account questions to a private message.", " and invite them to send a message"
	}
	post := ""
	if c := strings.TrimSpace(context); c != "" {
		post = "\nThe comment is on this post:\n" + head(c, 1500) + "\n"
	}
	// The company's own reply rules (Social tab) override the style lines, never the facts rule.
	rules := ""
	if r := strings.TrimSpace(tenantDoc(a.tenant, "replies.md")); r != "" {
		rules = "\nThe company's own rules for replies. Follow them over the style lines above, but never break the facts rule:\n" + head(r, 4000) + "\n"
	}
	prompt := fmt.Sprintf(replyRules, what, a.client, a.platform, a.name, fallback, style, rules,
		or(head(tenantDoc(a.tenant, "knowledge.md"), 30000), "(empty: say a team member will reply soon)"), post, convo.String(), a.name)
	text, cost, tokens, err := claudeOnce(tenantRoot(a.tenant), prompt)
	if err == nil || cost > 0 {
		// Inbox work has no project; the row still counts toward the client's costs.
		db.Exec("insert into usage(project_id,tenant_id,provider,item,quantity,cost) values(0,?,?,?,?,?)", a.tenant, "anthropic", "inbox replies", tokens, cost)
	}
	return text, err
}

// sendReply posts the editor's reply: a DM inside the platform's 24-hour window, or a public comment reply.
func sendReply(r *http.Request) (any, error) {
	b, err := body(r)
	if err != nil {
		return nil, err
	}
	a, id, kind, err := inboxTarget(b)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(str(b["text"]))
	if text == "" {
		return nil, bad("reply is empty")
	}
	if err := sendTo(a, kind, id, str(b["to"]), text); err != nil {
		return nil, err
	}
	db.Exec("update inbox_drafts set status='sent' where account_id=? and item=? and status='ready'", a.id, id)
	return nil, nil
}

func sendTo(a inboxAccount, kind, id, to, text string) error {
	switch {
	case a.platform == "whatsapp":
		return waSend(a, id, text)
	case kind == "dm":
		if !graphID.MatchString(to) {
			return bad("no recipient")
		}
		return call(jsonReq("POST", graph("/me/messages", nil), map[string]any{"recipient": map[string]string{"id": to},
			"messaging_type": "RESPONSE", "message": map[string]string{"text": text}}, "Bearer "+a.tok), nil)
	case a.platform == "facebook":
		return call(form("POST", graph("/"+id+"/comments", nil), url.Values{"message": {text}}, "Bearer "+a.tok), nil)
	default:
		return call(form("POST", graph("/"+id+"/replies", nil), url.Values{"message": {text}}, "Bearer "+a.tok), nil)
	}
}

// ---- auto-replies: per company, "draft" (Claude writes, you approve in the Inbox) or "send" (Claude answers alone).
// They run while 37Studio runs. Messages that came in while the computer was off are answered when it's back, as long
// as the platform still allows a reply: 24 hours for messages; comments older than two days are left to people.

const inboxAutoMax = 20 // ponytail: per two-minute round, so a flood of comments can't run up the Claude bill

func inboxLoop() {
	for {
		time.Sleep(2 * time.Minute)
		inboxTick()
	}
}

func inboxTick() {
	ts, _ := rows("select id, inbox_mode from tenants where inbox_mode in ('draft','send')")
	n := 0
	for _, t := range ts {
		accts, _ := inboxAccounts("and a.tenant_id=?", t["id"])
		for _, a := range accts {
			for _, kind := range []string{"dm", "comment"} {
				items, _ := fetchInbox(a, kind)
				for _, it := range items {
					if n >= inboxAutoMax || !it.Pending || !inboxAnswerable(it) {
						continue
					}
					var done int
					if db.QueryRow("select 1 from inbox_drafts where account_id=? and item=? and answers=?", a.id, it.ID, it.Time).Scan(&done) == nil {
						continue // already drafted or answered this message
					}
					n++
					autoReply(a, it, str(t["inbox_mode"]))
				}
			}
		}
	}
}

func inboxAnswerable(it inboxItem) bool {
	t, err := time.Parse("2006-01-02T15:04:05-0700", it.Time)
	limit := map[string]time.Duration{"dm": 23 * time.Hour, "comment": 48 * time.Hour}[it.Kind]
	return err == nil && time.Since(t) < limit
}

func autoReply(a inboxAccount, it inboxItem, mode string) {
	text, err := draftFor(a, it.Kind, it.ID, it.Context)
	status := "ready"
	if err == nil && mode == "send" {
		if err = sendTo(a, it.Kind, it.ID, it.To, text); err == nil {
			status = "sent"
		}
	}
	msg := ""
	if err != nil {
		status, msg = "failed", err.Error()
		log.Printf("auto-reply to %s on %s: %v", it.From, a.name, err)
	}
	db.Exec(`insert into inbox_drafts(account_id,item,kind,answers,text,status,error) values(?,?,?,?,?,?,?)
	  on conflict(account_id,item) do update set kind=excluded.kind, answers=excluded.answers, text=excluded.text, status=excluded.status,
	  error=excluded.error, at=current_timestamp`, a.id, it.ID, it.Kind, it.Time, text, status, msg)
}
