package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/wolvever/litefaas/internal/client"
	"github.com/wolvever/litefaas/internal/config"
	"github.com/wolvever/litefaas/internal/version"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage(os.Stdout)
		return nil
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Printf("lf %s (%s)\n", version.Version, version.Commit)
		return nil
	case "help", "--help", "-h":
		printUsage(os.Stdout)
		return nil
	case "health":
		return cmdHealth(args[1:])
	case "init":
		return cmdInit(args[1:])
	case "build":
		return cmdBuild(args[1:])
	case "deploy":
		return cmdDeploy(args[1:])
	case "invoke":
		return cmdInvoke(args[1:])
	case "list":
		return cmdList(args[1:])
	case "delete":
		return cmdDelete(args[1:])
	case "context":
		return cmdContext(args[1:])
	default:
		return fmt.Errorf("unknown command %q\n\nRun 'lf help' for usage", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `lf — litefaas CLI (RFC-0001)

Usage:
  lf version                 Print CLI version
  lf health                  GET /healthz on the current gateway
  lf init <name> --runtime go|java|python|dockerfile|static [--preset ...] [--kind function|backend|frontend]
  lf build [path]            docker build the litefaas.yaml image
  lf deploy [path]           Register + deploy the container via the API
  lf invoke <name> [-d BODY] POST /v1/invoke/{name}
  lf list                    List resources
  lf delete <name>           Delete a resource (and its container)
  lf context                 Show / list / create / use CLI contexts
  lf help                    Show this help

The daemon (litefaasd) exposes GET /healthz and the /v1 resource API.
See docs/RFC-0001-architecture.md.
`)
}

func gatewayFlags(args []string) (gateway, token, configDir string, rest []string) {
	fs := flag.NewFlagSet("lf", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	gw := fs.String("gateway", "", "litefaasd URL (overrides context)")
	tok := fs.String("token", "", "bearer token (overrides context / LITEFAAS_TOKEN)")
	dir := fs.String("config-dir", "", "CLI config directory (default ~/.litefaas)")
	rest, _ = parseMixed(fs, args)
	return *gw, *tok, *dir, rest
}

// parseMixed accepts flags before or after positional arguments
// (so `lf delete NAME --gateway URL` works).
func parseMixed(fs *flag.FlagSet, args []string) ([]string, error) {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") {
			pos = append(pos, a)
			continue
		}
		name := strings.TrimPrefix(a, "--")
		name = strings.TrimPrefix(name, "-")
		if j := strings.IndexByte(name, '='); j >= 0 {
			flags = append(flags, a)
			continue
		}
		flags = append(flags, a)
		if f := fs.Lookup(name); f != nil && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			flags = append(flags, args[i+1])
			i++
		}
	}
	if err := fs.Parse(flags); err != nil {
		return nil, err
	}
	return pos, nil
}

func resolveClient(gatewayFlag, tokenFlag, configDir string) (*client.Client, error) {
	cfg, err := config.Load(configDir)
	if err != nil {
		return nil, err
	}
	_, ctx, err := cfg.CurrentContext()
	if err != nil {
		return nil, err
	}
	gw := ctx.Gateway
	if v := os.Getenv("LITEFAAS_GATEWAY"); v != "" {
		gw = v
	}
	if gatewayFlag != "" {
		gw = gatewayFlag
	}
	tok := ctx.Token
	if v := os.Getenv("LITEFAAS_TOKEN"); v != "" {
		tok = v
	}
	if tokenFlag != "" {
		tok = tokenFlag
	}
	return client.New(gw, tok), nil
}

func cmdHealth(args []string) error {
	gw, tok, dir, _ := gatewayFlags(args)
	c, err := resolveClient(gw, tok, dir)
	if err != nil {
		return err
	}
	h, err := c.Healthz()
	if err != nil {
		return err
	}
	fmt.Printf("status=%s version=%s gateway=%s\n", h["status"], h["version"], c.Gateway)
	return nil
}

