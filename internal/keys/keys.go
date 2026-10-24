// Package keys stores provider keys and platform credentials encrypted at rest and applies them to the environment.
package keys

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/store"
)

var Names = []struct{ Name, Label, Hint string }{
	{"ANTHROPIC_API_KEY", "Anthropic API key", "console.anthropic.com → API keys. Runs the editor agent and writes captions and replies."},
	{"CLAUDE_CODE_OAUTH_TOKEN", "Claude subscription token", "Instead of an API key, uses your Claude Pro/Max plan. In the 37Studio folder run `docker compose exec studio claude setup-token`, sign in, and paste the token here. Claude Desktop's sign-in doesn't carry over."},
	{"GEMINI_API_KEY", "Gemini API key", "aistudio.google.com → API keys. Image and video generation, smarter website scans."},
	{"ELEVENLABS_API_KEY", "ElevenLabs API key", "elevenlabs.io → Developers → API keys. Voiceover, sound effects and music."},
	{"FAL_KEY", "fal.ai API key", "fal.ai → Dashboard → API keys. Video models such as Seedance, Kling, Veo and Wan."},
	// Other agent models, run through OpenCode: each key adds that provider's models to the model pickers.
	{"OPENCODE_API_KEY", "OpenCode API key", "opencode.ai → Zen or Go. Many models (GPT, Grok, Kimi, GLM, DeepSeek…) on one bill."},
	{"OPENAI_API_KEY", "OpenAI API key", "platform.openai.com → API keys. GPT models."},
	{"XAI_API_KEY", "xAI API key", "console.x.ai → API keys. Grok models."},
	{"OPENROUTER_API_KEY", "OpenRouter API key", "openrouter.ai → Keys. Hundreds of models through one key."},
	// Optional: the user's own Meta app instead of the shared one (Brand → Social → Use your own Meta app).
	{"META_APP_ID", "Meta app ID", "Your own Meta app, for Facebook and Instagram. Used only together with its secret."},
	{"META_APP_SECRET", "Meta app secret", "developers.facebook.com → your app → App settings → Basic."},
	{"META_CONFIG_ID", "Meta login configuration ID", "Facebook Login for Business → Configurations. Needed for Instagram video uploads."},
	{"META_VERIFY_TOKEN", "WhatsApp webhook verify token", "Only for WhatsApp through a Cloudflare Tunnel: any long random text, entered again in your Meta app's WhatsApp webhook settings."},
	// Each platform's own developer app, for Brand → Social (README → Platform setup).
	{"GOOGLE_CLIENT_ID", "Google OAuth client ID", "YouTube: console.cloud.google.com → Credentials → OAuth client ID (Web application)."},
	{"GOOGLE_CLIENT_SECRET", "Google OAuth client secret", "The same OAuth client's secret."},
	{"TIKTOK_CLIENT_KEY", "TikTok client key", "developers.tiktok.com → your app (Desktop platform, Login Kit and Content Posting API)."},
	{"TIKTOK_CLIENT_SECRET", "TikTok client secret", "The same TikTok app's secret."},
	{"LINKEDIN_CLIENT_ID", "LinkedIn client ID", "linkedin.com/developers → your app → Auth. Products: Sign In with LinkedIn using OpenID Connect, Share on LinkedIn."},
	{"LINKEDIN_CLIENT_SECRET", "LinkedIn client secret", "The same LinkedIn app's primary client secret."},
	{"X_CLIENT_ID", "X OAuth 2.0 client ID", "developer.x.com → your app → Keys and tokens → OAuth 2.0 Client ID."},
	{"X_CLIENT_SECRET", "X OAuth 2.0 client secret", "The same app's OAuth 2.0 Client Secret."},
	{"PINTEREST_APP_ID", "Pinterest app ID", "developers.pinterest.com → My apps → your app."},
	{"PINTEREST_APP_SECRET", "Pinterest app secret", "The same Pinterest app's secret key."},
}

func knownKey(name string) bool {
	for _, k := range Names {
		if k.Name == name {
			return true
		}
	}
	return false
}

func secretKey() ([]byte, error) {
	p := filepath.Join(config.DataDir, "secret.key")
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

func Seal(plain string) ([]byte, error) {
	g, err := sealer()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, g.NonceSize())
	rand.Read(nonce)
	return g.Seal(nonce, nonce, []byte(plain), nil), nil
}

