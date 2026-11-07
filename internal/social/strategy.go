package social

import (
	"cmp"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/textutil"
)

// A brand's content strategy lives in strategy.md next to knowledge.md. Claude rewrites its "What the numbers say"
// section from the Performance numbers on request, the editor agent adds dated notes with `37studio strategy add`,
// and the team edits it on the Performance page. Every new video's agent reads it.

const numbersHeading = "## What the numbers say"

func strategyPath(tenant int64) string { return filepath.Join(files.TenantRoot(tenant), "strategy.md") }

// setSection replaces the section under heading (up to the next "## " heading) or puts it at the top.
func setSection(doc, heading, body string) string {
	section := heading + "\n\n" + strings.TrimSpace(body) + "\n"
	i := strings.Index(doc, heading)
	if i < 0 {
		return strings.TrimRight(section+"\n"+strings.TrimSpace(doc), "\n") + "\n"
	}
	rest := doc[i+len(heading):]
	if j := strings.Index(rest, "\n## "); j >= 0 {
		return doc[:i] + section + rest[j:]
	}
	return doc[:i] + section
}

// addNote appends a dated line under "## Notes", making the section when it isn't there yet.
func addNote(doc, note string, at time.Time) string {
	line := fmt.Sprintf("- %s: %s\n", at.Format("2006-01-02"), strings.Join(strings.Fields(note), " "))
	doc = strings.TrimRight(doc, "\n")
	if !strings.Contains(doc, "\n## Notes") && !strings.HasPrefix(doc, "## Notes") {
		if doc != "" {
			doc += "\n\n"
		}
		return doc + "## Notes\n\n" + line
	}
	return doc + "\n" + line
}

var (
	reviewMu  sync.Mutex
	reviewing = map[int64]bool{}
	reviewErr = map[int64]string{}
)

// statsDigest is the numbers in a few compact lines per video, for a prompt.
func statsDigest(tenant int64) string {
	rows, _ := statsRows(tenant, "", "views", false, 150)
	n := func(v any) string {
		if v == nil {
			return "-"
		}
		return strconv.FormatFloat(store.Num(v), 'f', -1, 64)
	}
	var b strings.Builder
	b.WriteString("platform | posted | views | % watched | avg watch s | length s | engagement per 100 views | % left in first 3 s | 37Studio project | title\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "%s | %.10s | %s | %s | %s | %s | %s | %s | %s | %s\n", r["platform"], store.Str(r["published_at"]), n(r["views"]), n(r["retention"]),
			n(r["watch_avg"]), n(r["duration"]), n(r["engagement_rate"]), n(r["skip_rate"]), cmp.Or(store.Str(r["project"]), "-"),
			textutil.Cut(strings.Join(strings.Fields(store.Str(r["title"])), " "), 90))
	}
	return b.String()
}

var bullets = regexp.MustCompile(`(?s)^.*?(\n?[-*] )`)

