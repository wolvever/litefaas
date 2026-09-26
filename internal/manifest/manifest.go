// Package manifest loads per-service litefaas.yaml (RFC-0001 §7).
package manifest

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wolvever/litefaas/internal/types"
	"gopkg.in/yaml.v3"
)

// Manifest is the on-disk litefaas.yaml document.
type Manifest struct {
	Name     string            `yaml:"name"`
	Kind     string            `yaml:"kind"`
	Runtime  string            `yaml:"runtime"`
	Preset   string            `yaml:"preset,omitempty"`
	Handler  string            `yaml:"handler,omitempty"`
	Image    string            `yaml:"image,omitempty"`
	Port     int               `yaml:"port,omitempty"`
	Memory   int               `yaml:"memory,omitempty"`
	Timeout  string            `yaml:"timeout,omitempty"`
	Health   string            `yaml:"health,omitempty"`
	Triggers []types.Trigger   `yaml:"triggers,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
	Build    *types.Build      `yaml:"build,omitempty"`
}

// Load reads litefaas.yaml from dir (or from a file path).
func Load(path string) (Manifest, string, error) {
	file, dir, err := resolve(path)
	if err != nil {
		return Manifest{}, "", err
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return Manifest{}, "", fmt.Errorf("read %s: %w", file, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return Manifest{}, "", fmt.Errorf("parse %s: %w", file, err)
	}
	if err := m.normalize(); err != nil {
		return Manifest{}, "", err
	}
	return m, dir, nil
}

func resolve(path string) (file, dir string, err error) {
	if path == "" {
		path = "."
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", "", fmt.Errorf("open %s: %w", path, err)
	}
	if info.IsDir() {
		return filepath.Join(path, types.ManifestFile), path, nil
	}
	return path, filepath.Dir(path), nil
}

func (m *Manifest) normalize() error {
	if !types.ValidName(m.Name) {
		return fmt.Errorf("litefaas.yaml: name must match [A-Za-z0-9][A-Za-z0-9._-]{0,63}")
	}
	kind, err := types.ParseKind(m.Kind)
	if err != nil {
		return fmt.Errorf("litefaas.yaml: %w", err)
	}
	m.Kind = string(kind)
	rt, err := types.ParseRuntime(m.Runtime)
	if err != nil {
		return fmt.Errorf("litefaas.yaml: %w", err)
	}
	m.Runtime = string(rt)
	if m.Image == "" {
		m.Image = types.DefaultImage(m.Name)
	}
	if m.Port == 0 {
		m.Port = 8080
	}
	if m.Health == "" {
		m.Health = "/healthz"
	}
	if m.Handler == "" {
		m.Handler = "."
	}
	if m.Timeout == "" && kind == types.KindFunction {
		m.Timeout = "60s"
	}
	return nil
}

// Resource converts the manifest to control-plane metadata.
func (m Manifest) Resource() types.Resource {
	return types.Resource{
		Name:     m.Name,
		Kind:     types.Kind(m.Kind),
		Runtime:  types.Runtime(m.Runtime),
		Preset:   m.Preset,
		Handler:  m.Handler,
		Image:    m.Image,
		Port:     m.Port,
		Memory:   m.Memory,
		Timeout:  m.Timeout,
		Health:   m.Health,
		Triggers: m.Triggers,
		Env:      m.Env,
		Build:    m.Build,
	}
}

// Dockerfile returns the build-context Dockerfile path if present.
func Dockerfile(dir string) string {
	return filepath.Join(dir, "Dockerfile")
}
