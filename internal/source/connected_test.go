package source

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/store"
)

func connect(t *testing.T) string {
	t.Helper()
	config.DataDir = t.TempDir()
	store.Open()
	t.Cleanup(store.Close)
	dir := filepath.Join(t.TempDir(), "my product")
	os.MkdirAll(filepath.Join(dir, "data"), 0o755)
	for _, n := range []string{"bank.db", "users.db"} {
		db, err := sql.Open("sqlite", filepath.Join(dir, "data", n))
		if err != nil {
			t.Fatal(err)
		}
		db.Exec("create table q(id integer, stem text)")
		db.Exec("insert into q values(1,'ক্ষুদ্রতম মহাদেশ'),(2,?)", strings.Repeat("ক", 3000))
		db.Close()
	}
	store.DB.Exec("insert into tenants(id,name,source_path,source_dbs) values(1,'Shop',?,'data/bank.db')", dir)
	return dir
}

// A named database answers selects; everything else is refused, and the file stays as it was.
func TestQuery(t *testing.T) {
	dir := connect(t)
	rows, err := Query(1, "./data/bank.db", "select stem from q where id=1;")
	if err != nil || len(rows) != 1 || rows[0]["stem"] != "ক্ষুদ্রতম মহাদেশ" {
		t.Fatal(rows, err)
	}
	long, _ := Query(1, "data/bank.db", "select stem from q where id=2")
	if s := long[0]["stem"].(string); len(s) > sqlCell+4 || !strings.HasSuffix(s, "…") {
		t.Fatal("a long cell wasn't cut", len(s))
	}
	for _, bad := range [][2]string{
		{"data/users.db", "select * from q"}, {"../data/users.db", "select * from q"},
		{"data/bank.db", "delete from q"}, {"data/bank.db", "select 1; delete from q"},
		{"data/bank.db", "with x as (select 1) delete from q"}, {"data/bank.db", "attach '/etc/passwd' as p"},
	} {
		if _, err := Query(1, bad[0], bad[1]); err == nil {
			t.Fatalf("%v went through", bad)
		}
	}
	if rows, _ := Query(1, "data/bank.db", "select count(*) n from q"); rows[0]["n"] != int64(2) {
		t.Fatal("rows changed", rows)
	}
	store.DB.Exec("update tenants set source_path=? where id=1", filepath.Join(dir, "gone"))
	if _, err := Query(1, "data/bank.db", "select 1"); err == nil {
		t.Fatal("a missing folder went through")
	}
}

func TestCheckFolder(t *testing.T) {
	dir := connect(t)
	for _, c := range []struct {
		dir, dbs string
		ok       bool
	}{{"", "", true}, {dir, "", true}, {dir, "data/bank.db, data/users.db", true}, {"relative/path", "", false}, {dir + "x", "", false}, {dir, "data/none.db", false}, {"", "data/bank.db", false}} {
		if err := CheckFolder(c.dir, c.dbs); (err == nil) != c.ok {
			t.Errorf("%q %q: %v", c.dir, c.dbs, err)
		}
	}
}

// The agent may read the folder but must not write to it, and its keys stay out of reach.
func TestFolderArgs(t *testing.T) {
	a := FolderArgs("/work/app")
	if a[0] != "--add-dir" || a[1] != "/work/app" || a[2] != "--disallowedTools" {
		t.Fatal(a)
	}
	for _, want := range []string{"Edit(//work/app/**)", "Read(//work/app/**/.env*)", "Read(//work/app/**/*.db)", "Bash(cat *.env*)"} {
		if !strings.Contains(a[3], want) {
			t.Error("missing", want)
		}
	}
}
