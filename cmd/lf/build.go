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
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	dir := "."
	if len(rest) > 0 {
		dir = rest[0]
	}
	fmt.Fprintf(os.Stderr, "building %s\n", dir)
	results, err := builder.BuildStackWith(context.Background(), dir, *stackID, os.Stdout, os.Stderr)
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
		fmt.Printf("built %s\n", res.Image)
	}
	return nil
}
