package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

var db *sql.DB

const schema = `
create table if not exists tenants(
  id integer primary key,
  name text not null,
  contact text not null default '',
  email text not null default '',
  phone text not null default '',
  website text not null default '',
  teacher text not null default '',
  bio text not null default '',
  language text not null default 'en',
  brand_color text not null default '#5f47d7',
  accent_color text not null default '#f5b83d',
  font text not null default '',
  notes text not null default '',
  margin_pct real,
  created_at text not null default current_timestamp
);
create table if not exists projects(
  id integer primary key,
  tenant_id integer not null references tenants(id),
  title text not null,
  kind text not null default 'custom',
  aspect text not null default '9:16',
  seconds integer not null default 60,
  quality text not null default 'standard',
  brief text not null default '',
  status text not null default 'draft',
  session_id text not null default '',
  invoice_id integer,
  created_at text not null default current_timestamp
);
create table if not exists campaigns(
  id integer primary key,
  tenant_id integer not null references tenants(id),
  name text not null,
  brief text not null default '',
  created_at text not null default current_timestamp
);
create table if not exists events(
  id integer primary key,
  project_id integer not null,
  kind text not null,
  body text not null,
  created_at text not null default current_timestamp
);
create index if not exists events_project on events(project_id, id);
create table if not exists usage(
  id integer primary key,
  project_id integer not null,
  tenant_id integer not null,
  provider text not null,
  item text not null,
  quantity real not null default 0,
  cost real not null,
  created_at text not null default current_timestamp
);
create table if not exists settings(key text primary key, value text not null);
create table if not exists secrets(name text primary key, value blob not null);
create table if not exists templates(
  id integer primary key,
  name text not null,
  notes text not null default '',
  tenant_id integer references tenants(id) on delete cascade, -- null: every company
  project_id integer,
  kind text not null default '',
  aspect text not null default '',
  seconds integer not null default 0,
  created_at text not null default current_timestamp
);
create table if not exists autopilots(
  id integer primary key,
  tenant_id integer not null references tenants(id) on delete cascade,
  name text not null default '',
  enabled integer not null default 1,
  brief text not null default '',
  kind text not null default 'promo',
  aspect text not null default '9:16',
  seconds integer not null default 30,
  language text not null default '',
  quality text not null default 'standard',
  days text not null default '0,2,4',
  at text not null default '20:00',
  accounts text not null default '[]',
  approval integer not null default 1,
  last_slot text not null default '',
  created_at text not null default current_timestamp
);
create table if not exists social_accounts(
  id integer primary key,
  tenant_id integer not null references tenants(id) on delete cascade,
  platform text not null,
  account_id text not null,
  name text not null default '',
  picture text not null default '',
  token text not null default '',
  refresh text not null default '',
  created_at text not null default current_timestamp,
  unique(tenant_id, platform, account_id)
);
create table if not exists wa_messages(
  id text primary key,
  account_id integer not null references social_accounts(id) on delete cascade,
  customer text not null,
  name text not null default '',
  mine integer not null default 0,
  text text not null,
  at text not null,
  seq integer
);
create index if not exists wa_messages_thread on wa_messages(account_id, customer);
create table if not exists inbox_drafts(
  account_id integer not null references social_accounts(id) on delete cascade,
  item text not null,
  kind text not null,
  answers text not null,
  text text not null,
  status text not null,
  error text not null default '',
  at text not null default current_timestamp,
  primary key(account_id, item)
);
create table if not exists posts(
  id integer primary key,
  project_id integer not null,
  account_id integer not null,
  platform text not null,
  account_name text not null,
  file text not null,
  title text not null default '',
  caption text not null default '',
  privacy text not null default '',
  status text not null default 'scheduled',
  publish_at text not null,
  remote_id text not null default '',
  url text not null default '',
  error text not null default '',
  created_at text not null default current_timestamp
);
create index if not exists posts_due on posts(status, publish_at);
`

