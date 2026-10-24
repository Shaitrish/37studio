// Package httpapi holds the JSON API's plumbing: handler adapter, errors, request helpers and basic auth.
package httpapi

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Shaitrish/37studio/internal/config"
)

// Guard wraps the whole app. Other websites' pages may not make changes: a browser resends saved basic auth, and the
// app on localhost needs none, so a cross-site form POST would otherwise act as the user. Non-browser clients (MCP,
// curl) send neither Sec-Fetch-Site nor Origin and pass. With STUDIO37_PASSWORD set it also needs basic auth (any username).
func Guard(h http.Handler) http.Handler {
	h = http.NewCrossOriginProtection().Handler(h)
	pw := config.Env("STUDIO37_PASSWORD", "")
	if pw == "" {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/webhooks/meta" { // Meta can't send the password; the handler checks Meta's signature instead
			h.ServeHTTP(w, r)
			return
		}
		if _, p, ok := r.BasicAuth(); !ok || subtle.ConstantTimeCompare([]byte(p), []byte(pw)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="37studio"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

type Error struct {
	Code int
	Msg  string
}

func (e Error) Error() string { return e.Msg }

func Bad(msg string) error { return Error{Code: http.StatusBadRequest, Msg: msg} }

func JSON(fn func(r *http.Request) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, err := fn(r)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			code := http.StatusInternalServerError
			var ae Error
			if errors.As(err, &ae) {
				code = ae.Code
			} else if errors.Is(err, sql.ErrNoRows) {
				code = http.StatusNotFound
			}
			w.WriteHeader(code)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(v)
	}
}

func PathID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id
}

func Body(r *http.Request) (map[string]any, error) {
	m := map[string]any{}
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		return nil, Bad("invalid JSON body")
	}
	return m, nil
}
