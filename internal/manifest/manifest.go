// Package manifest reads and writes per-service litefaas.yaml files.
package manifest

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wolvever/litefaas/internal/types"
	"gopkg.in/yaml.v3"
)

const FileName = "litefaas.yaml"

func Resolve(path string) string {
	if path == "" {
		path = "."
	}
	st, err := os.Stat(path)
	if err == nil && st.IsDir() {
		return filepath.Join(path, FileName)
	}
	return path
}

func Load(path string) (types.Resource, error) {
	file := Resolve(path)
	raw, err := os.ReadFile(file)
	if err != nil {
		return types.Resource{}, fmt.Errorf("read %s: %w", file, err)
	}
	var r types.Resource
	if err := yaml.Unmarshal(raw, &r); err != nil {
		return types.Resource{}, fmt.Errorf("parse %s: %w", file, err)
	}
	if r.Name == "" {
		return types.Resource{}, fmt.Errorf("%s: name is required", file)
	}
	kind, err := types.ParseKind(string(r.Kind))
	if err != nil {
		return types.Resource{}, err
	}
	r.Kind = kind
	rt, err := types.ParseRuntime(string(r.Runtime))
	if err != nil {
		return types.Resource{}, err
	}
	r.Runtime = rt
	if r.Image == "" {
		r.Image = types.DefaultImage(r.Name)
	}
	if r.Port == 0 {
		r.Port = 8080
	}
	if r.Health == "" {
		r.Health = "/healthz"
	}
	if r.Handler == "" {
		r.Handler = "."
	}
	return r, nil
}

func Write(path string, r types.Resource) error {
	file := Resolve(path)
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	raw, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(file, raw, 0o644)
}

func Dir(path string) string {
	file := Resolve(path)
	return filepath.Dir(file)
}
