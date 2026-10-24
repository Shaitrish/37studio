// Package config reads 37Studio's settings from the environment and .env.
package config

import (
	"os"
	"strings"
	"sync"
)

var DataDir = Env("STUDIO37_DATA", "data")

var Addr = Env("STUDIO37_ADDR", "127.0.0.1:8080") // this computer only; Docker sets :8080 and publishes it on 127.0.0.1

var WebDir = Env("STUDIO37_WEB", "web/dist")

var HFVersion = Env("HF_VERSION", "0.8.186")

var ClaudeBin = Env("STUDIO37_CLAUDE", "claude")

var dotenv sync.Once

// Env reads a setting. A .env in the working directory fills in variables the environment lacks, so keys added
// there apply on the next server restart (air restarts on every .go change) without restarting dev.sh.
func Env(k, def string) string {
	dotenv.Do(func() {
		b, _ := os.ReadFile(".env")
		for _, line := range strings.Split(string(b), "\n") {
			if key, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok && !strings.HasPrefix(key, "#") && os.Getenv(key) == "" {
				os.Setenv(key, strings.Trim(v, `"'`))
			}
		}
	})
	if v := os.Getenv(k); v != "" {
		return v
	}
	if rest, ok := strings.CutPrefix(k, "STUDIO37_"); ok { // pre-rename name, so older .env files and launchers keep working
		return Env("VIDBIT_"+rest, def)
	}
	return def
}

var PublicURL = strings.TrimRight(Env("STUDIO37_PUBLIC_URL", "http://localhost:8080"), "/") // where OAuth providers send the browser back

// RelayURL is the user's own WhatsApp webhook relay (relay/); empty means WhatsApp isn't set up. STUDIO37_AUTH_URL is
// its name from before 0.6.0, when copies shared one relay.
var RelayURL = strings.TrimRight(Env("STUDIO37_RELAY_URL", Env("STUDIO37_AUTH_URL", "")), "/")
