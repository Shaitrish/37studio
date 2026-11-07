package source

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/store"
)

// A connected folder is read where it lies, unlike source/, which is a filtered copy. It suits a project too big to
// copy that keeps changing, and gives the agent the brand's real lessons, images and data.

// Connected is the brand's live folder, or "" if it has none or the folder is gone.
func Connected(tenant int64) string {
	t, err := store.Row("select source_path from tenants where id=?", tenant)
	if err != nil {
		return ""
	}
	dir := store.Str(t["source_path"])
	if st, err := os.Stat(dir); dir == "" || err != nil || !st.IsDir() {
		return ""
	}
	return dir
}

// Databases are the SQLite files in that folder the agent may query, relative to it. A folder can hold a user database
// beside the one a video needs, so nothing is queryable until the owner names it.
func Databases(tenant int64) []string {
	t, err := store.Row("select source_dbs from tenants where id=?", tenant)
	if err != nil {
		return nil
	}
	return splitDatabases(store.Str(t["source_dbs"]))
}

func splitDatabases(s string) []string {
	var out []string
	for _, d := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' }) {
		if d = path.Clean(filepath.ToSlash(strings.TrimSpace(d))); d != "." {
			out = append(out, d)
		}
	}
	return out
}

// CheckFolder is the brand form's check, so a mistyped path is caught when it's saved, not during an agent's turn.
func CheckFolder(dir, dbs string) error {
	if dir == "" {
		if dbs != "" {
			return errors.New("choose a connected folder before naming its databases")
		}
		return nil
	}
	if !filepath.IsAbs(dir) {
		return errors.New("the connected folder must be a full path, like /home/you/my-product")
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return fmt.Errorf("can't open the folder %s here", dir)
	}
	for _, d := range splitDatabases(dbs) {
		if st, err := os.Stat(files.Within(dir, d)); err != nil || st.IsDir() {
			return fmt.Errorf("can't find the database %s in that folder", d)
		}
	}
	return nil
}

// secretGlobs mirror secretFile. Databases are on the list too: the agent reaches the ones it may use through
// `37studio sql`, not as raw files.
var secretGlobs = []string{".env*", "*.pem", "*.key", "*.p12", "*.pfx", "*.jks", "*.keystore", "*.kdbx", "*.db", "*.sqlite*", "id_rsa*", "id_ed25519*", "*secret*", "*credential*", "*service-account*"}

// FolderArgs lets Claude Code read the folder and nothing more: no writes in it, no secrets out of it. Like safeTools
// this narrows what the agent does by accident; it isn't a security boundary.
func FolderArgs(dir string) []string {
	root := "/" + filepath.ToSlash(dir)                // an absolute path in a permission rule starts with //
	deny := []string{fmt.Sprintf("Edit(%s/**)", root)} // Edit rules cover the Write tool too
	for _, g := range secretGlobs {
		deny = append(deny, fmt.Sprintf("Read(%s/**/%s)", root, g))
	}
	for _, c := range []string{"cat", "head", "tail", "cp"} {
		deny = append(deny, fmt.Sprintf("Bash(%s *.env*)", c))
	}
	return []string{"--add-dir", dir, "--disallowedTools", strings.Join(deny, ",")}
}

const sqlRows, sqlCell = 200, 2000

var selectOnly = regexp.MustCompile(`(?i)^(select|with)\b`)

// Query runs one read-only statement on a database the owner named. The rows and cells are capped, because they go
// straight into the agent's context.
func Query(tenant int64, db, query string) ([]map[string]any, error) {
	dir := Connected(tenant)
	if dir == "" {
		return nil, errors.New("this brand has no connected folder")
	}
	allowed := Databases(tenant)
	name := path.Clean(filepath.ToSlash(db))
	if !slices.Contains(allowed, name) {
		return nil, fmt.Errorf("%s isn't one of the brand's databases: %s", db, strings.Join(allowed, ", "))
	}
	q := strings.TrimRight(strings.TrimSpace(query), "; \n\t")
	if !selectOnly.MatchString(q) || strings.Contains(q, ";") {
		return nil, errors.New("only one select statement at a time")
	}
	// SQLite wants file:///C:/… on Windows: a path without a leading slash would read as a host ("C:").
	u := url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(filepath.ToSlash(files.Within(dir, name)), "/")}
	conn, err := sql.Open("sqlite", u.String()+"?mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	rows, err := conn.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	vals, ptrs := make([]any, len(cols)), make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() && len(out) < sqlRows {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := map[string]any{}
		for i, c := range cols {
			v := vals[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			if s, ok := v.(string); ok && len(s) > sqlCell {
				v = strings.ToValidUTF8(s[:sqlCell], "") + "…"
			}
			row[c] = v
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// SQLCLI is `37studio sql <database> "<select>"`, the agent's read-only way into a brand's database: one JSON object
// a line. A built-in command, so it works where python and sqlite3 aren't installed or aren't allowed.
func SQLCLI(args []string) int {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, `usage: 37studio sql <database> "select ..."   (the database's path inside the brand's connected folder)`)
		return 2
	}
	config.DataDir, _ = filepath.Abs(config.DataDir)
	store.Open()
	pid, _ := strconv.ParseInt(config.Env("STUDIO37_PROJECT", ""), 10, 64)
	p, err := store.Row("select tenant_id from projects where id=?", pid)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: run this inside a project (STUDIO37_PROJECT is not set)")
		return 1
	}
	rows, err := Query(p["tenant_id"].(int64), args[0], args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	for _, r := range rows {
		enc.Encode(r)
	}
	if len(rows) == sqlRows {
		fmt.Fprintf(os.Stderr, "stopped at %d rows: narrow the query\n", sqlRows)
	}
	return 0
}
