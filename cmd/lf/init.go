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
	runtime := fs.String("runtime", "", "runtime: go|java|python")
	kind := fs.String("kind", string(types.KindFunction), "kind: function|backend|frontend")
	preset := fs.String("preset", "", "optional preset: spring-boot (java) or fastapi (python)")
	force := fs.Bool("force", false, "overwrite files in an existing directory")
	dir := fs.String("dir", ".", "parent directory for the new project")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	if len(rest) < 1 || *runtime == "" {
		return fmt.Errorf("usage: lf init <name> --runtime go|java|python [--preset spring-boot|fastapi] [--kind function]")
	}
	rt, err := types.ParseRuntime(*runtime)
	if err != nil {
		return err
	}
	k, err := types.ParseKind(*kind)
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
	fmt.Printf("next: cd %s && lf build && lf deploy && lf invoke %s -d '{\"name\":\"litefaas\"}'\n", dest, rest[0])
	return nil
}