func cmdList(args []string) error {
	gw, tok, dir, _ := gatewayFlags(args)
	c, err := resolveClient(gw, tok, dir)
	if err != nil {
		return err
	}
	list, err := c.List()
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tKIND\tRUNTIME\tREPLICAS\tIMAGE")
	for _, r := range list {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n", r.Name, r.Kind, r.Runtime, r.Replicas, r.Image)
	}
	return tw.Flush()
}

func cmdDelete(args []string) error {
	gw, tok, dir, rest := gatewayFlags(args)
	if len(rest) < 1 {
		return fmt.Errorf("usage: lf delete <name>")
	}
	c, err := resolveClient(gw, tok, dir)
	if err != nil {
		return err
	}
	if err := c.Delete(rest[0]); err != nil {
		return err
	}
	fmt.Printf("deleted %s\n", rest[0])
	return nil
}

func cmdContext(args []string) error {
	if len(args) == 0 {
		return contextShow(nil)
	}
	switch args[0] {
	case "list":
		return contextList(args[1:])
	case "show":
		return contextShow(args[1:])
	case "use":
		return contextUse(args[1:])
	case "create":
		return contextCreate(args[1:])
	case "help", "-h", "--help":
		fmt.Print(`lf context commands:
  lf context              Show the current context
  lf context list         List contexts
  lf context show [name]  Show one context
  lf context use <name>   Select current context
  lf context create <name> [--gateway URL] [--token TOKEN]
`)
		return nil
	default:
		return fmt.Errorf("unknown context command %q", args[0])
	}
}

func configDirFlag(args []string) (dir string, rest []string) {
	fs := flag.NewFlagSet("context", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	d := fs.String("config-dir", "", "CLI config directory")
	rest, _ = parseMixed(fs, args)
	return *d, rest
}

func contextList(args []string) error {
	dir, _ := configDirFlag(args)
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "CURRENT\tNAME\tGATEWAY")
	for name, ctx := range cfg.Contexts {
		mark := ""
		if name == cfg.Current {
			mark = "*"
		}
		gw := ctx.Gateway
		if gw == "" {
			gw = config.DefaultGateway
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", mark, name, gw)
	}
	return tw.Flush()
}

func contextShow(args []string) error {
	dir, rest := configDirFlag(args)
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	name := cfg.Current
	if len(rest) > 0 {
		name = rest[0]
	}
	ctx, ok := cfg.Contexts[name]
	if !ok {
		if name == "default" {
			ctx = config.Context{Gateway: config.DefaultGateway}
		} else {
			return fmt.Errorf("context %q not found", name)
		}
	}
	if ctx.Gateway == "" {
		ctx.Gateway = config.DefaultGateway
	}
	fmt.Printf("current=%s name=%s gateway=%s\n", cfg.Current, name, ctx.Gateway)
	return nil
}

func contextUse(args []string) error {
	dir, rest := configDirFlag(args)
	if len(rest) < 1 {
		return fmt.Errorf("usage: lf context use <name>")
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	if _, ok := cfg.Contexts[rest[0]]; !ok {
		return fmt.Errorf("context %q not found", rest[0])
	}
	cfg.Current = rest[0]
	if err := cfg.Save(dir); err != nil {
		return err
	}
	fmt.Printf("using context %s\n", rest[0])
	return nil
}

func contextCreate(args []string) error {
	fs := flag.NewFlagSet("context create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	gw := fs.String("gateway", config.DefaultGateway, "litefaasd URL")
	tok := fs.String("token", "", "bearer token")
	dir := fs.String("config-dir", "", "CLI config directory")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	if len(rest) < 1 {
		return fmt.Errorf("usage: lf context create <name> [--gateway URL] [--token TOKEN]")
	}
	cfg, err := config.Load(*dir)
	if err != nil {
		return err
	}
	cfg.Contexts[rest[0]] = config.Context{Gateway: *gw, Token: *tok}
	if cfg.Current == "" {
		cfg.Current = rest[0]
	}
	if err := cfg.Save(*dir); err != nil {
		return err
	}
	fmt.Printf("created context %s gateway=%s\n", rest[0], *gw)
	return nil
}
