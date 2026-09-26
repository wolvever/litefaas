package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/wolvever/litefaas/internal/manifest"
)

func cmdInvoke(args []string) error {
	fs := flag.NewFlagSet("invoke", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	data := fs.String("d", "", "request body")
	dataLong := fs.String("data", "", "request body")
	ct := fs.String("content-type", "application/json", "Content-Type")
	timeout := fs.String("timeout", "30s", "invoke timeout")
	gw := fs.String("gateway", "", "litefaasd URL")
	tok := fs.String("token", "", "bearer token")
	dir := fs.String("config-dir", "", "CLI config directory")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	if len(rest) < 1 {
		return fmt.Errorf("usage: lf invoke <name> [-d payload]")
	}
	payload := *data
	if *dataLong != "" {
		payload = *dataLong
	}
	d, err := manifest.ParseTimeout(*timeout)
	if err != nil {
		return err
	}
	c, err := resolveClient(*gw, *tok, *dir)
	if err != nil {
		return err
	}
	var body []byte
	if payload != "" {
		body = []byte(payload)
	}
	out, err := c.Invoke(rest[0], body, *ct, d)
	if err != nil {
		return err
	}
	if len(out.Body) > 0 {
		os.Stdout.Write(out.Body)
		if out.Body[len(out.Body)-1] != '\n' {
			fmt.Println()
		}
	}
	if out.StatusCode >= 400 {
		return fmt.Errorf("invoke status %d", out.StatusCode)
	}
	return nil
}
