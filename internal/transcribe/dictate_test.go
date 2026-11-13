package transcribe

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Shaitrish/37studio/internal/httpapi"
)

// The worker protocol end to end, with a fake Python that echoes how many samples it got: drafts and finals reach
// it, an empty body only starts it, and odd or oversized uploads are the user's error (400), not the server's.
func TestDictate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake Python is a shell script")
	}
	fake := filepath.Join(t.TempDir(), "python")
	os.WriteFile(fake, []byte(`#!/bin/sh
echo '{"ready": true}'
while read -r head; do
  n=$(echo "$head" | sed 's/.*"n":\([0-9]*\).*/\1/')
  final=$(echo "$head" | grep -c '"final":true')
  head -c "$n" >/dev/null
  echo "{\"text\": \"$((n / 2)) samples, final $final\"}"
done
`), 0o755)
	t.Setenv("STUDIO37_ASR_PYTHON", fake)
	t.Cleanup(func() {
		if dictation.w != nil {
			dictation.drop(dictation.w)
		}
	})

	post := func(query string, body []byte) (any, error) {
		return Dictate(httptest.NewRequest("POST", "/api/dictate"+query, bytes.NewReader(body)))
	}
	text := func(v any) string { b, _ := json.Marshal(v); return string(b) }

	if v, err := post("", nil); err != nil || text(v) != `{"ok":true}` {
		t.Fatalf("warm: %v %v", text(v), err)
	}
	if v, err := post("?final=1", make([]byte, 3200)); err != nil || text(v) != `{"text":"1600 samples, final 1"}` {
		t.Fatalf("final: %v %v", text(v), err)
	}
	if v, err := post("", make([]byte, 64)); err != nil || text(v) != `{"text":"32 samples, final 0"}` {
		t.Fatalf("draft: %v %v", text(v), err)
	}
	bad := func(err error) bool { _, ok := err.(httpapi.Error); return ok }
	if _, err := post("?final=1", make([]byte, 3)); !bad(err) {
		t.Fatalf("odd bytes: %v", err)
	}
	if _, err := Dictate(httptest.NewRequest("POST", "/api/dictate", io.LimitReader(zeros{}, maxClip+2))); !bad(err) || !strings.Contains(err.Error(), "too long") {
		t.Fatalf("too long: %v", err)
	}
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) { clear(p); return len(p), nil }
