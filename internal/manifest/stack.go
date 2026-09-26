package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	StackFileName    = "stack.yaml"
	StackFileNameAlt = "stack.yml"
)

// Stack is a multi-service stack.yaml (RFC-0001 §7 / §15 Phase 6).
type Stack struct {
	Path     string
	Services []Manifest
}

type stackFile struct {
	Services yaml.Node `yaml:"services"`
}

func LoadStack(path string) (*Stack, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sf stackFile
	if err := yaml.Unmarshal(raw, &sf); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	services, err := parseStackServices(&sf.Services)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("%s: services is required", filepath.Base(path))
	}
	for i := range services {
		if err := services[i].normalize(); err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
	}
	return &Stack{Path: path, Services: services}, nil
}

func parseStackServices(n *yaml.Node) ([]Manifest, error) {
	if n == nil || n.Kind == 0 {
		return nil, fmt.Errorf("services is required")
	}
	switch n.Kind {
	case yaml.SequenceNode:
		var list []Manifest
		if err := n.Decode(&list); err != nil {
			return nil, err
		}
		return list, nil
	case yaml.MappingNode:
		var out []Manifest
		for i := 0; i+1 < len(n.Content); i += 2 {
			name := strings.TrimSpace(n.Content[i].Value)
			var m Manifest
			if err := n.Content[i+1].Decode(&m); err != nil {
				return nil, err
			}
			if m.Name == "" {
				m.Name = name
			}
			if name != "" && m.Name != name {
				return nil, fmt.Errorf("service %q: name %q does not match key", name, m.Name)
			}
			out = append(out, m)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("services must be a map or list")
	}
}

// Resolve loads litefaas.yaml (preferred) or stack.yaml from a path or directory.
func Resolve(path string) (root string, m *Manifest, stack *Stack, err error) {
	if path == "" {
		path = "."
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", nil, nil, err
	}
	if !info.IsDir() {
		base := filepath.Base(path)
		if isStackName(base) {
			s, err := LoadStack(path)
			return filepath.Dir(path), nil, s, err
		}
		m, err := Load(path)
		return filepath.Dir(path), m, nil, err
	}
	if _, err := os.Stat(filepath.Join(path, FileName)); err == nil {
		m, err := Load(filepath.Join(path, FileName))
		return path, m, nil, err
	}
	for _, name := range []string{StackFileName, StackFileNameAlt} {
		p := filepath.Join(path, name)
		if _, err := os.Stat(p); err == nil {
			s, err := LoadStack(p)
			return path, nil, s, err
		}
	}
	return "", nil, nil, fmt.Errorf("no %s or %s in %s", FileName, StackFileName, path)
}

func isStackName(base string) bool {
	return base == StackFileName || base == StackFileNameAlt
}

// ServiceDir is the docker build context for a stack service (handler relative to stack root).
func ServiceDir(root string, m *Manifest) string {
	h := strings.TrimSpace(m.Handler)
	if h == "" || h == "." {
		return root
	}
	if filepath.IsAbs(h) {
		return h
	}
	return filepath.Join(root, h)
}
