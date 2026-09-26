package main

import (
	"context"
	"fmt"
	"os"

	"github.com/wolvever/litefaas/internal/builder"
)

func cmdBuild(args []string) error {
	dir := "."
	if len(args) > 0 && !isFlag(args[0]) {
		dir = args[0]
	}
	fmt.Fprintf(os.Stderr, "building %s\n", dir)
	res, err := builder.Build(context.Background(), dir, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}
	fmt.Printf("built %s\n", res.Image)
	return nil
}

func isFlag(s string) bool {
	return len(s) > 0 && s[0] == '-'
}
