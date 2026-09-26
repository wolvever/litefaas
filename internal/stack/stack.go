// Package stack reads optional multi-service stack.yaml (RFC-0001 §7 / Phase 6).
package stack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const FileName = "stack.yaml"

type Service struct {
	Path string `yaml:"path"`
}

type Stack struct {
	Name     string    `yaml:"name"`
	Services []Service `yaml:"services"`
}

func Load(path string) (*Stack, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Stack
	if err := yaml.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := s.normalize(); err != nil {
		return nil, err
	}
	return &s, nil
}

// LoadDir finds stack.yaml in dir (or dir itself if it is the file).
func LoadDir(dir string) (*Stack, string, error) {
	if dir == "" {
		dir = "."
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, "", err
	}
	path := dir
	root := dir
	if info.IsDir() {
		path = filepath.Join(dir, FileName)
		root = dir
	} else {
		root = filepath.Dir(path)
	}
	s, err := Load(path)
	if err != nil {
		return nil, "", err
	}
	return s, root, nil
}

func (s *Stack) normalize() error {
	s.Name = strings.TrimSpace(s.Name)
	if len(s.Services) == 0 {
		return fmt.Errorf("%s: services is required", FileName)
	}
	for i, svc := range s.Services {
		p := strings.TrimSpace(svc.Path)
		if p == "" {
			return fmt.Errorf("%s: services[%d].path is required", FileName, i)
		}
		s.Services[i].Path = p
	}
	return nil
}

func (s *Stack) ServiceDirs(root string) []string {
	var out []string
	for _, svc := range s.Services {
		p := svc.Path
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		out = append(out, p)
	}
	return out
}
