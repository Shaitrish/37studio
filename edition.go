package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Editions: "agency" (Shaitrish's own install: clients, pricing, invoices) and "personal" (sold to friends and family:
// brands, social, autopilot, no billing). Personal copies check studio.shaitrish.com for updates and carry a license key.

const version = "0.3.0"

var updateURL = env("STUDIO37_UPDATE_URL", "https://studio.shaitrish.com/version.json")

// licensePub verifies license keys. The private half lives only on the release machine (~/.config/37studio).
// A var so tests can sign with their own key pair; the Worker on studio.shaitrish.com holds the same public key.
var licensePub = "/4H+3jLcxlI0cnvC94wpnc4Doqa2iActEj2Nq5XoKn8="

// Without a license: two videos to try it, no Autopilot, auto-replies or publishing. Facebook and WhatsApp
// also need it server side: the Worker refuses unlicensed copies, so patching this file doesn't unlock them.
const trialProjects = 2

const licenseURL = "https://studio.shaitrish.com/#pricing"

func licensed() bool {
	_, err := parseLicense(setting("license"))
	return err == nil
}

// trialLeft counts projects ever made, so deleting one doesn't hand the trial back.
func trialLeft() int {
	var n int
	db.QueryRow("select max((select count(*) from projects), coalesce((select cast(value as integer) from settings where key='projects_made'), 0))").Scan(&n)
	return max(0, trialProjects-n)
}

// needLicense is the 402 the UI turns into the "Activate 37Studio" dialog.
func needLicense(what string) error {
	if licensed() {
		return nil
	}
	return apiErr{http.StatusPaymentRequired, what + " needs a 37Studio license. Add your key in Settings, or get one at " + licenseURL}
}

type license struct {
	Name   string `json:"name"`
	Email  string `json:"email"`
	Issued string `json:"issued"`
}

// parseLicense checks a key of the form 37S1.<payload>.<signature> (both base64url) against licensePub.
func parseLicense(key string) (*license, error) {
	parts := strings.Split(strings.TrimSpace(key), ".")
	if len(parts) != 3 || parts[0] != "37S1" {
		return nil, errors.New("that is not a 37Studio license key")
	}
	payload, err1 := base64.RawURLEncoding.DecodeString(parts[1])
	sig, err2 := base64.RawURLEncoding.DecodeString(parts[2])
	pub, _ := base64.StdEncoding.DecodeString(licensePub)
	if err1 != nil || err2 != nil || !ed25519.Verify(pub, payload, sig) {
		return nil, errors.New("that license key is not valid")
	}
	var l license
	return &l, json.Unmarshal(payload, &l)
}

func getLicense(r *http.Request) (any, error) {
	l, err := parseLicense(setting("license"))
	if err != nil {
		return map[string]any{"valid": false, "trial_left": trialLeft(), "trial": trialProjects, "url": licenseURL}, nil
	}
	return map[string]any{"valid": true, "name": l.Name, "email": l.Email}, nil
}

func putLicense(r *http.Request) (any, error) {
	b, err := body(r)
	if err != nil {
		return nil, err
	}
	if _, err := parseLicense(str(b["key"])); err != nil {
		return nil, bad(err.Error())
	}
	db.Exec("insert into settings(key,value) values('license',?) on conflict(key) do update set value=excluded.value", strings.TrimSpace(str(b["key"])))
	return getLicense(r)
}

// licenseCLI issues keys on the release machine: 37studio license --name "Rafi" --email rafi@example.com
func licenseCLI(args []string) int {
	fs := flag.NewFlagSet("license", flag.ExitOnError)
	name := fs.String("name", "", "buyer's name")
	email := fs.String("email", "", "buyer's email")
	keyFile := fs.String("key", filepath.Join(os.Getenv("HOME"), ".config", "37studio", "license-private.key"), "signing key")
	fs.Parse(args)
	raw, err := os.ReadFile(*keyFile)
	priv, err2 := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || err2 != nil || len(priv) != ed25519.PrivateKeySize || *name == "" {
		fmt.Fprintln(os.Stderr, "usage: 37studio license --name NAME [--email EMAIL]  (needs the private key at", *keyFile+")")
		return 1
	}
	payload, _ := json.Marshal(license{*name, *email, time.Now().Format(time.DateOnly)})
	fmt.Printf("37S1.%s.%s\n", base64.RawURLEncoding.EncodeToString(payload), base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, payload)))
	return 0
}

// ---- update check: version.json on studio.shaitrish.com, fetched at most every 6 hours

var (
	updMu   sync.Mutex
	updAt   time.Time
	updInfo map[string]any
)

func checkUpdate(r *http.Request) (any, error) {
	updMu.Lock()
	defer updMu.Unlock()
	if updInfo == nil || time.Since(updAt) > 6*time.Hour {
		updAt = time.Now()
		var v struct {
			Version string `json:"version"`
			Notes   string `json:"notes"`
			URL     string `json:"url"`
		}
		if err := call(get(updateURL, ""), &v); err != nil {
			return map[string]any{"current": version, "error": err.Error()}, nil // offline is fine; try again later
		}
		updInfo = map[string]any{"current": version, "latest": v.Version, "notes": v.Notes, "url": v.URL, "newer": newer(v.Version, version)}
	}
	return updInfo, nil
}

// newer reports whether a > b for dotted versions like 0.3.10.
func newer(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < max(len(pa), len(pb)); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x > y
		}
	}
	return false
}
