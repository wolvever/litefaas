package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/wolvever/litefaas/internal/builder"
)

func cmdBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	stackID := fs.String("stack", "", "stack pack id (overrides detection)")
	planOnly := fs.Bool("plan", false, "print detect plan and exit without building")
	asJSON := fs.Bool("json", false, "with --plan, print JSON")
	force := fs.Bool("force", false, "rebuild every stack.yaml service (ignore dirty hashes)")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	dir := "."
	if len(rest) > 0 {
		dir = rest[0]
	}
	if *planOnly {
		return printPlan(dir, *stackID, *asJSON)
	}
	if *asJSON {
		return fmt.Errorf("--json requires --plan")
	}
	fmt.Fprintf(os.Stderr, "building %s\n", dir)
	results, err := builder.BuildProject(context.Background(), dir, builder.ProjectOptions{StackID: *stackID, Dirty: true, Force: *force}, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}
	for _, res := range results {
		if res.Detected && res.Stack != "" {
			fmt.Fprintf(os.Stderr, "detected stack=%s\n", res.Stack)
			if res.Hints != "" {
				fmt.Fprintf(os.Stderr, "stack hints (not started by litefaas): %s\n", res.Hints)
			}
		} else if res.Stack != "" && *stackID != "" {
			fmt.Fprintf(os.Stderr, "stack=%s\n", res.Stack)
		}
		if res.Skipped {
			fmt.Printf("unchanged %s (left running)\n", res.Image)
			continue
		}
		fmt.Printf("built %s\n", res.Image)
	}
	return nil
}
