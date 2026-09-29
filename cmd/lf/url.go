package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wolvever/litefaas/internal/client"
	"github.com/wolvever/litefaas/internal/manifest"
	"github.com/wolvever/litefaas/internal/types"
)

type edgeURLInfo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Kind string `json:"kind"` // http | invoke
}

func cmdURL(args []string) error {
	fs := flag.NewFlagSet("url", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	asJSON := fs.Bool("json", false, "print JSON")
	all := fs.Bool("all", false, "print all resolved resources (multi-service)")
	gw := fs.String("gateway", "", "litefaasd URL")
	tok := fs.String("token", "", "bearer token")
	cfg := fs.String("config-dir", "", "CLI config directory")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	c, err := resolveClient(*gw, *tok, *cfg)
	if err != nil {
		return err
	}
	infos, err := resolveEdgeURLs(c, rest, *all)
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if len(infos) == 1 && !*all {
			return enc.Encode(infos[0])
		}
		return enc.Encode(infos)
	}
	for _, info := range infos {
		fmt.Println(info.URL)
	}
	return nil
}

func cmdOpen(args []string) error {
	fs := flag.NewFlagSet("open", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	gw := fs.String("gateway", "", "litefaasd URL")
	tok := fs.String("token", "", "bearer token")
	cfg := fs.String("config-dir", "", "CLI config directory")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	c, err := resolveClient(*gw, *tok, *cfg)
	if err != nil {
		return err
	}
	infos, err := resolveEdgeURLs(c, rest, false)
	if err != nil {
		return err
	}
	info := infos[0]
	fmt.Println(info.URL)
	if info.Kind == "invoke" {
		fmt.Fprintln(os.Stderr, "open: invoke URL is POST-only; not opening a browser")
		return nil
	}
	if err := openBrowser(info.URL); err != nil {
		fmt.Fprintf(os.Stderr, "open: %v\n", err)
	}
	return nil
}

// resolveEdgeURLs resolves name|path → one or more primary edge URLs.
func resolveEdgeURLs(c *client.Client, rest []string, all bool) ([]edgeURLInfo, error) {
	arg := ""
	if len(rest) > 0 {
		arg = rest[0]
	}
	resources, err := resolveURLResources(c, arg, all)
	if err != nil {
		return nil, err
	}
	out := make([]edgeURLInfo, 0, len(resources))
	for _, res := range resources {
		u, kind, err := primaryEdgeURL(c.Gateway, res)
		if err != nil {
			return nil, err
		}
		out = append(out, edgeURLInfo{Name: res.Name, URL: u, Kind: kind})
	}
	return out, nil
}

func resolveURLResources(c *client.Client, arg string, all bool) ([]types.Resource, error) {
	if arg != "" && !looksLikePath(arg) {
		res, err := c.Get(arg)
		if err == nil {
			return []types.Resource{res}, nil
		}
		// Fall through to project path if Get fails and arg looks usable as a path.
		if !pathExists(arg) {
			return nil, fmt.Errorf("resource %q: %w", arg, err)
		}
	}
	dir := arg
	if dir == "" {
		dir = "."
	}
	resolved, err := manifest.ResolveDetect(dir, "")
	if err != nil {
		return nil, err
	}
	if resolved.Multi != nil {
		names := make([]string, 0, len(resolved.Multi.Services))
		var list []types.Resource
		for i := range resolved.Multi.Services {
			r := resolved.Multi.Services[i].Resource()
			names = append(names, r.Name)
			list = append(list, r)
		}
		if all {
			return fetchOrLocal(c, list), nil
		}
		return nil, fmt.Errorf("multi-service project (%s); pass a resource name or --all", strings.Join(names, ", "))
	}
	local := resolved.Manifest.Resource()
	if remote, err := c.Get(local.Name); err == nil {
		return []types.Resource{remote}, nil
	}
	return []types.Resource{local}, nil
}

func fetchOrLocal(c *client.Client, local []types.Resource) []types.Resource {
	out := make([]types.Resource, 0, len(local))
	for _, r := range local {
		if remote, err := c.Get(r.Name); err == nil {
			out = append(out, remote)
		} else {
			out = append(out, r)
		}
	}
	return out
}

func looksLikePath(s string) bool {
	if s == "." || s == ".." {
		return true
	}
	if strings.Contains(s, string(filepath.Separator)) || strings.Contains(s, "/") {
		return true
	}
	return pathExists(s) && isDirOrManifest(s)
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func isDirOrManifest(p string) bool {
	fi, err := os.Stat(p)
	if err != nil {
		return false
	}
	if fi.IsDir() {
		return true
	}
	base := filepath.Base(p)
	return base == "litefaas.yaml" || base == "litefaas.yml" || base == "stack.yaml" || base == "stack.yml"
}

func openBrowser(u string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	return cmd.Start()
}
