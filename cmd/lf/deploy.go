package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/wolvever/litefaas/internal/client"
	"github.com/wolvever/litefaas/internal/manifest"
	"github.com/wolvever/litefaas/internal/netlifycompat"
	"github.com/wolvever/litefaas/internal/proxy"
	"github.com/wolvever/litefaas/internal/types"
)

func cmdDeploy(args []string) error {
	fs := flag.NewFlagSet("deploy", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	gw := fs.String("gateway", "", "litefaasd URL (overrides context)")
	tok := fs.String("token", "", "bearer token (overrides context / LITEFAAS_TOKEN)")
	cfgDir := fs.String("config-dir", "", "CLI config directory (default ~/.litefaas)")
	stackID := fs.String("stack", "", "stack pack id (overrides detection)")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	dir := "."
	if len(rest) > 0 {
		dir = rest[0]
	}
	res, err := manifest.ResolveDetect(dir, *stackID)
	if err != nil {
		return err
	}
	c, err := resolveClient(*gw, *tok, *cfgDir)
	if err != nil {
		return err
	}
	if res.Multi != nil {
		n := 0
		for i := range res.Multi.Services {
			if err := deployResource(c, res.Multi.Services[i].Resource()); err != nil {
				return err
			}
			n++
		}
		fmt.Printf("deployed %d services → gateway %s\n", n, c.Gateway)
		return applyProjectEdgeRules(c, dir)
	}
	if res.Detected && res.Pack != nil {
		fmt.Printf("detected stack=%s\n", res.Pack.ID)
		if h := res.Pack.FormatHints(); h != "" {
			fmt.Printf("stack hints (not started by litefaas): %s\n", h)
		}
	}
	if err := deployResource(c, res.Manifest.Resource()); err != nil {
		return err
	}
	return applyProjectEdgeRules(c, dir)
}

func deployResource(c *client.Client, res types.Resource) error {
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
	dep, err := c.Deploy(res.Name, res.Image)
	if err != nil {
		return err
	}
	formatDeploySummary(os.Stdout, c.Gateway, res, dep)
	return nil
}


func applyProjectEdgeRules(c *client.Client, dir string) error {
	rules, err := netlifycompat.LoadProject(dir)
	if err != nil {
		return err
	}
	if len(rules) == 0 {
		return nil
	}
	rules = netlifycompat.MergeByFrom(rules)
	out := make([]proxy.EdgeRule, len(rules))
	copy(out, rules)
	got, err := c.PutEdgeRules(out)
	if err != nil {
		return fmt.Errorf("edge rules: %w", err)
	}
	nRedir, nHdr := 0, 0
	for _, r := range got {
		if r.Status == 0 {
			nHdr++
		} else {
			nRedir++
		}
	}
	fmt.Printf("edge rules: %d redirect/rewrite, %d header (from _redirects / netlify.toml / vercel.json)\n", nRedir, nHdr)
	return nil
}
