package social

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// A sign-in started from the desktop window ends back in that window; one started in a browser redirects as before.
func TestOAuthHandoff(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
	var back string
	AfterSignIn = func(target string) { back = target }
	t.Cleanup(func() { AfterSignIn = nil })

	signIn := func(handoff string) *httptest.ResponseRecorder {
		t.Helper()
		start := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/oauth/google/start?tenant=5&return=http://127.0.0.1:8080/"+handoff, nil)
		req.SetPathValue("provider", "google")
		OAuthStart(start, req)
		loc, err := url.Parse(start.Header().Get("Location"))
		if start.Code != http.StatusFound || err != nil {
			t.Fatalf("start: %d %v", start.Code, start.Body)
		}
		// The person said no at Google: no token exchange, the same path to the account picker.
		cb := httptest.NewRecorder()
		req = httptest.NewRequest("GET", "/api/oauth/google/callback?error=access_denied&state="+loc.Query().Get("state"), nil)
		req.SetPathValue("provider", "google")
		OAuthCallback(cb, req)
		return cb
	}

	res := signIn("&handoff=1")
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "Go back to 37Studio") {
		t.Fatalf("desktop sign-in: %d %s", res.Code, res.Body)
	}
	if !strings.HasPrefix(back, "http://127.0.0.1:8080/#/clients/5/social/") {
		t.Fatalf("window sent to %q", back)
	}

	back = ""
	res = signIn("")
	if res.Code != http.StatusFound || !strings.Contains(res.Header().Get("Location"), "#/clients/5/social/") || back != "" {
		t.Fatalf("browser sign-in: %d %q, window sent to %q", res.Code, res.Header().Get("Location"), back)
	}
}
