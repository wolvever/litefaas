package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/wolvever/litefaas/internal/client"
	"github.com/wolvever/litefaas/internal/manifest"
)

func cmdRevisions(args []string) error {
	if len(args) > 0 && (args[0] == "pin" || args[0] == "unpin") {
		return cmdRevisionPin(args)
	}
	fs := flag.NewFlagSet("revisions", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
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
	name := ""
	if len(rest) > 0 {
		name = rest[0]
	}
	name, err = resolveResourceName(c, name)
	if err != nil {
		return err
	}
	revs, err := c.Revisions(name)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSTATUS\tTARGET\tPIN\tIMAGE\tIMAGE ID\tPACK")
	for _, rev := range revs {
		pin := ""
		if rev.Pinned {
			pin = "yes"
		}
		target := rev.Target
		if target == "" {
			target = "prod"
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n", rev.ID, rev.Status, target, pin, rev.Image, rev.ImageID, rev.Snapshot.PackID)
	}
	return tw.Flush()
}

func cmdRevisionPin(args []string) error {
	pin := args[0] == "pin"
	fs := flag.NewFlagSet("revisions "+args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	gw := fs.String("gateway", "", "litefaasd URL (overrides context)")
	tok := fs.String("token", "", "bearer token")
	cfgDir := fs.String("config-dir", "", "CLI config directory")
	rest, err := parseMixed(fs, args[1:])
	if err != nil {
		return err
	}
	if len(rest) < 2 {
		return fmt.Errorf("usage: lf revisions %s NAME ID", args[0])
	}
	id, err := strconv.ParseInt(rest[1], 10, 64)
	if err != nil || id <= 0 {
		return fmt.Errorf("revision id must be a positive integer")
	}
	c, err := resolveClient(*gw, *tok, *cfgDir)
	if err != nil {
		return err
	}
	rev, err := c.PinRevision(rest[0], id, pin)
	if err != nil {
		return err
	}
	state := "unpinned"
	if rev.Pinned {
		state = "pinned"
	}
	fmt.Printf("revision %d %s (%s)\n", rev.ID, state, rev.Image)
	return nil
}

func cmdRollback(args []string) error {
	fs := flag.NewFlagSet("rollback", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	gw := fs.String("gateway", "", "litefaasd URL (overrides context)")
	tok := fs.String("token", "", "bearer token")
	cfgDir := fs.String("config-dir", "", "CLI config directory")
	id := fs.Int64("id", 0, "revision id (default: previous successful image)")
	secEnv := fs.String("env", "", "secret env bag for ${secret:…} resolution")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	if len(rest) < 1 {
		return fmt.Errorf("usage: lf rollback NAME [--id N]")
	}
	c, err := resolveClient(*gw, *tok, *cfgDir)
	if err != nil {
		return err
	}
	name, err := resolveResourceName(c, rest[0])
	if err != nil {
		return err
	}
	dep, err := c.Rollback(name, *id, *secEnv)
	if err != nil {
		return err
	}
	fmt.Printf("rolled back %s to %s (revision %d)\n", name, dep.Image, dep.ID)
	fmt.Println("note: rollback swaps the running image only; it does not undo migrations or volume data")
	return nil
}

// resolveResourceName accepts a resource name or a project path.
func resolveResourceName(c *client.Client, arg string) (string, error) {
	if arg == "" {
		arg = "."
	}
	if _, err := c.Get(arg); err == nil {
		return arg, nil
	}
	res, err := manifest.ResolveDetect(arg, "")
	if err != nil {
		return "", err
	}
	if res.Multi != nil {
		return "", fmt.Errorf("%s is a multi-service stack; pass a service name", arg)
	}
	if res.Manifest == nil || res.Manifest.Name == "" {
		return "", fmt.Errorf("could not resolve a resource name from %s", arg)
	}
	return res.Manifest.Name, nil
}
