package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/wolvever/litefaas/internal/scaffold"
	"github.com/wolvever/litefaas/internal/types"
)

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	runtime := fs.String("runtime", "", "runtime: go|java|python|dockerfile|static")
	kind := fs.String("kind", "", "kind: function|backend|frontend (default function; backend for dockerfile; frontend for static)")
	preset := fs.String("preset", "", "optional preset: spring-boot (java) or fastapi (python)")
	force := fs.Bool("force", false, "overwrite files in an existing directory")
	dir := fs.String("dir", ".", "parent directory for the new project")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	if len(rest) < 1 || *runtime == "" {
		return fmt.Errorf("usage: lf init <name> --runtime go|java|python|dockerfile|static [--preset spring-boot|fastapi] [--kind function|backend|frontend]")
	}
	rt, err := types.ParseRuntime(*runtime)
	if err != nil {
		return err
	}
	kindVal := *kind
	if kindVal == "" {
		kindVal = string(scaffold.DefaultKind(rt))
	}
	k, err := types.ParseKind(kindVal)
	if err != nil {
		return err
	}
	dest, err := scaffold.Init(scaffold.Options{
		Name:    rest[0],
		Runtime: rt,
		Kind:    k,
		Preset:  *preset,
		Dir:     *dir,
		Force:   *force,
	})
	if err != nil {
		return err
	}
	if *preset != "" {
		fmt.Printf("created %s (runtime=%s preset=%s kind=%s)\n", dest, rt, *preset, k)
	} else {
		fmt.Printf("created %s (runtime=%s kind=%s)\n", dest, rt, k)
	}
	if k == types.KindFunction {
		fmt.Printf("next: cd %s && lf build && lf deploy && lf invoke %s -d '{\"name\":\"litefaas\"}'\n", dest, rest[0])
	} else {
		fmt.Printf("next: cd %s && lf build && lf deploy  # always-on %s; hit its trigger path (not lf invoke)\n", dest, k)
	}
	return nil
}