// reviewStrategy has Claude turn the numbers into lessons and rewrites the "What the numbers say" section.
func reviewStrategy(tenant int64) {
	defer func() { reviewMu.Lock(); delete(reviewing, tenant); reviewMu.Unlock() }()
	fail := func(err error) { reviewMu.Lock(); reviewErr[tenant] = err.Error(); reviewMu.Unlock() }
	t, err := store.Row("select name from tenants where id=?", tenant)
	if err != nil {
		fail(err)
		return
	}
	prompt := fmt.Sprintf(`You are the content strategist for %s. Below are the platforms' own numbers for its published videos, and its current strategy notes.

Write the section "What the numbers say" of its strategy: 5 to 10 short bullet points. Each is one concrete lesson the editor can act on, with the evidence in brackets, for example: "- Map-and-history explainers far outperform exam tips on Instagram (avg 98K vs 21K views, 6 vs 4 videos)". Cover what topics and formats work, the hook and first 3 seconds (skip rate, %% watched), length (watch time against length), which platform responds best to what, and end with 2 or 3 things to try next. Only claim what the numbers support, and say when there are too few videos to tell. Write in English, keep titles in their own language when you quote them.

Reply with only the bullet list in Markdown, nothing before or after it.

Numbers:
%s
Current strategy:
%s`, t["name"], statsDigest(tenant), cmp.Or(textutil.Head(files.TenantDoc(tenant, "strategy.md"), 6000), "(none yet)"))
	text, cost, tokens, err := claudeOnce(files.TenantRoot(tenant), prompt)
	if cost > 0 {
		store.DB.Exec("insert into usage(project_id,tenant_id,provider,item,quantity,cost) values(0,?,?,?,?,?)", tenant, "anthropic", "strategy review", tokens, cost)
	}
	if err != nil {
		fail(err)
		return
	}
	if !strings.Contains(text, "- ") && !strings.Contains(text, "* ") {
		fail(fmt.Errorf("claude did not return a list: %s", textutil.Tail(text, 200)))
		return
	}
	body := strings.TrimSpace(bullets.ReplaceAllString(text, "$1")) // drop any preamble before the first bullet
	body = fmt.Sprintf("_Claude, from the Performance numbers on %s._\n\n%s", time.Now().Format("2 Jan 2006"), body)
	doc := setSection(files.TenantDoc(tenant, "strategy.md"), numbersHeading, body)
	if err := os.WriteFile(strategyPath(tenant), []byte(doc), 0o644); err != nil {
		fail(err)
		return
	}
	reviewMu.Lock()
	delete(reviewErr, tenant)
	reviewMu.Unlock()
}

// ReviewStrategy is POST /api/stats/review {tenant_id}: Claude rewrites the brand's "What the numbers say" in the
// background (about $0.05); GET /api/stats?tenant= shows reviewing until it's done.
func ReviewStrategy(r *http.Request) (any, error) {
	b, _ := httpapi.Body(r)
	tenant := int64(store.Num(b["tenant_id"]))
	if _, err := store.Row("select id from tenants where id=?", tenant); err != nil {
		return nil, httpapi.Bad("pick a brand first")
	}
	reviewMu.Lock()
	defer reviewMu.Unlock()
	if !reviewing[tenant] {
		reviewing[tenant] = true
		delete(reviewErr, tenant)
		go reviewStrategy(tenant)
	}
	return map[string]any{"reviewing": true}, nil
}

func reviewState(tenant int64) (bool, string) {
	reviewMu.Lock()
	defer reviewMu.Unlock()
	return reviewing[tenant], reviewErr[tenant]
}

// StrategyCLI is `37studio strategy`: print the brand's strategy, `add "note"` to record a lesson, `set <file>` to
// replace it whole. The editor agent runs it inside a project.
func StrategyCLI(args []string) int {
	config.DataDir, _ = filepath.Abs(config.DataDir)
	store.Open()
	pid, _ := strconv.ParseInt(config.Env("STUDIO37_PROJECT", ""), 10, 64)
	p, err := store.Row("select tenant_id from projects where id=?", pid)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: run this inside a project (STUDIO37_PROJECT is not set)")
		return 1
	}
	tenant := p["tenant_id"].(int64)
	doc := files.TenantDoc(tenant, "strategy.md")
	switch {
	case len(args) == 0:
		fmt.Print(cmp.Or(doc, "(no strategy yet: add lessons with 37studio strategy add \"...\")\n"))
		return 0
	case args[0] == "add" && len(args) > 1:
		doc = addNote(doc, strings.Join(args[1:], " "), time.Now())
	case args[0] == "set" && len(args) == 2:
		b, err := os.ReadFile(args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		doc = string(b)
	default:
		fmt.Fprintln(os.Stderr, `usage: 37studio strategy | strategy add "one lesson" | strategy set <file>`)
		return 2
	}
	if err := os.WriteFile(strategyPath(tenant), []byte(doc), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Println("saved")
	return 0
}
