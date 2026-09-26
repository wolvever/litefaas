package main

import (
	"encoding/json"
	"os"
)

func cmdMetrics(args []string) error {
	gw, tok, dir, _ := gatewayFlags(args)
	c, err := resolveClient(gw, tok, dir)
	if err != nil {
		return err
	}
	m, err := c.Metrics()
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return err
	}
	return nil
}
