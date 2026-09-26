package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	dirName    = ".litefaas"
	configName = "config.json"
	DefaultGW  = "http://127.0.0.1:8080"
)

// File is the CLI context stored under ~/.litefaas/config.json.
type File struct {
	Gateway string `json:"gateway,omitempty"`
	Token   string `json:"token,omitempty"`
}

// Dir returns ~/.litefaas (or $LITEFAAS_HOME if set).
func Dir() (string, error) {
	if home := os.Getenv("LITEFAAS_HOME"); home != "" {
		return home, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, dirName), nil
}

// Path is the CLI config file path.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configName), nil
}

// Load reads the config file. Missing file is an empty config, not an error.
func Load() (File, error) {
	p, err := Path()
	if err != nil {
		return File{}, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return File{}, nil
	}
	if err != nil {
		return File{}, err
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return File{}, err
	}
	return f, nil
}

// Save writes the config file, creating the directory if needed.
func Save(f File) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(p, b, 0o600)
}

// ResolveGateway picks --gateway, then LITEFAAS_GATEWAY, then config, then default.
func ResolveGateway(flagVal string, cfg File) string {
	return firstNonEmpty(flagVal, os.Getenv("LITEFAAS_GATEWAY"), cfg.Gateway, DefaultGW)
}

// ResolveToken picks --token, then LITEFAAS_TOKEN, then config.
func ResolveToken(flagVal string, cfg File) string {
	return firstNonEmpty(flagVal, os.Getenv("LITEFAAS_TOKEN"), cfg.Token)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