func openDB() {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatal(err)
	}
	path := filepath.Join(dataDir, "37studio.db")
	if _, err := os.Stat(path); os.IsNotExist(err) { // ponytail: one-time move from the pre-rename name; drop once every copy has started a build that has it
		for _, s := range []string{"", "-wal", "-shm"} {
			if err := os.Rename(filepath.Join(dataDir, "vidbit.db"+s), path+s); err != nil && !os.IsNotExist(err) {
				log.Fatal(err)
			}
		}
	}
	var err error
	// WAL + busy_timeout: `37studio gen` runs as a separate process and writes usage rows concurrently.
	db, err = sql.Open("sqlite", path+"?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err == nil {
		_, err = db.Exec(schema)
	}
	if err != nil {
		log.Fatal(err)
	}
	// Additive migrations; "duplicate column" errors on an up-to-date DB are expected and ignored.
	for _, m := range []string{
		"alter table tenants add column scan_status text not null default ''",
		"alter table tenants add column scan_note text not null default ''",
		"alter table projects add column campaign_id integer references campaigns(id)",
		"alter table projects add column focus text not null default ''",
		"alter table projects add column language text not null default ''",
		"alter table usage add column file text not null default ''",
		"alter table projects add column oc_session text not null default ''",
		"alter table projects add column autopilot_id integer",
		"alter table projects add column autopilot_slot text not null default ''",
		"alter table projects add column autopilot_stage text not null default ''",
		"alter table projects add column parent_id integer",
		"alter table projects add column clip_stage text not null default ''",
		"alter table projects add column template_id integer",
		"alter table tenants add column inbox_mode text not null default 'off'",
	} {
		db.Exec(m)
	}
}

// rows returns query results as JSON-ready maps; the API is thin enough that structs would only add ceremony.
func rows(q string, args ...any) ([]map[string]any, error) {
	rs, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	cols, _ := rs.Columns()
	out := []map[string]any{}
	for rs.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			if b, ok := vals[i].([]byte); ok {
				vals[i] = string(b)
			}
			m[c] = vals[i]
		}
		out = append(out, m)
	}
	return out, rs.Err()
}

func row(q string, args ...any) (map[string]any, error) {
	rs, err := rows(q, args...)
	if err != nil {
		return nil, err
	}
	if len(rs) == 0 {
		return nil, sql.ErrNoRows
	}
	return rs[0], nil
}

// save inserts (id == 0) or updates a row from a JSON body, touching only whitelisted columns.
func save(table string, cols []string, id int64, b map[string]any) (int64, error) {
	var set []string
	var args []any
	for _, c := range cols {
		if v, ok := b[c]; ok {
			set = append(set, c)
			args = append(args, v)
		}
	}
	if id == 0 {
		q := fmt.Sprintf("insert into %s(%s) values(%s)", table, strings.Join(set, ","), strings.TrimSuffix(strings.Repeat("?,", len(set)), ","))
		res, err := db.Exec(q, args...)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}
	if len(set) == 0 {
		return id, nil
	}
	q := fmt.Sprintf("update %s set %s=? where id=?", table, strings.Join(set, "=?,"))
	_, err := db.Exec(q, append(args, id)...)
	return id, err
}

func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func num(v any) float64 {
	switch x := v.(type) {
	case int64:
		return float64(x)
	case float64:
		return x
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	}
	return 0
}

var settingDefaults = map[string]string{
	"run_budget_usd":      "25",
	"image_model":         "gemini-nano-banana-2.1",
	"image_price":         "0.0504",
	"video_model":         "gemini-omni-1.1-flash",
	"video_price_per_sec": "0.10",
	"voice_model":         "eleven_v4",
	"voice_default":       "Xb7hH8MSUJpSbSDYk0k2", // Alice, ElevenLabs premade
	"voice_bn":            "70QpbCWFDvTpWo8ZKOUb", // Niharika, energetic Bengali creator
	"voice_price_per_1k":  "0.20",
	"sfx_price_per_sec":   "0.01",
	"music_price_per_min": "0.50",
	"render_workers":      "auto",
	"render_gpu":          "false",
	"render_fast_capture": "false",
	"render_draft_fps":    "15",
	"currency":            "USD",
	"timezone":            "UTC", // replaced by the browser's zone when an autopilot plan is saved
	"fx_rate":             "1",
}

func setting(k string) string {
	var v string
	if db.QueryRow("select value from settings where key=?", k).Scan(&v) != nil {
		return settingDefaults[k]
	}
	return v
}

func settingF(k string) float64 { return num(setting(k)) }

func getSettings(r *http.Request) (any, error) {
	out := map[string]string{}
	for k := range settingDefaults {
		out[k] = setting(k)
	}
	return out, nil
}

func putSettings(r *http.Request) (any, error) {
	b, err := body(r)
	if err != nil {
		return nil, err
	}
	for k, v := range b {
		if _, ok := settingDefaults[k]; !ok {
			continue
		}
		if _, err := db.Exec("insert into settings(key,value) values(?,?) on conflict(key) do update set value=excluded.value", k, str(v)); err != nil {
			return nil, err
		}
	}
	return getSettings(r)
}
