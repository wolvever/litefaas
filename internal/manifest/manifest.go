// Package manifest reads per-service litefaas.yaml (RFC-0001 §7).
package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wolvever/litefaas/internal/types"
	"gopkg.in/yaml.v3"
)

const FileName = "litefaas.yaml"

// Manifest is the on-disk shape of litefaas.yaml.
type Manifest struct {
	Name     string            `yaml:"name"`
	Kind     string            `yaml:"kind"`
	Runtime  string            `yaml:"runtime"`
	Preset   string            `yaml:"preset,omitempty"`
	Stack    string            `yaml:"stack,omitempty"` // Phase 7 pack id (optional override)
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

func Load(path string) (*Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := m.normalize(); err != nil {
		return nil, err
	}
	return &m, nil
}

// LoadDir finds litefaas.yaml in dir (or dir itself if it is the file).
func LoadDir(dir string) (*Manifest, string, error) {
	if dir == "" {
		dir = "."
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, "", err
	}
	path := dir
	if info.IsDir() {
		path = filepath.Join(dir, FileName)
	}
	m, err := Load(path)
	if err != nil {
		return nil, "", err
	}
	return m, filepath.Dir(path), nil
}

func (m *Manifest) normalize() error {
	m.Name = strings.TrimSpace(m.Name)
	if m.Name == "" {
		return fmt.Errorf("%s: name is required", FileName)
	}
	kind, err := types.ParseKind(m.Kind)
	if err != nil {
		return err
	}
	m.Kind = string(kind)
	rt, err := types.ParseRuntime(m.Runtime)
	if err != nil {
		return err
	}
	m.Runtime = string(rt)
	if m.Handler == "" {
		m.Handler = "."
	}
	if m.Image == "" {
		m.Image = m.Name + ":latest"
	}
	if m.Port == 0 {
		m.Port = 8080
	}
	if m.Memory == 0 {
		m.Memory = types.DefaultMemoryMiB
	}
	if err := ValidateMemory(m.Memory); err != nil {
		return err
	}
	if m.Timeout == "" {
		m.Timeout = "30s"
	}
	if _, err := ParseTimeout(m.Timeout); err != nil {
		return err
	}
	if m.Health == "" {
		if m.Kind == string(types.KindFrontend) {
			m.Health = "/"
		} else {
			m.Health = "/healthz"
		}
	}
	return nil
}

func (m *Manifest) Resource() types.Resource {
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

func ParseTimeout(s string) (time.Duration, error) {
	if s == "" {
		return types.DefaultTimeout, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("timeout %q: %w", s, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("timeout must be positive")
	}
	if d > types.MaxTimeout {
		return 0, fmt.Errorf("timeout %s exceeds max %s", d, types.MaxTimeout)
	}
	return d, nil
}

func ValidateMemory(mib int) error {
	if mib < types.MinMemoryMiB || mib > types.MaxMemoryMiB {
		return fmt.Errorf("memory must be %d–%d MiB, got %d", types.MinMemoryMiB, types.MaxMemoryMiB, mib)
	}
	return nil
}
