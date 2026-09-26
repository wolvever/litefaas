package main

import (
	"context"
	"fmt"
	"os"

	"github.com/wolvever/litefaas/internal/builder"
	"github.com/wolvever/litefaas/internal/client"
	"github.com/wolvever/litefaas/internal/manifest"
	"github.com/wolvever/litefaas/internal/stack"
)

func cmdUp(args []string) error {
	gw, tok, cfgDir, rest := gatewayFlags(args)
	dir := "."
	if len(rest) > 0 {
		dir = rest[0]
	}
	c, err := resolveClient(gw, tok, cfgDir)
	if err != nil {
		return err
	}
	if st, root, err := stack.LoadDir(dir); err == nil {
		fmt.Printf("stack %s (%d services)\n", st.Name, len(st.Services))
		for _, svcDir := range st.ServiceDirs(root) {
			if err := buildAndDeploy(c, svcDir); err != nil {
				return err
			}
		}
		return nil
	}
	if _, _, err := manifest.LoadDir(dir); err == nil {
		return buildAndDeploy(c, dir)
	}
	return fmt.Errorf("no %s or %s in %s", stack.FileName, manifest.FileName, dir)
}

func buildAndDeploy(c *client.Client, dir string) error {
	fmt.Fprintf(os.Stderr, "building %s\n", dir)
	res, err := builder.Build(context.Background(), dir, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}
	fmt.Printf("built %s\n", res.Image)
	return registerAndDeploy(c, dir)
}

func registerAndDeploy(c *client.Client, dir string) error {
	m, _, err := manifest.LoadDir(dir)
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
