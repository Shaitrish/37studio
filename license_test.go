package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
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

	payload, _ := json.Marshal(license{"Rafi", "rafi@example.com", "2026-10-23"})
	key := "37S1." + base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, payload))
	db.Exec("insert into settings(key,value) values('license',?)", key)
	if err := newProject(); err != nil {
		t.Fatalf("licensed copy: %v", err)
	}
}
