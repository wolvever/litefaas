// Package scaffold copies runtime templates for `lf init` (RFC-0001 §8, §10).
package scaffold

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/wolvever/litefaas/internal/types"
	"github.com/wolvever/litefaas/templates"
)

const goHTTP = "runtimes/go/http"

// Options control `lf init`.
type Options struct {
	Name    string
	Runtime types.Runtime
	Kind    types.Kind
	Preset  string
	Dir     string // parent directory; created project is Dir/Name
	Force   bool
}

func templateRoot(rt types.Runtime, preset string) (fs.FS, string, error) {
	if preset != "" {
		switch string(rt) + "/" + preset {
		case "java/spring-boot":
			return templates.Presets, "presets/java/spring-boot", nil
		case "python/fastapi":
			return templates.Presets, "presets/python/fastapi", nil
		default:
			return nil, "", fmt.Errorf("unknown preset %q for runtime %s (Phase 3 ships spring-boot and fastapi)", preset, rt)
		}
	}
	switch rt {
	case types.RuntimeGo:
		return templates.Runtimes, goHTTP, nil
	case types.RuntimeJava:
		return templates.Runtimes, "runtimes/java/http", nil
	case types.RuntimePython:
		return templates.Runtimes, "runtimes/python/http", nil
	case types.RuntimeStatic:
		return templates.Frontend, "frontend/static", nil
	case types.RuntimeNode:
		return templates.Runtimes, "runtimes/node/http", nil
	case types.RuntimeDockerfile:
		return templates.Meta, "meta/dockerfile", nil
	default:
		return nil, "", fmt.Errorf("unknown runtime %q (want go|java|python|node|dockerfile|static)", rt)
	}
}

func Init(opts Options) (string, error) {
	if opts.Name == "" {
		return "", fmt.Errorf("name is required")
	}
	srcFS, src, err := templateRoot(opts.Runtime, opts.Preset)
	if err != nil {
		return "", err
	}
	kind := opts.Kind
	if kind == "" {
		kind = types.DefaultKind(opts.Runtime)
	}
	if _, err := types.ParseKind(string(kind)); err != nil {
		return "", err
	}
	parent := opts.Dir
	if parent == "" {
		parent = "."
	}
	dest := filepath.Join(parent, opts.Name)
	if err := prepareDir(dest, opts.Force); err != nil {
		return "", err
	}
	replacer := strings.NewReplacer(
		"{{name}}", opts.Name,
		"{{kind}}", string(kind),
	)
	err = fs.WalkDir(srcFS, src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(filepath.FromSlash(src), filepath.FromSlash(p))
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		base := path.Base(p)
		if base == "template.yml" {
			return nil
		}
		if strings.HasSuffix(rel, ".tmpl") {
			rel = strings.TrimSuffix(rel, ".tmpl")
		}
		out := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		raw, err := fs.ReadFile(srcFS, p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		body := replacer.Replace(string(raw))
		return os.WriteFile(out, []byte(body), 0o644)
	})
	if err != nil {
		return "", err
	}
	return dest, nil
}

func prepareDir(dest string, force bool) error {
	info, err := os.Stat(dest)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(dest, 0o755)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s exists and is not a directory", dest)
	}
	ents, err := os.ReadDir(dest)
	if err != nil {
		return err
	}
	if len(ents) > 0 && !force {
		return fmt.Errorf("%s is not empty (use --force to overwrite)", dest)
	}
	return nil
}

// WriteDockerfile copies the generic Dockerfile for runtime into dir if present.
func WriteDockerfile(dir string, runtime types.Runtime) error {
	srcFS, src, err := templateRoot(runtime, "")
	if err != nil {
		return err
	}
	raw, err := fs.ReadFile(srcFS, path.Join(src, "Dockerfile"))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "Dockerfile"), raw, 0o644)
}

// WriteGoDockerfile copies the Go HTTP Dockerfile into dir if missing.
func WriteGoDockerfile(dir string) error {
	return WriteDockerfile(dir, types.RuntimeGo)
}

// HasDockerfile reports whether the runtime template ships a Dockerfile
// (without writing anything to disk).
func HasDockerfile(runtime types.Runtime) bool {
	srcFS, src, err := templateRoot(runtime, "")
	if err != nil {
		return false
	}
	_, err = fs.ReadFile(srcFS, path.Join(src, "Dockerfile"))
	return err == nil
}
