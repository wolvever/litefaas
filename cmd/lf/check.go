package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/wolvever/litefaas/internal/check"
)

func cmdCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	stackID := fs.String("stack", "", "stack pack id (overrides detection)")
	skipHost := fs.Bool("skip-host", false, "skip pack host.build preflight")
	skipSmoke := fs.Bool("skip-smoke", false, "skip one-shot container smoke")
	asJSON := fs.Bool("json", false, "print machine-readable stage results")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	dir := "."
	if len(rest) > 0 {
		dir = rest[0]
	}
	_, err = check.Run(context.Background(), check.Options{
		Dir:       dir,
		StackID:   *stackID,
		SkipHost:  *skipHost,
		SkipSmoke: *skipSmoke,
		JSON:      *asJSON,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
	})
	return err
}

func runPreflightCheck(dir string, stdout, stderr io.Writer) error {
	if dir == "" {
		dir = "."
	}
	fmt.Fprintf(stderr, "preflight: lf check %s\n", dir)
	_, err := check.Run(context.Background(), check.Options{
		Dir:     dir,
		Stdout:  stdout,
		Stderr:  stderr,
	})
	return err
}
