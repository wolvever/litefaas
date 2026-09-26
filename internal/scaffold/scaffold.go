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
	Dir     string // parent directory; created project is Dir/Name
	Force   bool
}

func Init(opts Options) (string, error) {
	if opts.Name == "" {
		return "", fmt.Errorf("name is required")
	}
	if opts.Runtime != types.RuntimeGo {
		return "", fmt.Errorf("runtime %q is not available yet (Phase 2 implements go; java/python come in later phases)", opts.Runtime)
	}
	kind := opts.Kind
	if kind == "" {
		kind = types.KindFunction
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
	err := fs.WalkDir(templates.Runtimes, goHTTP, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(filepath.FromSlash(goHTTP), filepath.FromSlash(p))
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
		raw, err := templates.Runtimes.ReadFile(p)
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

// WriteGoDockerfile copies the Go HTTP Dockerfile into dir if missing.
func WriteGoDockerfile(dir string) error {
	const src = goHTTP + "/Dockerfile"
	raw, err := templates.Runtimes.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "Dockerfile"), raw, 0o644)
}
