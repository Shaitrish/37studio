package social

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Shaitrish/37studio/internal/agent"
	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/remote"
)

// callWith must tell success from failure across Meta, Google and TikTok reply styles, and surface the platform's message.
func TestCallWithErrors(t *testing.T) {
	cases := []struct {
		status        int
		body, wantErr string
	}{
		{200, `{"id":"1"}`, ""},
		{200, `{"data":{"publish_id":"p"},"error":{"code":"ok","message":""}}`, ""}, // TikTok success
		{403, `{"error":{"code":"spam_risk_too_many_posts","message":"too many posts"}}`, "too many posts"},
		{400, `{"error":{"message":"Invalid OAuth access token","error_user_msg":"Session expired"}}`, "Session expired"},
		{400, `{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`, "expired or revoked"},
		{200, `{"error":"invalid_request","error_description":"Code verifier mismatch"}`, "Code verifier mismatch"}, // TikTok token endpoint
	}
	for _, c := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
			w.Write([]byte(c.body))
		}))
		var out map[string]any
		err := remote.Call(remote.Get(srv.URL, ""), &out)
		srv.Close()
		if c.wantErr == "" && err != nil {
			t.Errorf("%s: unexpected error %v", c.body, err)
		}
		if c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)) {
			t.Errorf("%s: got %v, want error containing %q", c.body, err, c.wantErr)
		}
	}
}

// Facebook needs the user's own Meta app, ID and secret both, and agents never get the secret.
func TestMetaApp(t *testing.T) {
	t.Setenv("META_APP_ID", "111")
	t.Setenv("META_APP_SECRET", "")
	if ProviderReady("meta") {
		t.Fatal("an app ID without its secret counts as set up")
	}
	t.Setenv("META_APP_SECRET", "s3cret")
	t.Setenv("META_CONFIG_ID", "")
	if id, c := metaApp(); id != "111" || c != "" {
		t.Fatalf("own app: got %s %q", id, c)
	}
	for _, kv := range agent.Env() {
		if strings.HasPrefix(kv, "META_APP_SECRET=") {
			t.Fatal("agents got the Meta app secret")
		}
	}
}

// An auto-reply that carries a link or number the brand never wrote down waits for the editor.
func TestUnsourced(t *testing.T) {
	config.DataDir = t.TempDir()
	os.MkdirAll(files.TenantRoot(3), 0o755)
	os.WriteFile(filepath.Join(files.TenantRoot(3), "knowledge.md"), []byte("Shop at https://course37.com/shop or call +880 1711-000000. Mail hello@course37.com."), 0o644)
	for reply, want := range map[string]string{
		"Thanks! A team member will reply soon.":                  "",
		"Order at https://course37.com/shop.":                     "",
		"Call +880 1711-000000 or mail hello@course37.com today!": "",
		"Our anniversary sale is live at https://evil.example/x":  "https://evil.example/x",
		"Grab it at bit.ly/sale50":                                "bit.ly/sale50",
		"WhatsApp us on 01999-123456":                             "01999-123456",
	} {
		if got := unsourced(reply, 3); got != want {
			t.Errorf("%q: got %q, want %q", reply, got, want)
		}
	}
}