func Unseal(b []byte) (string, error) {
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

const sealedPrefix = "sealed:"

// SealText seals a platform token for a text column. If secret.key can't be read or written it keeps the value as
// it is, which OpenText reads like a row saved before tokens were sealed, rather than lose the login.
func SealText(plain string) string {
	if plain == "" || strings.HasPrefix(plain, sealedPrefix) {
		return plain
	}
	b, err := Seal(plain)
	if err != nil {
		return plain
	}
	return sealedPrefix + base64.StdEncoding.EncodeToString(b)
}

// OpenText reads what SealText wrote. One it can't open (secret.key replaced) reads as "", so the platform asks for
// a new login.
func OpenText(s string) string {
	rest, ok := strings.CutPrefix(s, sealedPrefix)
	if !ok {
		return s
	}
	b, err := base64.StdEncoding.DecodeString(rest)
	if err != nil {
		return ""
	}
	v, err := Unseal(b)
	if err != nil {
		return ""
	}
	return v
}

var version atomic.Int64 // bumped when a key changes, so lists built from the keys (OpenCode's models) refresh

// Version changes whenever a key is saved or removed.
func Version() int64 { return version.Load() }

// envOrig remembers keys from the real environment (or .env): the Keys page says so, and removing an override restores them.
var envOrig = map[string]string{}

// Load applies stored keys over the environment. Called once after openDB.
func Load() {
	for _, k := range Names {
		envOrig[k.Name] = os.Getenv(k.Name)
	}
	rs, _ := store.Rows("select name, value from secrets")
	for _, r := range rs {
		raw, _ := r["value"].(string) // rows() turns blobs into strings
		if v, err := Unseal([]byte(raw)); err == nil && knownKey(store.Str(r["name"])) {
			os.Setenv(store.Str(r["name"]), v)
		}
	}
}

func List(r *http.Request) (any, error) {
	stored := map[string]bool{}
	rs, _ := store.Rows("select name from secrets")
	for _, r := range rs {
		stored[store.Str(r["name"])] = true
	}
	out := []map[string]any{}
	for _, k := range Names {
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

// Put saves {NAME: value}; an empty value removes the stored key.
func Put(r *http.Request) (any, error) {
	b, err := httpapi.Body(r)
	if err != nil {
		return nil, err
	}
	for name, v := range b {
		if !knownKey(name) {
			return nil, httpapi.Bad("unknown key " + name)
		}
		val := store.Str(v)
		if val == "" {
			store.DB.Exec("delete from secrets where name=?", name)
			os.Setenv(name, envOrig[name])
			continue
		}
		enc, err := Seal(val)
		if err != nil {
			return nil, err
		}
		if _, err := store.DB.Exec("insert into secrets(name,value) values(?,?) on conflict(name) do update set value=excluded.value", name, enc); err != nil {
			return nil, err
		}
		os.Setenv(name, val)
	}
	version.Add(1)
	return List(r)
}

const noClaude = "37Studio has no Claude key yet. Add one in Settings → Keys: an Anthropic API key, or a Claude subscription token. " +
	"Claude Desktop's sign-in can't be shared with 37Studio; the setup guide shows how to get a token: https://studio.shaitrish.com/setup#keys"

// ClaudeLoggedIn reports whether Claude Code itself is signed in here (`claude /login`, or Claude Code already set up
// on the machine running 37Studio from source), which works without any key in Settings.
func ClaudeLoggedIn() bool {
	home, _ := os.UserHomeDir()
	_, err := os.Stat(filepath.Join(home, ".claude", ".credentials.json"))
	return err == nil
}

// ClaudeReady is checked before starting Claude, so a missing key reads as a clear message instead of Claude Code's "Please run /login".
func ClaudeReady() error {
	if os.Getenv("ANTHROPIC_API_KEY") != "" || os.Getenv("CLAUDE_CODE_OAUTH_TOKEN") != "" || ClaudeLoggedIn() {
		return nil
	}
	return httpapi.Bad(noClaude)
}

// ClaudeHint turns Claude Code's sign-in and billing errors into what to do about them; "" when it's something else.
func ClaudeHint(msg string) string {
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
