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
	runtime := fs.String("runtime", "", "runtime (Phase 2: go)")
	kind := fs.String("kind", string(types.KindFunction), "kind: function|backend|frontend")
	preset := fs.String("preset", "", "optional framework preset (not implemented in Phase 2)")
	force := fs.Bool("force", false, "overwrite files in an existing directory")
	dir := fs.String("dir", ".", "parent directory for the new project")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	if len(rest) < 1 || *runtime == "" {
		return fmt.Errorf("usage: lf init <name> --runtime go [--kind function]")
	}
	if *preset != "" {
		return fmt.Errorf("presets are not implemented yet (Phase 2 ships the generic Go HTTP template)")
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
		Dir:     *dir,
		Force:   *force,
	})
	if err != nil {
		return err
	}
	fmt.Printf("created %s (runtime=go kind=%s)\n", dest, k)
	fmt.Printf("next: cd %s && lf build && lf deploy && lf invoke %s -d '{\"name\":\"litefaas\"}'\n", dest, rest[0])
	return nil
}
