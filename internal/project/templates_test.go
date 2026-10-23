package project

import (
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Shaitrish/37studio/internal/agent"
	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/templates"
)

// Saving a template copies the composition (not its HyperFrames history); a project made from it starts with that
// composition, and its first prompt says to keep the design and swap the content.
func TestTemplates(t *testing.T) {
	config.DataDir = t.TempDir()
	store.Open()
	store.DB.Exec("insert into tenants(id,name) values(1,'Shop')")
	store.DB.Exec("insert into projects(id,tenant_id,title,kind,aspect,seconds) values(1,1,'Eid promo','promo','9:16',30)")
	comp := filepath.Join(files.ProjectDir(1), "comp")
	os.MkdirAll(filepath.Join(comp, "assets"), 0o755)
	os.MkdirAll(filepath.Join(comp, ".hyperframes"), 0o755)
	os.WriteFile(filepath.Join(comp, "index.html"), []byte("<html>eid</html>"), 0o644)
	os.WriteFile(filepath.Join(comp, "assets", "logo.svg"), []byte("<svg/>"), 0o644)
	os.WriteFile(filepath.Join(comp, ".hyperframes", "history.json"), []byte("{}"), 0o644)

	r := httptest.NewRequest("POST", "/api/projects/1/template", strings.NewReader(`{"name":"Promo look","notes":"keep the intro"}`))
	r.SetPathValue("id", "1")
	tpl, err := templates.Save(r)
	if err != nil {
		t.Fatal(err)
	}
	tid := tpl.(map[string]any)["id"].(int64)
	if _, err := os.Stat(filepath.Join(templates.Dir(tid), "comp", ".hyperframes")); err == nil {
		t.Fatal("the old project's history was copied")
	}

	p, err := Save(httptest.NewRequest("POST", "/api/projects", strings.NewReader(fmt.Sprintf(`{"tenant_id":1,"title":"Puja promo","template_id":%d}`, tid))))
	if err != nil {
		t.Fatal(err)
	}
	np := p.(map[string]any)
	if b, _ := os.ReadFile(filepath.Join(files.ProjectDir(np["id"].(int64)), "comp", "index.html")); string(b) != "<html>eid</html>" {
		t.Fatalf("new project's comp/index.html = %q", b)
	}
	if fp := agent.FirstPrompt(np); !strings.Contains(fp, `template "Promo look"`) || !strings.Contains(fp, "keep the intro") {
		t.Fatalf("first prompt lacks the template note:\n%s", fp)
	}
}
