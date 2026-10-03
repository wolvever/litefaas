package main

import (
	"fmt"
	"os"

	"github.com/wolvever/litefaas/internal/schema"
)

func cmdConfig(args []string) error {
	if len(args) == 0 || args[0] == "validate" {
		rest := args
		if len(rest) > 0 && rest[0] == "validate" {
			rest = rest[1:]
		}
		return cmdConfigValidate(rest)
	}
	return fmt.Errorf("unknown config command %q\n\nRun 'lf config validate [path]'", args[0])
}

func cmdConfigValidate(args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}
	diags, err := schema.ValidatePath(path)
	if err != nil {
		return err
	}
	if len(diags) == 0 {
		fmt.Println("config ok")
		return nil
	}
	for _, d := range diags {
		fmt.Fprintln(os.Stderr, d.String())
	}
	if schema.HasErrors(diags) {
		return fmt.Errorf("config invalid")
	}
	return nil
}

func printSchemaWarnings(path string) {
	diags, err := schema.ValidatePath(path)
	if err != nil || len(diags) == 0 {
		return
	}
	for _, d := range diags {
		if d.Severity == "warning" {
			fmt.Fprintln(os.Stderr, d.String())
		}
	}
}
