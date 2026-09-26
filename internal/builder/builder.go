// Package builder materializes a build context and runs docker build (RFC-0001 §11).
package builder

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/wolvever/litefaas/internal/dockercli"
	"github.com/wolvever/litefaas/internal/manifest"
	"github.com/wolvever/litefaas/internal/scaffold"
	"github.com/wolvever/litefaas/internal/types"
)

// Result is the tagged local image.
type Result struct {
	Image   string
	Context string
}

func Build(ctx context.Context, dir string, stdout, stderr io.Writer) (Result, error) {
	m, root, err := manifest.LoadDir(dir)
	if err != nil {
		return Result{}, err
	}
	if err := materialize(root, m); err != nil {
		return Result{}, err
	}
	if err := dockercli.Available(ctx); err != nil {
		return Result{}, err
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	args := []string{"build", "-t", m.Image, root}
	if err := dockercli.Exec(ctx, stdout, stderr, "docker", args...); err != nil {
		return Result{}, fmt.Errorf("docker build %s: %w", m.Image, err)
	}
	return Result{Image: m.Image, Context: root}, nil
}

func materialize(root string, m *manifest.Manifest) error {
	df := filepath.Join(root, "Dockerfile")
	if _, err := os.Stat(df); err == nil {
		return nil
	}
	rt, err := types.ParseRuntime(m.Runtime)
	if err != nil {
		return fmt.Errorf("no Dockerfile in %s and runtime %s has no built-in template", root, m.Runtime)
	}
	if rt == types.RuntimeDockerfile {
		return fmt.Errorf("runtime dockerfile requires a user-supplied Dockerfile in %s", root)
	}
	if err := scaffold.WriteDockerfile(root, rt); err != nil {
		return fmt.Errorf("no Dockerfile in %s and runtime %s has no built-in template: %w", root, m.Runtime, err)
	}
	return nil
}
