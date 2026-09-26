// Package token persists the shared bearer token (RFC-0001 §9 / Phase 6).
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

func Path(dataDir string) string {
	return filepath.Join(dataDir, FileName)
}

func Generate() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func Load(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return "", fmt.Errorf("token file %s is empty", path)
	}
	return s, nil
}

// LoadOrCreate returns the token at path, generating one if the file is missing.
func LoadOrCreate(path string) (tok string, created bool, err error) {
	tok, err = Load(path)
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", false, err
	}
	if err := os.WriteFile(path, []byte(tok+"\n"), 0o600); err != nil {
		return "", false, err
	}
	return tok, true, nil
}

// Resolve picks an explicit token, else the token file (creating it unless insecure).
func Resolve(explicit, tokenFile string, insecure bool) (tok string, created bool, err error) {
	if insecure {
		return "", false, nil
	}
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit), false, nil
	}
	return LoadOrCreate(tokenFile)
}
