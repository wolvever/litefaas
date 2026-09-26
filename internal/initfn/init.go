// Package initfn copies a runtime template into a new service directory.
package initfn

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/wolvever/litefaas/internal/manifest"
	"github.com/wolvever/litefaas/internal/types"
	"github.com/wolvever/litefaas/templates"
)

func TemplateDir(runtime types.Runtime, preset string) (string, error) {
	if preset != "" {
		return "", fmt.Errorf("preset %q is not available yet (Phase 3+)", preset)
	}
	switch runtime {
	case types.RuntimeGo:
		return "runtimes/go/http", nil
	default:
		return "", fmt.Errorf("runtime %q is not scaffolded yet; Phase 2 ships go only", runtime)
	}
}

func Init(dest, name string, runtime types.Runtime, kind types.Kind, preset string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if kind == "" {
		kind = types.KindFunction
	}
	src, err := TemplateDir(runtime, preset)
	if err != nil {
		return err
	}
	if dest == "" {
		dest = name
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	entries, err := fs.ReadDir(templates.FS, src)
	if err != nil {
		return fmt.Errorf("template %s: %w", src, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		// template.yml is metadata only; litefaas.yaml is generated below.
		if e.Name() == "template.yml" || e.Name() == "litefaas.yaml" {
			continue
		}
		raw, err := templates.FS.ReadFile(path.Join(src, e.Name()))
		if err != nil {
			return err
		}
		outName := strings.TrimSuffix(e.Name(), ".tmpl")
		if err := os.WriteFile(filepath.Join(dest, outName), raw, 0o644); err != nil {
			return err
		}
	}
	res := types.Resource{
		Name:    name,
		Kind:    kind,
		Runtime: runtime,
		Handler: ".",
		Image:   types.DefaultImage(name),
		Port:    8080,
		Timeout: "60s",
		Health:  "/healthz",
		Triggers: []types.Trigger{{
			Type: "http",
			Path: "/fn/" + name,
		}},
	}
	if err := manifest.Write(dest, res); err != nil {
		return err
	}
	return nil
}

func RelNote(dest string) string {
	return strings.TrimPrefix(dest, "./")
}
