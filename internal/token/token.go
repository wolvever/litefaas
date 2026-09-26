// Package token persists the shared bearer token for litefaasd / lf (RFC-0001 §9).
package token

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const FileName = "token"

// Generate returns a 256-bit hex token.
func Generate() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func Path(dir string) string {
	return filepath.Join(dir, FileName)
}

func ReadFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	t := strings.TrimSpace(string(raw))
	if t == "" {
		return "", fmt.Errorf("token file %s is empty", path)
	}
	return t, nil
}

func WriteFile(path, tok string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(tok+"\n"), 0o600)
}

// LoadOrCreate reads path, or writes a new token if the file is missing.
func LoadOrCreate(path string) (tok string, created bool, err error) {
	tok, err = ReadFile(path)
	if err == nil {
		return tok, false, nil
	}
	if !os.IsNotExist(err) {
		return "", false, err
	}
	tok, err = Generate()
	if err != nil {
		return "", false, err
	}
	if err := WriteFile(path, tok); err != nil {
		return "", false, err
	}
	return tok, true, nil
}

// Info describes how the daemon resolved its token.
type Info struct {
	Mode      string // off | explicit | file
	Path      string
	Generated bool
}

// ResolveDaemon picks --token / LITEFAAS_TOKEN, else {data-dir}/token (created if needed).
func ResolveDaemon(explicit, dataDir string, noAuth bool) (string, Info, error) {
	if noAuth {
		return "", Info{Mode: "off"}, nil
	}
	if t := strings.TrimSpace(explicit); t != "" {
		return t, Info{Mode: "explicit"}, nil
	}
	if dataDir == "" {
		return "", Info{}, fmt.Errorf("data-dir is required to persist a token")
	}
	p := Path(dataDir)
	tok, created, err := LoadOrCreate(p)
	if err != nil {
		return "", Info{}, err
	}
	return tok, Info{Mode: "file", Path: p, Generated: created}, nil
}

// ResolveClient is flag > LITEFAAS_TOKEN > context token > {config-dir}/token.
func ResolveClient(flag, env, contextTok, configDir string) string {
	if t := strings.TrimSpace(flag); t != "" {
		return t
	}
	if t := strings.TrimSpace(env); t != "" {
		return t
	}
	if t := strings.TrimSpace(contextTok); t != "" {
		return t
	}
	if configDir == "" {
		return ""
	}
	tok, err := ReadFile(Path(configDir))
	if err != nil {
		return ""
	}
	return tok
}
