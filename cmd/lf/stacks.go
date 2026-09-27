package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/wolvever/litefaas/internal/stackpack"
)

func cmdStacks(args []string) error {
	fs := flag.NewFlagSet("stacks", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	asJSON := fs.Bool("json", false, "print JSON")
	_, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	cat, err := stackpack.Open()
	if err != nil {
		return err
	}
	packs := cat.All()
	sort.SliceStable(packs, func(i, j int) bool {
		if packs[i].Priority != packs[j].Priority {
			return packs[i].Priority > packs[j].Priority
		}
		return packs[i].ID < packs[j].ID
	})
	if *asJSON {
		type row struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description,omitempty"`
			Language    string `json:"language"`
			Runtime     string `json:"runtime"`
			Kind        string `json:"kind"`
			Priority    int    `json:"priority"`
		}
		out := make([]row, 0, len(packs))
		for _, p := range packs {
			out = append(out, row{
				ID: p.ID, Title: p.Title, Description: p.Description,
				Language: p.Language, Runtime: p.Runtime, Kind: p.Kind, Priority: p.Priority,
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tLANGUAGE\tRUNTIME\tKIND\tPRIORITY\tTITLE")
	for _, p := range packs {
		title := p.Title
		if title == "" {
			title = p.ID
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n", p.ID, p.Language, p.Runtime, p.Kind, p.Priority, title)
	}
	return w.Flush()
}
