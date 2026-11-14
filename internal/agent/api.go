package agent

import (
	"cmp"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/costs"
	"github.com/Shaitrish/37studio/internal/integrations"
	"github.com/Shaitrish/37studio/internal/remote"
	"github.com/Shaitrish/37studio/internal/store"
)

// APICLI is `37studio api <service> <METHOD> </path>`: one call to a service connected in Settings → AI services →
// Other services, signed with its key, so the agent can use any API without the key in its environment. A reply
// saved with --out is streamed to the file; with --units its cost lands on $STUDIO37_PROJECT.
func APICLI(args []string) int {
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: 37studio api <service> <GET|POST|PUT|PATCH|DELETE> </path> [--json BODY | --json-file FILE] [--out FILE] [--units N --item WHAT] [--approved]")
		return 2
	}
	id, method, path := args[0], args[1], args[2]
	fs := flag.NewFlagSet("api", flag.ContinueOnError)
	body := fs.String("json", "", "JSON request body")
	bodyFile := fs.String("json-file", "", "or a file holding the JSON body")
	out := fs.String("out", "", "save the reply to this file (audio, images, video) instead of printing it")
	units := fs.Float64("units", 0, "what this call bills, in the service's unit (Settings shows it), for the cost and the spending check")
	item := fs.String("item", "", "what it made, for the Costs page, like voiceover")
	approved := fs.Bool("approved", false, "the editor agreed to go over the spending limit for generated media")
	if err := fs.Parse(args[3:]); err != nil {
		return 2
	}
	config.DataDir, _ = filepath.Abs(config.DataDir)
	store.Open()
	var rd io.Reader
	ct := ""
	if *bodyFile != "" {
		f, err := os.Open(*bodyFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 2
		}
		defer f.Close()
		rd, ct = f, "application/json"
	} else if *body != "" {
		rd, ct = strings.NewReader(*body), "application/json"
	}
	req, err := integrations.Request(id, method, path, rd, ct)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	pid, _ := strconv.ParseInt(config.Env("STUDIO37_PROJECT", ""), 10, 64)
	var tenant int64
	if p, _ := store.Row("select tenant_id from projects where id=?", pid); p != nil {
		tenant = p["tenant_id"].(int64)
	}
	var price float64
	store.DB.QueryRow("select unit_price from integrations where id=?", id).Scan(&price)
	cost := *units * price
	if msg := overMediaLimit(pid, id+" call", cost); msg != "" && cost > 0 && !*approved {
		fmt.Fprintln(os.Stderr, msg)
		return 3
	}
	res, err := remote.Uploads.Do(req) // no timeout: a long voiceover or a video can take minutes
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		fmt.Fprintln(os.Stderr, "error:", integrations.Error(res))
		return 1
	}
	dst := io.Writer(os.Stdout)
	if *out != "" {
		*out, _ = filepath.Abs(*out)
		os.MkdirAll(filepath.Dir(*out), 0o755)
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		defer f.Close()
		dst = f
	}
	if _, err := io.Copy(dst, res.Body); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if *units > 0 {
		costs.Record(id, pid, tenant, cmp.Or(*item, strings.ToUpper(method)+" "+path), *units, cost, *out)
	}
	if *out != "" {
		fmt.Println(*out)
	}
	return 0
}
