package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
		err := call(get(srv.URL, ""), &out)
		srv.Close()
		if c.wantErr == "" && err != nil {
			t.Errorf("%s: unexpected error %v", c.body, err)
		}
		if c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)) {
			t.Errorf("%s: got %v, want error containing %q", c.body, err, c.wantErr)
		}
	}
}

// An app ID only switches logins to the user's own Meta app together with its secret, and agents never get the secret.
func TestMetaApp(t *testing.T) {
	t.Setenv("META_APP_ID", "111")
	t.Setenv("META_APP_SECRET", "")
	if id, c := metaApp(); id != sharedMetaApp || c != sharedMetaConfig {
		t.Fatalf("ID without secret: got %s %s, want the shared app", id, c)
	}
	t.Setenv("META_APP_SECRET", "s3cret")
	t.Setenv("META_CONFIG_ID", "")
	if id, c := metaApp(); id != "111" || c != "" {
		t.Fatalf("own app: got %s %q", id, c)
	}
	for _, kv := range agentEnv() {
		if strings.HasPrefix(kv, "META_APP_SECRET=") {
			t.Fatal("agents got the Meta app secret")
		}
	}
}
