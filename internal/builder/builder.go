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
	"github.com/wolvever/litefaas/internal/stackpack"
	"github.com/wolvever/litefaas/internal/types"
)

// Result is the tagged local image.
type Result struct {
	Image    string
	Context  string
	Stack    string
	Detected bool
	Hints    string
}

func Build(ctx context.Context, dir string, stdout, stderr io.Writer) (Result, error) {
	return BuildWith(ctx, dir, "", stdout, stderr)
}

// BuildResolved builds a single already-resolved service (used by lf check multi-service).
func BuildResolved(ctx context.Context, res *manifest.Resolution, stdout, stderr io.Writer) (Result, error) {
	if res != nil && res.Multi != nil {
		return Result{}, fmt.Errorf("BuildResolved expects a single service")
	}
	return buildOne(ctx, res, stdout, stderr)
}

func BuildWith(ctx context.Context, dir, stackID string, stdout, stderr io.Writer) (Result, error) {
	res, err := manifest.ResolveDetect(dir, stackID)
	if err != nil {
		return Result{}, err
	}
	if res.Multi != nil {
		return Result{}, fmt.Errorf("%s is a stack.yaml; use BuildStack", dir)
	}
	return buildOne(ctx, res, stdout, stderr)
}

func BuildStack(ctx context.Context, dir string, stdout, stderr io.Writer) ([]Result, error) {
	return BuildStackWith(ctx, dir, "", stdout, stderr)
}

func BuildStackWith(ctx context.Context, dir, stackID string, stdout, stderr io.Writer) ([]Result, error) {
	res, err := manifest.ResolveDetect(dir, stackID)
	if err != nil {
		return nil, err
	}
	if res.Multi == nil {
		one, err := buildOne(ctx, res, stdout, stderr)
		if err != nil {
			return nil, err
		}
		return []Result{one}, nil
	}
	var out []Result
	for i := range res.Multi.Services {
		svc := &res.Multi.Services[i]
		ctxDir := manifest.ServiceDir(res.Root, svc)
		one, err := buildOne(ctx, &manifest.Resolution{Root: ctxDir, Manifest: svc}, stdout, stderr)
		if err != nil {
			return out, fmt.Errorf("service %s: %w", svc.Name, err)
		}
		out = append(out, one)
	}
	return out, nil
}

func buildOne(ctx context.Context, res *manifest.Resolution, stdout, stderr io.Writer) (Result, error) {
	if res == nil || res.Manifest == nil {
		return Result{}, fmt.Errorf("manifest is required")
	}
	m := res.Manifest
	if err := materialize(res.Root, m, res.Pack); err != nil {
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
	args := []string{"build", "-t", m.Image, res.Root}
	if err := dockercli.Exec(ctx, stdout, stderr, "docker", args...); err != nil {
		return Result{}, fmt.Errorf("docker build %s: %w", m.Image, err)
	}
	stackID := m.Stack
	if stackID == "" && res.Pack != nil {
		stackID = res.Pack.ID
	}
	hints := ""
	if res.Pack != nil {
		hints = res.Pack.FormatHints()
	}
	return Result{Image: m.Image, Context: res.Root, Stack: stackID, Detected: res.Detected, Hints: hints}, nil
}

func materialize(root string, m *manifest.Manifest, pack *stackpack.Pack) error {
	df := filepath.Join(root, "Dockerfile")
	if _, err := os.Stat(df); err == nil {
		return nil
	}
	if pack == nil && m != nil && m.Stack != "" {
		cat, err := stackpack.Open()
		if err == nil {
			pack, _ = cat.Get(m.Stack)
		}
	}
	if pack != nil {
		if err := pack.WriteDockerfile(root); err != nil {
			return fmt.Errorf("no Dockerfile in %s and stack %s: %w", root, pack.ID, err)
		}
		return nil
	}
	rt, err := types.ParseRuntime(m.Runtime)
	if err != nil {
		return fmt.Errorf("no Dockerfile in %s and runtime %s has no built-in template", root, m.Runtime)
	}
	if err := scaffold.WriteDockerfile(root, rt); err != nil {
		return fmt.Errorf("no Dockerfile in %s and runtime %s has no built-in template: %w", root, m.Runtime, err)
	}
	return nil
}
