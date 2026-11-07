package company

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/store"
)

// Putting a logo first makes it the brand's icon: the brand list shows the first image in logo/.
func TestPutLogoFirst(t *testing.T) {
	config.DataDir = t.TempDir()
	store.Open()
	t.Cleanup(store.Close)
	store.DB.Exec("insert into tenants(id,name) values(1,'Shop')")
	dir := filepath.Join(files.TenantDir(1), "logo")
	os.MkdirAll(dir, 0o755)
	old := time.Now().Add(-time.Hour)
	for i, name := range []string{"app-icon.png", "mark.svg"} {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte("x"), 0o644)
		at := old.Add(time.Duration(i) * time.Minute) // mark.svg is the newer, so it's the icon
		os.Chtimes(p, at, at)
	}
	icon := func() string {
		rs, _ := List(httptest.NewRequest("GET", "/api/tenants", nil))
		return store.Str(rs.([]map[string]any)[0]["logo"])
	}
	if !strings.HasSuffix(icon(), "/mark.svg") {
		t.Fatalf("before: icon %s", icon())
	}
	r := httptest.NewRequest("PUT", "/api/tenants/1/assets", strings.NewReader(`{"path":"logo/app-icon.png","first":true}`))
	r.SetPathValue("id", "1")
	if _, err := UpdateAsset(r); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(icon(), "/app-icon.png") {
		t.Fatalf("after: icon %s", icon())
	}
}
