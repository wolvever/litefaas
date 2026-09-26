package main

import (
	"fmt"

	"github.com/wolvever/litefaas/internal/client"
	"github.com/wolvever/litefaas/internal/manifest"
)

func cmdDeploy(args []string) error {
	gw, tok, cfgDir, rest := gatewayFlags(args)
	dir := "."
	if len(rest) > 0 {
		dir = rest[0]
	}
	m, _, err := manifest.LoadDir(dir)
	if err != nil {
		return err
	}
	c, err := resolveClient(gw, tok, cfgDir)
	if err != nil {
		return err
	}
	res := m.Resource()
	if _, err := c.Create(res); err != nil {
		if !client.IsConflict(err) {
			return err
		}
		if _, err := c.Update(res); err != nil {
			return err
		}
		fmt.Printf("updated %s\n", res.Name)
	} else {
		fmt.Printf("registered %s\n", res.Name)
	}
	rev, err := c.Deploy(res.Name, res.Image)
	if err != nil {
		return err
	}
	fmt.Printf("deployed %s image=%s status=%s revision=%d\n", res.Name, rev.Image, rev.Status, rev.ID)
	return nil
}
