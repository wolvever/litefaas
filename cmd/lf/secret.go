package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func cmdSecret(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: lf secret set|get|list|delete ...")
	}
	switch args[0] {
	case "set":
		return secretSet(args[1:])
	case "get":
		return secretGet(args[1:])
	case "list", "ls":
		return secretList(args[1:])
	case "delete", "rm":
		return secretDelete(args[1:])
	default:
		return fmt.Errorf("unknown secret command %q (want set|get|list|delete)", args[0])
	}
}

func secretSet(args []string) error {
	fs := flag.NewFlagSet("secret set", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	value := fs.String("value", "", "secret value")
	fromFile := fs.String("from-file", "", "read secret value from file")
	gw := fs.String("gateway", "", "litefaasd URL")
	tok := fs.String("token", "", "bearer token")
	cfg := fs.String("config-dir", "", "CLI config directory")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	if len(rest) < 1 {
		return fmt.Errorf("usage: lf secret set <name> [--value V | --from-file PATH]")
	}
	name := rest[0]
	val := *value
	switch {
	case *fromFile != "":
		raw, err := os.ReadFile(*fromFile)
		if err != nil {
			return err
		}
		val = strings.TrimRight(string(raw), "\n")
	case val != "":
	default:
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		val = strings.TrimRight(string(raw), "\n")
	}
	if val == "" {
		return fmt.Errorf("secret value is empty")
	}
	c, err := resolveClient(*gw, *tok, *cfg)
	if err != nil {
		return err
	}
	out, err := c.SecretPut(name, val)
	if err != nil {
		return err
	}
	fmt.Printf("secret %s set\n", out.Name)
	return nil
}

func secretGet(args []string) error {
	gw, tok, cfg, rest := gatewayFlags(args)
	if len(rest) < 1 {
		return fmt.Errorf("usage: lf secret get <name>")
	}
	c, err := resolveClient(gw, tok, cfg)
	if err != nil {
		return err
	}
	out, err := c.SecretGet(rest[0])
	if err != nil {
		return err
	}
	fmt.Println(out.Value)
	return nil
}

func secretList(args []string) error {
	gw, tok, cfg, _ := gatewayFlags(args)
	c, err := resolveClient(gw, tok, cfg)
	if err != nil {
		return err
	}
	list, err := c.SecretList()
	if err != nil {
		return err
	}
	for _, s := range list {
		fmt.Println(s.Name)
	}
	return nil
}

func secretDelete(args []string) error {
	gw, tok, cfg, rest := gatewayFlags(args)
	if len(rest) < 1 {
		return fmt.Errorf("usage: lf secret delete <name>")
	}
	c, err := resolveClient(gw, tok, cfg)
	if err != nil {
		return err
	}
	if err := c.SecretDelete(rest[0]); err != nil {
		return err
	}
	fmt.Printf("secret %s deleted\n", rest[0])
	return nil
}
