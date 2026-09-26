// Package builder runs docker build from a function directory (RFC-0001 §11).
package builder

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/wolvever/litefaas/internal/dockerx"
	"github.com/wolvever/litefaas/internal/manifest"
	"github.com/wolvever/litefaas/internal/types"
	"github.com/wolvever/litefaas/templates"
)

// Build reads litefaas.yaml under path, materializes a Dockerfile if needed,
// and tags a local image (no registry push).
func Build(ctx context.Context, path string, out io.Writer) (image string, err error) {
	m, dir, err := manifest.Load(path)
	if err != nil {
		return "", err
	}
	if err := materializeDockerfile(dir, types.Runtime(m.Runtime)); err != nil {
		return "", err
	}
	if err := dockerx.Available(ctx); err != nil {
		return "", err
	}
	if out == nil {
		out = io.Discard
	}
	fmt.Fprintf(out, "building %s from %s\n", m.Image, dir)
	if err := dockerx.Run(ctx, out, out, "build", "-t", m.Image, dir); err != nil {
		return "", fmt.Errorf("docker build: %w", err)
	}
	return m.Image, nil
}

func materializeDockerfile(dir string, rt types.Runtime) error {
	dst := filepath.Join(dir, "Dockerfile")
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	if rt != types.RuntimeGo {
		return fmt.Errorf("no Dockerfile in %s (runtime %s has no Phase 2 template)", dir, rt)
	}
	raw, err := templates.GoHTTP.ReadFile(templates.GoHTTPDir + "/Dockerfile")
	if err != nil {
		return err
	}
	return os.WriteFile(dst, raw, 0o644)
}
