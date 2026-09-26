// Package scaffold implements lf init from embedded runtime templates.
package scaffold

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/wolvever/litefaas/internal/types"
	"github.com/wolvever/litefaas/templates"
)

type Options struct {
	Name    string
	Runtime string
	Kind    string
	Preset  string
	Dest    string
}

// Init copies a runtime template into Dest (default ./<name>) and fills litefaas.yaml.
func Init(opts Options) (string, error) {
	if !types.ValidName(opts.Name) {
		return "", fmt.Errorf("name must match [A-Za-z0-9][A-Za-z0-9._-]{0,63}")
	}
	rt, err := types.ParseRuntime(opts.Runtime)
	if err != nil {
		return "", err
	}
	kind, err := types.ParseKind(opts.Kind)
	if err != nil {
		return "", err
	}
	if opts.Preset != "" {
		return "", fmt.Errorf("presets are not implemented in Phase 2 (later phases add gin/spring-boot/fastapi)")
	}
	if rt != types.RuntimeGo {
		return "", fmt.Errorf("runtime %q is not implemented yet (Phase 2 is Go only; Java/Python/frontend come later)", rt)
	}
	if kind == types.KindFrontend {
		return "", fmt.Errorf("kind=frontend is not implemented yet (Phase 4)")
	}

	dest := opts.Dest
	if dest == "" {
		dest = opts.Name
	}
	if err := prepareDest(dest); err != nil {
		return "", err
	}
	if err := copyGoHTTP(dest, opts.Name, string(kind)); err != nil {
		return "", err
	}
	// Written here (not embedded) so templates/runtimes/go/http is not a nested Go module.
	if err := os.WriteFile(filepath.Join(dest, "go.mod"), []byte("module "+opts.Name+"\n\ngo 1.22\n"), 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

func prepareDest(dest string) error {
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
	if len(ents) > 0 {
		return fmt.Errorf("%s is not empty", dest)
	}
	return nil
}

func copyGoHTTP(dest, name, kind string) error {
	return fs.WalkDir(templates.GoHTTP, templates.GoHTTPDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(templates.GoHTTPDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := templates.GoHTTP.ReadFile(path)
		if err != nil {
			return err
		}
		text := strings.ReplaceAll(string(raw), "__NAME__", name)
		text = strings.ReplaceAll(text, "//go:build ignore\r\n\r\n", "")
		text = strings.ReplaceAll(text, "//go:build ignore\n\n", "")
		if filepath.Base(path) == types.ManifestFile {
			text = strings.Replace(text, "kind: function", "kind: "+kind, 1)
		}
		return os.WriteFile(target, []byte(text), 0o644)
	})
}
