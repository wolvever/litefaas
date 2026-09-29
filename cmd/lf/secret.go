package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/wolvever/litefaas/internal/secret"
)

func cmdSecret(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: lf secret set|get|list|delete|import ...")
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
	case "import":
		return secretImport(args[1:])
	default:
		return fmt.Errorf("unknown secret command %q (want set|get|list|delete|import)", args[0])
	}
}

func secretSet(args []string) error {
	fs := flag.NewFlagSet("secret set", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	value := fs.String("value", "", "secret value")
	fromFile := fs.String("from-file", "", "read secret value from file")
	env := fs.String("env", "", "secret env bag (default: default; not CLI context)")
	gw := fs.String("gateway", "", "litefaasd URL")
	tok := fs.String("token", "", "bearer token")
	cfg := fs.String("config-dir", "", "CLI config directory")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	if len(rest) < 1 {
		return fmt.Errorf("usage: lf secret set <name> [--value V | --from-file PATH] [--env NAME]")
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
		// --value wins over remaining argv
	case len(rest) > 1:
		val = strings.Join(rest[1:], " ")
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
	out, err := c.SecretPutEnv(*env, name, val)
	if err != nil {
		return err
	}
	if out.Env != "" && out.Env != secret.DefaultEnv {
		fmt.Printf("secret %s set (env=%s)\n", out.Name, out.Env)
	} else {
		fmt.Printf("secret %s set\n", out.Name)
	}
	return nil
}

func secretGet(args []string) error {
	fs := flag.NewFlagSet("secret get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	env := fs.String("env", "", "secret env bag")
	gw := fs.String("gateway", "", "litefaasd URL")
	tok := fs.String("token", "", "bearer token")
	cfg := fs.String("config-dir", "", "CLI config directory")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	if len(rest) < 1 {
		return fmt.Errorf("usage: lf secret get <name> [--env NAME]")
	}
	c, err := resolveClient(*gw, *tok, *cfg)
	if err != nil {
		return err
	}
	out, err := c.SecretGetEnv(*env, rest[0])
	if err != nil {
		return err
	}
	fmt.Println(out.Value)
	return nil
}

func secretList(args []string) error {
	fs := flag.NewFlagSet("secret list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	env := fs.String("env", "", "secret env bag")
	gw := fs.String("gateway", "", "litefaasd URL")
	tok := fs.String("token", "", "bearer token")
	cfg := fs.String("config-dir", "", "CLI config directory")
	_, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	c, err := resolveClient(*gw, *tok, *cfg)
	if err != nil {
		return err
	}
	list, err := c.SecretList(*env)
	if err != nil {
		return err
	}
	for _, s := range list {
		fmt.Println(s.Name)
	}
	return nil
}

func secretDelete(args []string) error {
	fs := flag.NewFlagSet("secret delete", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	env := fs.String("env", "", "secret env bag")
	gw := fs.String("gateway", "", "litefaasd URL")
	tok := fs.String("token", "", "bearer token")
	cfg := fs.String("config-dir", "", "CLI config directory")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	if len(rest) < 1 {
		return fmt.Errorf("usage: lf secret delete <name> [--env NAME]")
	}
	c, err := resolveClient(*gw, *tok, *cfg)
	if err != nil {
		return err
	}
	if err := c.SecretDeleteEnv(*env, rest[0]); err != nil {
		return err
	}
	fmt.Printf("secret %s deleted\n", rest[0])
	return nil
}

func secretImport(args []string) error {
	fs := flag.NewFlagSet("secret import", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dryRun := fs.Bool("dry-run", false, "parse and list names without calling the API")
	env := fs.String("env", "", "secret env bag to import into (default: default)")
	gw := fs.String("gateway", "", "litefaasd URL")
	tok := fs.String("token", "", "bearer token")
	cfg := fs.String("config-dir", "", "CLI config directory")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	if len(rest) < 1 {
		return fmt.Errorf("usage: lf secret import <file> [--env NAME] [--dry-run]")
	}
	raw, err := os.ReadFile(rest[0])
	if err != nil {
		return err
	}
	pairs, err := parseDotEnv(string(raw))
	if err != nil {
		return err
	}
	if len(pairs) == 0 {
		return fmt.Errorf("no secrets found in %s", rest[0])
	}
	names := make([]string, 0, len(pairs))
	for _, p := range pairs {
		names = append(names, p.Name)
	}
	envLabel := *env
	if envLabel == "" {
		envLabel = secret.DefaultEnv
	}
	if *dryRun {
		fmt.Printf("would import %d secrets into env %s: %s\n", len(names), envLabel, strings.Join(names, " "))
		return nil
	}
	c, err := resolveClient(*gw, *tok, *cfg)
	if err != nil {
		return err
	}
	succeeded := 0
	for _, p := range pairs {
		if _, err := c.SecretPutEnv(*env, p.Name, p.Value); err != nil {
			if succeeded > 0 {
				return fmt.Errorf("imported %d secrets (%s); failed on %s: %w", succeeded, strings.Join(names[:succeeded], " "), p.Name, err)
			}
			return fmt.Errorf("import %s: %w", p.Name, err)
		}
		succeeded++
	}
	fmt.Printf("imported %d secrets into env %s: %s\n", len(names), envLabel, strings.Join(names, " "))
	return nil
}

type envPair struct {
	Name  string
	Value string
}

// parseDotEnv parses a minimal dotenv subset. Values are never included in returned errors.
func parseDotEnv(src string) ([]envPair, error) {
	var out []envPair
	scanner := bufio.NewScanner(strings.NewReader(src))
	// Allow long lines (secrets) up to 1 MiB.
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1<<20)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if strings.HasPrefix(trim, "export ") {
			trim = strings.TrimSpace(strings.TrimPrefix(trim, "export "))
		}
		i := strings.IndexByte(trim, '=')
		if i <= 0 {
			return nil, fmt.Errorf("line %d: expected KEY=VALUE", lineNo)
		}
		key := strings.TrimSpace(trim[:i])
		val := trim[i+1:]
		if strings.ContainsAny(key, " \t") {
			return nil, fmt.Errorf("line %d: invalid secret name %q", lineNo, key)
		}
		if err := secret.ValidateName(key); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		if strings.HasPrefix(val, "\"") || strings.HasPrefix(val, "'") {
			q := val[0]
			end := strings.LastIndexByte(val, q)
			if end <= 0 {
				return nil, fmt.Errorf("line %d: unclosed quote (multiline values not supported)", lineNo)
			}
			if end != len(val)-1 {
				// Trailing comment after quoted value is uncommon; reject to avoid surprises.
				trail := strings.TrimSpace(val[end+1:])
				if trail != "" && !strings.HasPrefix(trail, "#") {
					return nil, fmt.Errorf("line %d: unexpected trailing content after quoted value", lineNo)
				}
			}
			val = val[1:end]
		} else {
			// Unquoted: strip inline comment (# preceded by space) and trailing space.
			if j := strings.Index(val, " #"); j >= 0 {
				val = val[:j]
			}
			val = strings.TrimRight(val, " \t")
		}
		out = append(out, envPair{Name: key, Value: val})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
