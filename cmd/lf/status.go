package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/wolvever/litefaas/internal/client"
	"github.com/wolvever/litefaas/internal/types"
)

type statusRow struct {
	Name     string `json:"name"`
	Revision *int64 `json:"revision"`
	URL      string `json:"url"`
	URLKind  string `json:"url_kind,omitempty"`
	DraftUp  bool   `json:"draft_up"`
}

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	asJSON := fs.Bool("json", false, "print JSON")
	gw := fs.String("gateway", "", "litefaasd URL (overrides context)")
	tok := fs.String("token", "", "bearer token (overrides context / LITEFAAS_TOKEN)")
	cfgDir := fs.String("config-dir", "", "CLI config directory (default ~/.litefaas)")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	c, err := resolveClient(*gw, *tok, *cfgDir)
	if err != nil {
		return err
	}
	arg := ""
	if len(rest) > 0 {
		arg = rest[0]
	}
	resources, err := statusResources(c, arg)
	if err != nil {
		return err
	}
	rows, err := statusRows(c, resources)
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tREVISION\tURL\tDRAFT")
	for _, row := range rows {
		rev := "-"
		if row.Revision != nil {
			rev = fmt.Sprintf("%d", *row.Revision)
		}
		draft := "down"
		if row.DraftUp {
			draft = "up"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", row.Name, rev, row.URL, draft)
	}
	return tw.Flush()
}

func statusResources(c *client.Client, arg string) ([]types.Resource, error) {
	if arg == "" {
		return c.List()
	}
	return resolveURLResources(c, arg, looksLikePath(arg))
}

func statusRows(c *client.Client, resources []types.Resource) ([]statusRow, error) {
	rows := make([]statusRow, 0, len(resources))
	for _, res := range resources {
		revs, err := c.Revisions(res.Name)
		if err != nil {
			return nil, err
		}
		row := statusRow{Name: res.Name, URL: "-"}
		if id, ok := currentProdRevision(revs); ok {
			row.Revision = &id
		}
		if u, kind, err := primaryEdgeURL(c.Gateway, res); err == nil {
			row.URL = u
			row.URLKind = kind
		}
		draft, err := c.DraftStatus(res.Name)
		if err != nil {
			return nil, err
		}
		row.DraftUp = draft.Up
		rows = append(rows, row)
	}
	return rows, nil
}

// currentProdRevision is the newest deployed prod revision.
// Draft rows and failed rows are not "what's running".
func currentProdRevision(revs []types.Revision) (int64, bool) {
	var id int64
	ok := false
	for _, rev := range revs {
		target := rev.Target
		if target == "" {
			target = "prod"
		}
		if target != "prod" || rev.Status != "deployed" {
			continue
		}
		if !ok || rev.ID >= id {
			id = rev.ID
			ok = true
		}
	}
	return id, ok
}
