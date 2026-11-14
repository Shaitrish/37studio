package integrations

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/remote"
	"github.com/Shaitrish/37studio/internal/store"
)

func put(t *testing.T, id, body string) (any, error) {
	t.Helper()
	r := httptest.NewRequest("PUT", "/api/integrations/"+id, strings.NewReader(body))
	r.SetPathValue("service", id)
	return Put(r)
}

// A service is saved from what the assistant read in its docs, and every call it signs stays on that service's host.
func TestIntegrations(t *testing.T) {
	config.DataDir = t.TempDir()
	store.Open()
	t.Cleanup(store.Close)

	for _, bad := range []struct{ id, body string }{
		{"Cartesia", `{"base_url":"https://api.cartesia.ai"}`},                           // id must be lowercase
		{"cartesia", `{"base_url":"http://api.cartesia.ai"}`},                            // not https
		{"cartesia", `{"base_url":"https://api.cartesia.ai","auth":"X-API-Key"}`},        // no {key}
		{"cartesia", `{"base_url":"https://api.cartesia.ai","kinds":["voice","pizza"]}`}, // unknown kind
	} {
		if _, err := put(t, bad.id, bad.body); err == nil {
			t.Errorf("%s %s saved", bad.id, bad.body)
		}
	}
	s, err := put(t, "cartesia", `{"name":"Cartesia","base_url":"https://api.cartesia.ai/","auth":"X-API-Key: {key}",
	  "headers":{"Cartesia-Version":"2025-04-16"},"kinds":["voice"],"prefer":true,"key":"sk_car_secret1234","unit":"1,000 characters","unit_price":0.04}`)
	if err != nil {
		t.Fatal(err)
	}
	if m := s.(map[string]any); m["last4"] != "1234" || m["key"] != nil || m["base_url"] != "https://api.cartesia.ai" {
		t.Fatalf("saved = %v", m)
	}
	var sealed string
	store.DB.QueryRow("select key from integrations where id='cartesia'").Scan(&sealed)
	if strings.Contains(sealed, "secret") {
		t.Fatal("key stored in the clear")
	}

	req, err := Request("cartesia", "post", "/tts/bytes", nil, "application/json")
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "POST" || req.URL.String() != "https://api.cartesia.ai/tts/bytes" || req.Header.Get("X-API-Key") != "sk_car_secret1234" || req.Header.Get("Cartesia-Version") != "2025-04-16" {
		t.Fatalf("request = %s %s %v", req.Method, req.URL, req.Header)
	}
	for _, p := range []string{"//evil.com/x", "voices", "@evil.com/x"} {
		if req, err := Request("cartesia", "GET", p, nil, ""); err == nil {
			t.Errorf("%q went to %s", p, req.URL)
		}
	}
	if _, err := Request("nope", "GET", "/", nil, ""); err == nil {
		t.Error("an unknown service made a request")
	}
	if p := Prompt(); !strings.Contains(p, "37studio api cartesia; for voice; the editor's choice") || strings.Contains(p, "secret") {
		t.Fatalf("prompt = %s", p)
	}

	// Changing only the guide keeps everything else; moving to another host drops the key.
	if _, err := put(t, "cartesia", `{"guide":"Use sonic-3."}`); err != nil {
		t.Fatal(err)
	}
	if s := All()[0]; s["guide"] != "Use sonic-3." || s["set"] != true || s["prefer"] != true {
		t.Fatalf("after guide change = %v", s)
	}
	if s, _ := put(t, "cartesia", `{"base_url":"https://evil.example.com"}`); s.(map[string]any)["set"] != false {
		t.Fatal("the key followed the service to another host")
	}

	// A query-parameter key, and a quick call that comes back as text.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") != "k1" {
			w.WriteHeader(401)
			w.Write([]byte(`{"error":{"message":"bad key"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"voices":[{"id":"v1"}]}`))
	}))
	defer srv.Close()
	old := remote.Client
	remote.Client = srv.Client()
	defer func() { remote.Client = old }()
	if _, err := put(t, "local", `{"base_url":"`+srv.URL+`","auth":"?key={key}","key":"k1"}`); err != nil {
		t.Fatal(err)
	}
	call := func(path string) (any, error) {
		r := httptest.NewRequest("POST", "/", strings.NewReader(`{"path":"`+path+`"}`))
		r.SetPathValue("service", "local")
		return Call(r)
	}
	if out, err := call("/voices?limit=2"); err != nil || !strings.Contains(out.(map[string]any)["body"].(string), "v1") {
		t.Fatalf("call = %v, %v", out, err)
	}
	put(t, "local", `{"key":"wrong"}`)
	if _, err := call("/voices"); err == nil || !strings.Contains(err.Error(), "bad key") {
		t.Fatalf("a refused call said %v", err)
	}
}
