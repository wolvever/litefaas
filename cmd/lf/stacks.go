package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/wolvever/litefaas/internal/stackpack"
)

func cmdStacks(args []string) error {
	fs := flag.NewFlagSet("stacks", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	asJSON := fs.Bool("json", false, "print JSON")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	_ = rest
	cat, err := stackpack.Open()
	if err != nil {
		return err
	}
	packs := cat.All()
	if *asJSON {
		type row struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Language string `json:"language"`
			Runtime  string `json:"runtime"`
			Kind     string `json:"kind"`
			Priority int    `json:"priority"`
		}
		out := make([]row, 0, len(packs))
		for _, p := range packs {
			out = append(out, row{
				ID: p.ID, Title: p.Title, Language: p.Language,
				Runtime: p.Runtime, Kind: p.Kind, Priority: p.Priority,
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTITLE\tPRIORITY\tRUNTIME\tKIND")
	for _, p := range packs {
		title := p.Title
		if title == "" {
			title = p.ID
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", p.ID, title, p.Priority, p.Runtime, p.Kind)
	}
	return w.Flush()
}
