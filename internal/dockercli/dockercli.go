// Package dockercli runs the docker CLI. Tests swap Exec/Output.
package dockercli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Exec runs a command, streaming stdout/stderr.
var Exec = func(ctx context.Context, stdout, stderr io.Writer, name string, args ...string) error {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// Output runs a command and returns trimmed stdout.
var Output = func(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(string(out)), nil
}

func Available(ctx context.Context) error {
	if _, err := Output(ctx, "docker", "version", "--format", "{{.Server.Version}}"); err != nil {
		return fmt.Errorf("docker is required (CLI + daemon on this host): %w", err)
	}
	return nil
}
