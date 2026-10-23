package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The inbox pulls WhatsApp messages from the Worker by cursor: a second pull must not duplicate, chats group by
// customer with the newest message on top, and a thread reads oldest first.
func TestWhatsAppSync(t *testing.T) {
	dataDir = t.TempDir()
	openDB()
	var gotAfter []string
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAfter = append(gotAfter, r.URL.Query().Get("after"))
		if r.Header.Get("Authorization") != "Bearer tok" || r.URL.Query().Get("phone") != "555" {
			w.WriteHeader(403)
			return
		}
		w.Write([]byte(`{"messages":[
		  {"seq":7,"id":"w1","from":"8801711","name":"Rahim","timestamp":"1761200000","type":"text","text":{"body":"দাম কত?"}},
		  {"seq":8,"id":"w2","from":"8801822","name":"","timestamp":"1761200100","type":"image"},
		  {"seq":9,"id":"w3","from":"8801711","name":"Rahim","timestamp":"1761200200","type":"text","text":{"body":"ডেলিভারি?"}}]}`))
	}))
	defer relay.Close()
	old := authURL
	authURL = relay.URL
	defer func() { authURL = old }()

	db.Exec("insert into tenants(id,name) values(1,'Shop')")
	db.Exec("insert into social_accounts(id,tenant_id,platform,account_id,name,token) values(1,1,'whatsapp','555','Shop +880','tok')")
	a := inboxAccount{id: 1, tenant: 1, client: "Shop", platform: "whatsapp", accountID: "555", name: "Shop +880", tok: "tok"}

	if _, err := waChats(a); err != nil {
		t.Fatal(err)
	}
	chats, _ := waChats(a) // the relay sends the same messages again: they must not duplicate
	if len(gotAfter) != 2 || gotAfter[0] != "0" || gotAfter[1] != "9" {
		t.Errorf("cursor: pulled after %v, want [0 9]", gotAfter)
	}
	if len(chats) != 2 || chats[0].From != "Rahim" || chats[0].Text != "ডেলিভারি?" || chats[1].From != "+8801822" || chats[1].Text != "[image]" {
		t.Fatalf("chats: %+v", chats)
	}
	th, err := waThread(a, "8801711")
	if err != nil || len(th) != 2 || th[0].Text != "দাম কত?" || th[1].Text != "ডেলিভারি?" || th[0].Time != "2025-10-23T06:13:20+0000" {
		t.Fatalf("thread: %+v %v", th, err)
	}
}

// Auto-replies only answer what the platform still accepts: messages within 24 hours, comments within 2 days.
func TestInboxAnswerable(t *testing.T) {
	hour, day := waTime(time.Now().Add(-time.Hour)), waTime(time.Now().Add(-30*time.Hour))
	if !inboxAnswerable(inboxItem{Kind: "dm", Time: hour}) || inboxAnswerable(inboxItem{Kind: "dm", Time: day}) ||
		!inboxAnswerable(inboxItem{Kind: "comment", Time: day}) || inboxAnswerable(inboxItem{Kind: "dm", Time: "garbage"}) {
		t.Error("wrong answer window")
	}
}
