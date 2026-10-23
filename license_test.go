package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Without a license a copy makes two videos, then project creation, Autopilot and publishing answer 402.
// Deleting a project doesn't give the trial back; a signed key unlocks everything.
func TestTrialAndLicense(t *testing.T) {
	dataDir = t.TempDir()
	openDB()
	pub, priv, _ := ed25519.GenerateKey(nil)
	old := licensePub
	licensePub = base64.StdEncoding.EncodeToString(pub)
	defer func() { licensePub = old }()

	db.Exec("insert into tenants(id,name) values(1,'Shop')")
	newProject := func() error {
		_, err := saveProject(httptest.NewRequest("POST", "/api/projects", strings.NewReader(`{"tenant_id":1,"title":"Reel"}`)))
		return err
	}
	paid := func(err error) bool { var e apiErr; return errors.As(err, &e) && e.code == 402 }
	for i := 0; i < trialProjects; i++ {
		if err := newProject(); err != nil {
			t.Fatalf("trial project %d: %v", i+1, err)
		}
	}
	db.Exec("delete from projects")
	if err := newProject(); !paid(err) {
		t.Fatalf("after the trial (and deleting projects) want 402, got %v", err)
	}
	if _, err := createPosts(httptest.NewRequest("POST", "/api/projects/1/posts", strings.NewReader(`{}`))); !paid(err) {
		t.Fatalf("publishing without a license: want 402, got %v", err)
	}

	sign := func(l license) string {
		payload, _ := json.Marshal(l)
		return "37S1." + base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, payload))
	}
	setKey := func(k string) {
		db.Exec("insert into settings(key,value) values('license',?) on conflict(key) do update set value=excluded.value", k)
	}

	setKey(sign(license{Name: "Rafi", Issued: "2026-04-01", Expires: "2026-10-01"})) // a 6-month key that has run out
	if err := newProject(); !paid(err) || !strings.Contains(err.Error(), "ended on 2026-10-01") {
		t.Fatalf("expired license: want 402 naming the end date, got %v", err)
	}
	setKey(sign(license{Name: "Rafi", Issued: "2026-10-23", Expires: time.Now().AddDate(0, 6, 0).Format(time.DateOnly)}))
	if err := newProject(); err != nil {
		t.Fatalf("active 6-month license: %v", err)
	}
	setKey(sign(license{Name: "Shaitrish", Issued: "2026-10-23"})) // no end date
	if err := newProject(); err != nil {
		t.Fatalf("license without an end date: %v", err)
	}

	// Revoked on studio.shaitrish.com: the next check locks the copy; un-revoking (or another answer) unlocks it again.
	isRevoked := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/license/check" || !strings.HasPrefix(r.Header.Get("X-37Studio-License"), "37S1.") {
			w.WriteHeader(400)
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"revoked": isRevoked})
	}))
	defer srv.Close()
	oldAuth := authURL
	authURL = srv.URL
	defer func() { authURL = oldAuth }()
	checkRevoked()
	if err := newProject(); !paid(err) || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("revoked license: want 402 saying it was cancelled, got %v", err)
	}
	isRevoked = false
	checkRevoked()
	if err := newProject(); err != nil {
		t.Fatalf("un-revoked license: %v", err)
	}
}
