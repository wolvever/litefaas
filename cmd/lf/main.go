package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/wolvever/litefaas/internal/client"
	"github.com/wolvever/litefaas/internal/config"
	"github.com/wolvever/litefaas/internal/types"
	"github.com/wolvever/litefaas/internal/version"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "lf: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(os.Stdout, usage())
		return nil
	}

	cmd := args[0]
	rest := args[1:]
	switch cmd {
	case "health", "healthz":
		return cmdHealth(rest)
	case "version":
		return cmdVersion(rest)
	case "list":
		return cmdList(rest)
	case "create":
		return cmdCreate(rest)
	case "delete":
		return cmdDelete(rest)
	case "context":
		return cmdContext(rest)
	default:
		return fmt.Errorf("unknown command %q\n\n%s", cmd, usage())
	}
}

func usage() string {
	return `lf — litefaas CLI

Usage:
  lf <command> [flags]

Commands:
  health      Check daemon GET /healthz
  version     Print CLI version and query the gateway
  list        List registered resources
  create      Register resource metadata
  delete      Delete a resource by name
  context     Show or set gateway URL / token

Global flags (most commands):
  --gateway   Gateway URL (else LITEFAAS_GATEWAY, context, or http://127.0.0.1:8080)
  --token     Bearer token (else LITEFAAS_TOKEN or context)

See docs/RFC-0001-architecture.md for the API and roadmap.
`
}

func cmdHealth(args []string) error {
	c, err := newClient(args)
	if err != nil {
		return err
	}
	h, err := c.Health()
	if err != nil {
		return err
	}
	fmt.Printf("status=%s version=%s\n", h.Status, h.Version)
	return nil
}

func cmdVersion(args []string) error {
	fmt.Printf("lf %s\n", version.Version)
	c, err := newClient(args)
	if err != nil {
		return err
	}
	v, err := c.Version()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gateway: %v\n", err)
		return nil
	}
	fmt.Printf("litefaasd %s (%s)\n", v.Version, c.Gateway)
	return nil
}

func cmdList(args []string) error {
	c, err := newClient(args)
	if err != nil {
		return err
	}
	list, err := c.List()
	if err != nil {
		return err
	}
	if len(list.Items) == 0 {
		fmt.Println("No resources registered.")
		return nil
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tKIND\tRUNTIME\tIMAGE")
	for _, r := range list.Items {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Name, r.Kind, r.Runtime, r.Image)
	}
	return tw.Flush()
}

func cmdCreate(args []string) error {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var gateway, token string
	addClientFlags(fs, &gateway, &token)
	name := fs.String("name", "", "resource name")
	kind := fs.String("kind", types.KindFunction, "function | backend | frontend")
	runtime := fs.String("runtime", "", "go | java | python | dockerfile | static")
	image := fs.String("image", "", "container image (optional until deploy)")
	port := fs.Int("port", types.DefaultPort, "listen port contract")
	memory := fs.Int("memory", types.DefaultMemory, "memory MiB")
	timeout := fs.String("timeout", "", "function timeout (e.g. 60s)")
	health := fs.String("health", types.DefaultHealth, "health path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return fmt.Errorf("--name is required")
	}
	c, err := clientFrom(gateway, token)
	if err != nil {
		return err
	}
	created, err := c.Create(types.Resource{
		Name:    *name,
		Kind:    *kind,
		Runtime: *runtime,
		Image:   *image,
		Port:    *port,
		Memory:  *memory,
		Timeout: *timeout,
		Health:  *health,
	})
	if err != nil {
		return err
	}
	fmt.Printf("created %s (%s/%s)\n", created.Name, created.Kind, created.Runtime)
	return nil
}

func cmdDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var gateway, token string
	addClientFlags(fs, &gateway, &token)
	if err := fs.Parse(args); err != nil {
		return err
	}
	name := fs.Arg(0)
	if name == "" {
		return fmt.Errorf("usage: lf delete <name>")
	}
	c, err := clientFrom(gateway, token)
	if err != nil {
		return err
	}
	if err := c.Delete(name); err != nil {
		return err
	}
	fmt.Printf("deleted %s\n", name)
	return nil
}

func cmdContext(args []string) error {
	if len(args) == 0 || args[0] == "show" {
		return contextShow()
	}
	if args[0] == "set" {
		return contextSet(args[1:])
	}
	return fmt.Errorf("usage: lf context [show|set]")
}

func contextShow() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	p, err := config.Path()
	if err != nil {
		return err
	}
	fmt.Printf("config:  %s\n", p)
	fmt.Printf("gateway: %s\n", config.ResolveGateway("", cfg))
	if tok := config.ResolveToken("", cfg); tok != "" {
		fmt.Println("token:   set")
	} else {
		fmt.Println("token:   (unset)")
	}
	return nil
}

func contextSet(args []string) error {
	fs := flag.NewFlagSet("context set", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	gateway := fs.String("gateway", "", "gateway URL to persist")
	token := fs.String("token", "", "optional bearer token to persist")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *gateway == "" && *token == "" {
		return fmt.Errorf("usage: lf context set --gateway URL [--token TOKEN]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if *gateway != "" {
		cfg.Gateway = *gateway
	}
	if *token != "" {
		cfg.Token = *token
	}
	if err := config.Save(cfg); err != nil {
		return err
	}
	p, _ := config.Path()
	fmt.Printf("saved context to %s\n", p)
	return nil
}

func newClient(args []string) (*client.Client, error) {
	fs := flag.NewFlagSet("lf", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var gateway, token string
	addClientFlags(fs, &gateway, &token)
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return clientFrom(gateway, token)
}

func addClientFlags(fs *flag.FlagSet, gateway, token *string) {
	fs.StringVar(gateway, "gateway", "", "gateway URL")
	fs.StringVar(token, "token", "", "bearer token")
}

func clientFrom(gatewayFlag, tokenFlag string) (*client.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return &client.Client{
		Gateway: config.ResolveGateway(gatewayFlag, cfg),
		Token:   config.ResolveToken(tokenFlag, cfg),
	}, nil
}
