package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Another site's page can't make changes; the app itself, MCP clients and reads can.
func TestGuardCrossOrigin(t *testing.T) {
	h := Guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for _, c := range []struct {
		method, header, value string
		want                  int
	}{
		{"POST", "Sec-Fetch-Site", "cross-site", 403},
		{"POST", "Origin", "https://evil.example", 403},
		{"POST", "Sec-Fetch-Site", "same-origin", 200},
		{"POST", "Origin", "http://localhost:8080", 200},
		{"POST", "", "", 200}, // MCP clients and curl
		{"GET", "Sec-Fetch-Site", "cross-site", 200},
	} {
		r := httptest.NewRequest(c.method, "http://localhost:8080/api/projects/1/approve", nil)
		if c.header != "" {
			r.Header.Set(c.header, c.value)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != c.want {
			t.Errorf("%s %s=%s: %d, want %d", c.method, c.header, c.value, w.Code, c.want)
		}
	}
}

// With a password set, Meta's webhook still gets through (its handler checks Meta's signature); nothing else does.
func TestGuardWebhookSkipsPassword(t *testing.T) {
	t.Setenv("STUDIO37_PASSWORD", "pw")
	h := Guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for path, want := range map[string]int{"/webhooks/meta": 200, "/api/projects": 401} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", "http://localhost:8080"+path, nil))
		if w.Code != want {
			t.Errorf("%s: %d, want %d", path, w.Code, want)
		}
	}
}
