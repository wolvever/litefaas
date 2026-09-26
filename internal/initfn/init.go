// Package initfn copies a runtime or preset template into a new service directory.
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
		switch string(runtime) + "/" + preset {
		case "java/spring-boot":
			return "presets/java/spring-boot", nil
		case "python/fastapi":
			return "presets/python/fastapi", nil
		default:
			return "", fmt.Errorf("unknown preset %q for runtime %s", preset, runtime)
		}
	}
	switch runtime {
	case types.RuntimeGo:
		return "runtimes/go/http", nil
	case types.RuntimeJava:
		return "runtimes/java/http", nil
	case types.RuntimePython:
		return "runtimes/python/http", nil
	case types.RuntimeStatic:
		return "frontend/static", nil
	default:
		return "", fmt.Errorf("runtime %q is not scaffolded yet (dockerfile comes in Phase 5)", runtime)
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
	if err := copyTree(src, dest); err != nil {
		return fmt.Errorf("template %s: %w", src, err)
	}
	trigPath := "/fn/" + name
	health := "/healthz"
	if kind == types.KindFrontend {
		trigPath = "/"
		health = "/"
	}
	if kind == types.KindBackend {
		trigPath = "/api/" + name
	}
	res := types.Resource{
		Name:    name,
		Kind:    kind,
		Runtime: runtime,
		Preset:  preset,
		Handler: ".",
		Image:   types.DefaultImage(name),
		Port:    8080,
		Timeout: "60s",
		Health:  health,
		Triggers: []types.Trigger{{
			Type:        "http",
			Path:        trigPath,
			SPA:         kind == types.KindFrontend,
			StripPrefix: kind == types.KindBackend,
		}},
	}
	return manifest.Write(dest, res)
}

func copyTree(src, dest string) error {
	return fs.WalkDir(templates.FS, src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, src)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return nil
		}
		name := path.Base(p)
		if name == "template.yml" || name == "litefaas.yaml" {
			return nil
		}
		target := filepath.Join(dest, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := templates.FS.ReadFile(p)
		if err != nil {
			return err
		}
		out := strings.TrimSuffix(target, ".tmpl")
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		return os.WriteFile(out, raw, 0o644)
	})
}
