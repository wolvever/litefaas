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
	follow := fs.Bool("follow", false, "stream new log lines")
	followShort := fs.Bool("f", false, "stream new log lines")
	tail := fs.Int("tail", 100, "number of lines from the end")
	gw := fs.String("gateway", "", "litefaasd URL")
	tok := fs.String("token", "", "bearer token")
	dir := fs.String("config-dir", "", "CLI config directory")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	if len(rest) < 1 {
		return fmt.Errorf("usage: lf logs <name> [--follow] [--tail N]")
	}
	c, err := resolveClient(*gw, *tok, *dir)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return c.Logs(ctx, rest[0], *tail, *follow || *followShort, os.Stdout)
}
