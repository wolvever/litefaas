package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/wolvever/litefaas/internal/proxy"
)

func cmdRoutes(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "set":
			return cmdRoutesSet(args[1:])
		case "clear":
			return cmdRoutesClear(args[1:])
		case "list":
			args = args[1:]
		case "help", "-h", "--help":
			fmt.Print(`lf routes commands:
  lf routes                 List the edge route table
  lf routes set <file.json> Replace the route table (PUT /v1/routes)
  lf routes clear           Drop the override; derive routes from manifests
`)
			return nil
		}
	}
	gw, tok, dir, _ := gatewayFlags(args)
	c, err := resolveClient(gw, tok, dir)
	if err != nil {
		return err
	}
	list, err := c.Routes()
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PATH\tNAME\tSTRIP\tSPA\tENDPOINT")
	for _, r := range list {
		fmt.Fprintf(tw, "%s\t%s\t%t\t%t\t%s\n", r.Path, r.Name, r.StripPrefix, r.SPA, r.Endpoint)
	}
	return tw.Flush()
}

func cmdRoutesSet(args []string) error {
	gw, tok, dir, rest := gatewayFlags(args)
	if len(rest) < 1 {
		return fmt.Errorf("usage: lf routes set <file.json>")
	}
	raw, err := os.ReadFile(rest[0])
	if err != nil {
		return err
	}
	var routes []proxy.Route
	if err := json.Unmarshal(raw, &routes); err != nil {
		return fmt.Errorf("parse %s: %w", rest[0], err)
	}
	c, err := resolveClient(gw, tok, dir)
	if err != nil {
		return err
	}
	out, err := c.PutRoutes(routes)
	if err != nil {
		return err
	}
	fmt.Printf("set %d route(s)\n", len(out))
	return nil
}

func cmdRoutesClear(args []string) error {
	gw, tok, dir, _ := gatewayFlags(args)
	c, err := resolveClient(gw, tok, dir)
	if err != nil {
		return err
	}
	if err := c.ClearRoutes(); err != nil {
		return err
	}
	fmt.Println("cleared route override")
	return nil
}
