package main

import (
	"flag"
	"io"
	"os"

	"github.com/wolvever/litefaas/internal/manifest"
)

func cmdDetect(args []string) error {
	fs := flag.NewFlagSet("detect", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	stackID := fs.String("stack", "", "stack pack id (overrides detection)")
	asJSON := fs.Bool("json", false, "print JSON")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	dir := "."
	if len(rest) > 0 {
		dir = rest[0]
	}
	return printPlan(dir, *stackID, *asJSON)
}

func printPlan(dir, stackID string, asJSON bool) error {
	printSchemaWarnings(dir)
	plan, err := manifest.PlanDetect(dir, stackID)
	if err != nil {
		return err
	}
	if asJSON {
		return manifest.WritePlanJSON(os.Stdout, plan)
	}
	return manifest.FormatPlan(os.Stdout, plan)
}
