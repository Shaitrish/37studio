package social

import (
	"cmp"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/keys"
	"github.com/Shaitrish/37studio/internal/remote"
	"github.com/Shaitrish/37studio/internal/store"
)

var digitsOnly = regexp.MustCompile(`^\d+$`)

// ConnectWhatsApp adds a number using the three values on Meta's WhatsApp → API Setup page.
func ConnectWhatsApp(r *http.Request) (any, error) {
	b, err := httpapi.Body(r)
	if err != nil {
		return nil, err
	}
	phone, waba, tok := strings.TrimSpace(store.Str(b["phone_id"])), strings.TrimSpace(store.Str(b["waba_id"])), strings.TrimSpace(store.Str(b["token"]))
	if config.RelayURL == "" && (os.Getenv("META_VERIFY_TOKEN") == "" || !ownMetaApp()) {
		return nil, httpapi.Bad("WhatsApp delivers messages only to a public web address. Expose this copy's /webhooks/meta with a Cloudflare Tunnel (then add your Meta app and a webhook verify token in Settings → Keys), or deploy your own relay and set STUDIO37_RELAY_URL. The connect guide shows both: https://studio.shaitrish.com/connect#whatsapp-steps")
	}
	if !digitsOnly.MatchString(phone) || !digitsOnly.MatchString(waba) || tok == "" {
		return nil, httpapi.Bad("fill in all three from Meta's WhatsApp → API Setup page")
	}
	var p struct {
		Display string `json:"display_phone_number"`
		Name    string `json:"verified_name"`
	}
	if err := remote.Call(remote.Get(graph("/"+phone, url.Values{"fields": {"display_phone_number,verified_name"}}), "Bearer "+tok), &p); err != nil {
		return nil, httpapi.Bad("Meta didn't accept that number and token: " + err.Error())
	}
	// Without this subscription Meta sends the number's messages nowhere.
	if err := remote.Call(remote.Form("POST", graph("/"+waba+"/subscribed_apps", nil), nil, "Bearer "+tok), nil); err != nil {
		return nil, httpapi.Bad("couldn't turn on messages for that WhatsApp Business Account: " + err.Error())
	}
	_, err = store.DB.Exec(`insert into social_accounts(tenant_id,platform,account_id,name,token) values(?,'whatsapp',?,?,?)
	  on conflict(tenant_id,platform,account_id) do update set name=excluded.name, token=excluded.token`,
		httpapi.PathID(r), phone, strings.TrimSpace(p.Name+" "+p.Display), keys.SealText(tok))
	return nil, err
}

