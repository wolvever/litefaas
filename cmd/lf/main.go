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
	"github.com/wolvever/litefaas/internal/token"
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
	case "up":
		return cmdUp(args[1:])
	case "health":
		return cmdHealth(args[1:])
	case "init":
		return cmdInit(args[1:])
	case "detect":
		return cmdDetect(args[1:])
	case "build":
		return cmdBuild(args[1:])
	case "deploy":
		return cmdDeploy(args[1:])
	case "revisions", "revision":
		return cmdRevisions(args[1:])
	case "rollback":
		return cmdRollback(args[1:])
	case "invoke":
		return cmdInvoke(args[1:])
	case "list":
		return cmdList(args[1:])
	case "delete":
		return cmdDelete(args[1:])
	case "context":
		return cmdContext(args[1:])
	case "routes", "route":
		return cmdRoutes(args[1:])
	case "logs":
		return cmdLogs(args[1:])
	case "metrics":
		return cmdMetrics(args[1:])
	case "token":
		return cmdToken(args[1:])
	case "secret", "secrets":
		return cmdSecret(args[1:])
	case "stacks", "stack":
		return cmdStacks(args[1:])
	case "check":
		return cmdCheck(args[1:])
	case "watch":
		return cmdWatch(args[1:])
	case "url":
		return cmdURL(args[1:])
	case "open":
		return cmdOpen(args[1:])
	default:
		return fmt.Errorf("unknown command %q\n\nRun 'lf help' for usage", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `lf — litefaas CLI (RFC-0001)

Usage:
  lf version                 Print CLI version
  lf up [flags]               Start or reuse local litefaasd; set default context (--check runs preflight first)
  lf health                  GET /healthz on the current gateway
  lf init <name> --runtime go|java|python|node|dockerfile|static [--preset ...] [--kind function|backend|frontend]
  lf detect [path] [--stack ID] [--json]  Print stack/pack plan without building
  lf build [path] [--stack ID] [--plan] [--json]  docker build (or --plan dry-run)
  lf deploy [path] [--stack ID] [--env NAME] [--inject-env]  Register + deploy (optional release: before cutover)
  lf revisions [name|path]   List deploy revisions (image id, pack, env refs are server-side)
  lf revisions pin|unpin NAME ID  Keep a revision past the 5-row cap
  lf rollback <name> [--id N] [--env NAME]  Redeploy a retained image (does not undo migrations or volumes)
  lf invoke <name> [-d BODY] POST /v1/invoke/{name}
  lf list                    List resources
  lf delete <name> [--prune-volumes]  Delete resource (opt-in volume prune)
  lf logs [name|path] [-f]   Stream container logs (multi-service merges; or --daemon)
  lf routes                  List the edge route table (GET /v1/routes)
  lf routes set <file.json>  Replace the route table (PUT /v1/routes)
  lf routes clear            Drop the override; derive routes from manifests
  lf metrics                 Basic control-plane counters (GET /v1/metrics)
  lf token                   Print the resolved bearer token
  lf secret set|get|list|delete|import [--env NAME]  Secrets in named bags (default env; not CLI context)
  lf stacks [--json]          List embedded stack packs (and LITEFAAS_STACKS_DIR overlays)
  lf check [path] [--stack ID] [--skip-host] [--skip-smoke] [--json]  Host → image → smoke preflight
  lf watch [path] [--stack ID] [--debounce 500ms]  Watch → build → deploy (Docker cutover)
  lf url [name|path] [--all] [--json] [--draft]  Print primary or /--draft/<name>/ URL
  lf open [name|path] [--draft]  Open primary or draft edge URL in the browser
  lf context                 Show / list / create / use CLI contexts
  lf help                    Show this help

The daemon (litefaasd) exposes GET /healthz and the /v1 resource API.
Auth: Authorization: Bearer <token> (see LITEFAAS_TOKEN / --token / ~/.litefaas/token).
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
		if f := fs.Lookup(name); f != nil && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && !isBoolFlag(f) {
			flags = append(flags, args[i+1])
			i++
		}
	}
	if err := fs.Parse(flags); err != nil {
		return nil, err
	}
	return pos, nil
}

func isBoolFlag(f *flag.Flag) bool {
	if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok {
		return bf.IsBoolFlag()
	}
	return false
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
	cfgDir := configDir
	if cfgDir == "" {
		cfgDir = config.DefaultDir()
	}
	tok := token.ResolveClient(tokenFlag, os.Getenv("LITEFAAS_TOKEN"), ctx.Token, cfgDir)
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
	fmt.Fprintln(tw, "NAME\tKIND\tRUNTIME\tIMAGE")
	for _, r := range list {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Name, r.Kind, r.Runtime, r.Image)
	}
	return tw.Flush()
}

func cmdDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	gw := fs.String("gateway", "", "litefaasd URL (overrides context)")
	tok := fs.String("token", "", "bearer token (overrides context / LITEFAAS_TOKEN)")
	dir := fs.String("config-dir", "", "CLI config directory (default ~/.litefaas)")
	prune := fs.Bool("prune-volumes", false, "also remove named Docker volumes for this resource")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	if len(rest) < 1 {
		return fmt.Errorf("usage: lf delete <name> [--prune-volumes]")
	}
	c, err := resolveClient(*gw, *tok, *dir)
	if err != nil {
		return err
	}
	if err := c.Delete(rest[0], *prune); err != nil {
		return err
	}
	if *prune {
		fmt.Printf("deleted %s (volumes pruned)\n", rest[0])
	} else {
		fmt.Printf("deleted %s\n", rest[0])
	}
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
