// Package config is the lf CLI context file (~/.litefaas/config.yaml).
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultGateway = "http://127.0.0.1:8080"
	filename       = "config.yaml"
)

type Context struct {
	Gateway string `yaml:"gateway"`
	Token   string `yaml:"token,omitempty"`
}

type File struct {
	Current  string             `yaml:"current"`
	Contexts map[string]Context `yaml:"contexts"`
}

func DefaultDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".litefaas")
	}
	return ".litefaas"
}

func DefaultDataDir() string {
	return DefaultDir()
}

func Path(dir string) string {
	if dir == "" {
		dir = DefaultDir()
	}
	return filepath.Join(dir, filename)
}

func Load(dir string) (*File, error) {
	p := Path(dir)
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return empty(), nil
		}
		return nil, err
	}
	var f File
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	if f.Contexts == nil {
		f.Contexts = map[string]Context{}
	}
	if f.Current == "" {
		f.Current = "default"
	}
	if _, ok := f.Contexts[f.Current]; !ok {
		if f.Current == "default" {
			f.Contexts["default"] = Context{Gateway: DefaultGateway}
		}
	}
	return &f, nil
}

func (f *File) Save(dir string) error {
	if dir == "" {
		dir = DefaultDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	return os.WriteFile(Path(dir), raw, 0o600)
}

func (f *File) CurrentContext() (string, Context, error) {
	name := f.Current
	if name == "" {
		name = "default"
	}
	ctx, ok := f.Contexts[name]
	if !ok {
		if name == "default" {
			return name, Context{Gateway: DefaultGateway}, nil
		}
		return "", Context{}, fmt.Errorf("context %q not found", name)
	}
	if ctx.Gateway == "" {
		ctx.Gateway = DefaultGateway
	}
	return name, ctx, nil
}

func empty() *File {
	return &File{
		Current: "default",
		Contexts: map[string]Context{
			"default": {Gateway: DefaultGateway},
		},
	}
}
