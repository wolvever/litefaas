package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
)

func cmdLogs(args []string) error {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	follow := fs.Bool("f", false, "follow log output")
	followLong := fs.Bool("follow", false, "follow log output")
	tail := fs.Int("tail", 100, "lines from the end of the logs")
	gw := fs.String("gateway", "", "litefaasd URL")
	tok := fs.String("token", "", "bearer token")
	dir := fs.String("config-dir", "", "CLI config directory")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	if len(rest) < 1 {
		return fmt.Errorf("usage: lf logs <name> [-f] [--tail N]")
	}
	c, err := resolveClient(*gw, *tok, *dir)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return c.Logs(ctx, rest[0], *follow || *followLong, *tail, os.Stdout)
}