type waMsg struct {
	Seq       int64  `json:"seq"`
	ID        string `json:"id"`
	From      string `json:"from"`
	Name      string `json:"name"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Text      struct {
		Body string `json:"body"`
	} `json:"text"`
}

// waTime matches the Graph API's time format, so WhatsApp sorts in with Messenger and comments.
func waTime(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05+0000") }

// waSync pulls messages that arrived since the last pull.
func waSync(a inboxAccount) error {
	if config.RelayURL == "" {
		return nil // messages arrive at WhatsAppWebhook instead
	}
	var after int64
	store.DB.QueryRow("select coalesce(max(seq),0) from wa_messages where account_id=?", a.id).Scan(&after)
	var res struct {
		Messages []waMsg `json:"messages"`
	}
	if err := remote.Call(remote.Get(config.RelayURL+"/whatsapp/messages?"+url.Values{"phone": {a.accountID}, "after": {fmt.Sprint(after)}}.Encode(), "Bearer "+a.tok), &res); err != nil {
		return err
	}
	for _, m := range res.Messages {
		waStore(a.id, m)
	}
	return nil
}

func waStore(account int64, m waMsg) {
	text := m.Text.Body
	if text == "" {
		text = "[" + m.Type + "]" // photos, voice notes and the like: open WhatsApp to see them
	}
	var sec int64
	fmt.Sscan(m.Timestamp, &sec)
	var seq any
	if m.Seq > 0 { // only relay pulls have a cursor
		seq = m.Seq
	}
	store.DB.Exec("insert or ignore into wa_messages(id,account_id,customer,name,text,at,seq) values(?,?,?,?,?,?,?)",
		m.ID, account, m.From, m.Name, text, waTime(time.Unix(sec, 0)), seq)
}

// WhatsAppWebhook takes Meta's WhatsApp webhooks straight into this copy, for one the internet can reach: a server,
// or a Cloudflare Tunnel that exposes only this path. Meta can't send the app's password, so a POST counts only if it
// is signed with the Meta app's secret; GET is Meta's subscription check with META_VERIFY_TOKEN.
func WhatsAppWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		q, tok := r.URL.Query(), os.Getenv("META_VERIFY_TOKEN")
		if tok == "" || q.Get("hub.mode") != "subscribe" || subtle.ConstantTimeCompare([]byte(q.Get("hub.verify_token")), []byte(tok)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Write([]byte(q.Get("hub.challenge")))
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil || !signedByMeta(raw, r.Header.Get("X-Hub-Signature-256")) {
		http.Error(w, "bad signature", http.StatusUnauthorized)
		return
	}
	var body struct {
		Entry []struct {
			Changes []struct {
				Value struct {
					Metadata struct {
						Phone string `json:"phone_number_id"`
					} `json:"metadata"`
					Contacts []struct {
						WaID    string `json:"wa_id"`
						Profile struct {
							Name string `json:"name"`
						} `json:"profile"`
					} `json:"contacts"`
					Messages []waMsg `json:"messages"`
				} `json:"value"`
			} `json:"changes"`
		} `json:"entry"`
	}
	json.Unmarshal(raw, &body)
	for _, e := range body.Entry {
		for _, c := range e.Changes {
			v := c.Value
			rs, _ := store.Rows("select id from social_accounts where platform='whatsapp' and account_id=?", v.Metadata.Phone)
			for _, m := range v.Messages {
				for _, ct := range v.Contacts {
					if ct.WaID == m.From {
						m.Name = ct.Profile.Name
					}
				}
				for _, a := range rs {
					waStore(a["id"].(int64), m)
				}
			}
		}
	}
	w.Write([]byte("ok")) // anything else makes Meta retry
}

func signedByMeta(raw []byte, header string) bool {
	sig, err := hex.DecodeString(strings.TrimPrefix(header, "sha256="))
	secret := os.Getenv("META_APP_SECRET")
	if err != nil || secret == "" || !strings.HasPrefix(header, "sha256=") {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(raw)
	return hmac.Equal(sig, mac.Sum(nil))
}

func waChats(a inboxAccount) ([]inboxItem, error) {
	syncErr := waSync(a)
	rs, err := store.Rows(`select m.customer, m.text, m.at, m.mine,
	    (select name from wa_messages where account_id=m.account_id and customer=m.customer and name<>'' order by rowid desc limit 1) as name
	  from wa_messages m where m.account_id=? and m.rowid=(select max(rowid) from wa_messages where account_id=m.account_id and customer=m.customer)
	  order by m.rowid desc limit 50`, a.id)
	var out []inboxItem
	for _, r := range rs {
		c := store.Str(r["customer"])
		out = append(out, inboxItem{ID: c, To: c, Account: a.id, Tenant: a.tenant, Client: a.client, Platform: a.platform, Page: a.name, Kind: "dm",
			From: cmp.Or(store.Str(r["name"]), "+"+c), Text: store.Str(r["text"]), Time: store.Str(r["at"]), Pending: store.Num(r["mine"]) == 0})
	}
	if err == nil {
		err = syncErr
	}
	return out, err
}

func waThread(a inboxAccount, customer string) ([]inboxMsg, error) {
	waSync(a)
	rs, err := store.Rows("select name, mine, text, at from wa_messages where account_id=? and customer=? order by rowid desc limit 30", a.id, customer)
	out := make([]inboxMsg, len(rs))
	for i, r := range rs { // newest first from the query, oldest first for the thread
		mine := store.Num(r["mine"]) == 1
		from := cmp.Or(store.Str(r["name"]), "+"+customer)
		if mine {
			from = a.name
		}
		out[len(rs)-1-i] = inboxMsg{from, store.Str(r["text"]), store.Str(r["at"]), mine}
	}
	return out, err
}

// waSend replies inside WhatsApp's 24-hour window; later than that Meta only allows approved templates.
func waSend(a inboxAccount, customer, text string) error {
	var res struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := remote.Call(remote.JSON("POST", graph("/"+a.accountID+"/messages", nil), map[string]any{"messaging_product": "whatsapp", "to": customer,
		"type": "text", "text": map[string]string{"body": text}}, "Bearer "+a.tok), &res); err != nil {
		return err
	}
	if len(res.Messages) > 0 {
		store.DB.Exec("insert or ignore into wa_messages(id,account_id,customer,mine,text,at) values(?,?,?,1,?,?)", res.Messages[0].ID, a.id, customer, text, waTime(time.Now()))
	}
	return nil
}
