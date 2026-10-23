package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Provider keys entered in Settings → Keys. They are encrypted (AES-GCM) in 37studio.db with a key file that lives next to
// it (data/secret.key, readable only by this user), applied to the process environment at startup and on save, and sent
// nowhere but the provider they belong to. ponytail: the key file sits beside the DB, so this guards against a copied or
// shared database, not against someone who already has the whole data folder; disk encryption covers that.

var keyNames = []struct{ Name, Label, Hint string }{
	{"ANTHROPIC_API_KEY", "Anthropic API key", "console.anthropic.com → API keys. Runs the editor agent and writes captions and replies."},
	{"CLAUDE_CODE_OAUTH_TOKEN", "Claude subscription token", "Instead of an API key, uses your Claude Pro/Max plan. In the 37Studio folder run `docker compose exec studio claude setup-token`, sign in, and paste the token here. Claude Desktop's sign-in doesn't carry over."},
	{"GEMINI_API_KEY", "Gemini API key", "aistudio.google.com → API keys. Image and video generation, smarter website scans."},
	{"ELEVENLABS_API_KEY", "ElevenLabs API key", "elevenlabs.io → Developers → API keys. Voiceover, sound effects and music."},
	// Optional: the user's own Meta app instead of the shared one (Company → Social → Use your own Meta app).
	{"META_APP_ID", "Meta app ID", "Your own Meta app, for Facebook and Instagram. Used only together with its secret."},
	{"META_APP_SECRET", "Meta app secret", "developers.facebook.com → your app → App settings → Basic."},
	{"META_CONFIG_ID", "Meta login configuration ID", "Facebook Login for Business → Configurations. Needed for Instagram video uploads."},
}

func knownKey(name string) bool {
	for _, k := range keyNames {
		if k.Name == name {
			return true
		}
	}
	return false
}

func secretKey() ([]byte, error) {
	p := filepath.Join(dataDir, "secret.key")
	if b, err := os.ReadFile(p); err == nil && len(b) == 32 {
		return b, nil
	}
	b := make([]byte, 32)
	rand.Read(b)
	return b, os.WriteFile(p, b, 0o600)
}

func sealer() (cipher.AEAD, error) {
	k, err := secretKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func seal(plain string) ([]byte, error) {
	g, err := sealer()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, g.NonceSize())
	rand.Read(nonce)
	return g.Seal(nonce, nonce, []byte(plain), nil), nil
}

func unseal(b []byte) (string, error) {
	g, err := sealer()
	if err != nil {
		return "", err
	}
	if len(b) < g.NonceSize() {
		return "", errors.New("corrupt secret")
	}
	out, err := g.Open(nil, b[:g.NonceSize()], b[g.NonceSize():], nil)
	return string(out), err
}

// envOrig remembers keys from the real environment (or .env): the Keys page says so, and removing an override restores them.
var envOrig = map[string]string{}

// loadKeys applies stored keys over the environment. Called once after openDB.
func loadKeys() {
	for _, k := range keyNames {
		envOrig[k.Name] = os.Getenv(k.Name)
	}
	rs, _ := rows("select name, value from secrets")
	for _, r := range rs {
		raw, _ := r["value"].(string) // rows() turns blobs into strings
		if v, err := unseal([]byte(raw)); err == nil && knownKey(str(r["name"])) {
			os.Setenv(str(r["name"]), v)
		}
	}
}

func listKeys(r *http.Request) (any, error) {
	stored := map[string]bool{}
	rs, _ := rows("select name from secrets")
	for _, r := range rs {
		stored[str(r["name"])] = true
	}
	out := []map[string]any{}
	for _, k := range keyNames {
		v := os.Getenv(k.Name)
		item := map[string]any{"name": k.Name, "label": k.Label, "hint": k.Hint, "set": v != "", "source": ""}
		if v != "" {
			item["last4"] = v[max(0, len(v)-4):]
			item["source"] = map[bool]string{true: "app", false: "env"}[stored[k.Name]]
		}
		out = append(out, item)
	}
	return out, nil
}

// putKeys saves {NAME: value}; an empty value removes the stored key.
func putKeys(r *http.Request) (any, error) {
	b, err := body(r)
	if err != nil {
		return nil, err
	}
	for name, v := range b {
		if !knownKey(name) {
			return nil, bad("unknown key " + name)
		}
		val := str(v)
		if val == "" {
			db.Exec("delete from secrets where name=?", name)
			os.Setenv(name, envOrig[name])
			continue
		}
		enc, err := seal(val)
		if err != nil {
			return nil, err
		}
		if _, err := db.Exec("insert into secrets(name,value) values(?,?) on conflict(name) do update set value=excluded.value", name, enc); err != nil {
			return nil, err
		}
		os.Setenv(name, val)
	}
	return listKeys(r)
}

// ---- Claude sign-in: the agent, website scans, captions and replies all run Claude Code, which needs one of these.

const noClaude = "37Studio has no Claude key yet. Add one in Settings → Keys: an Anthropic API key, or a Claude subscription token. " +
	"Claude Desktop's sign-in can't be shared with 37Studio; the setup guide shows how to get a token: https://studio.shaitrish.com/setup#keys"

// claudeReady is checked before starting Claude, so a missing key reads as a clear message instead of Claude Code's "Please run /login".
func claudeReady() error {
	home, _ := os.UserHomeDir()
	if os.Getenv("ANTHROPIC_API_KEY") != "" || os.Getenv("CLAUDE_CODE_OAUTH_TOKEN") != "" {
		return nil
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", ".credentials.json")); err == nil { // `claude /login` inside the container
		return nil
	}
	return bad(noClaude)
}

// claudeHint turns Claude Code's sign-in and billing errors into what to do about them; "" when it's something else.
func claudeHint(msg string) string {
	m := strings.ToLower(msg)
	switch {
	case strings.Contains(m, "not logged in") || strings.Contains(m, "/login"):
		return noClaude
	case strings.Contains(m, "expired") && (strings.Contains(m, "oauth") || strings.Contains(m, "token")):
		return "Your Claude subscription token has expired. Make a new one with `claude setup-token` and paste it in Settings → Keys."
	case strings.Contains(m, "invalid api key") || strings.Contains(m, "authentication_error") || strings.Contains(m, "invalid x-api-key"):
		return "Claude didn't accept the key in Settings → Keys. Check it, or create a new one."
	case strings.Contains(m, "credit balance"):
		return "Your Anthropic account is out of credit. Add some at console.anthropic.com → Billing, then try again."
	}
	return ""
}
