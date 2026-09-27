// Package secret is a file-backed encrypt-at-rest secret store (RFC-0001 §13).
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	KeyFileName = "secrets.key"
	DirName     = "secrets"
)

var (
	ErrNotFound = errors.New("secret not found")
	nameRE      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	refRE       = regexp.MustCompile(`\$\{secret:([A-Za-z0-9][A-Za-z0-9._-]{0,63})\}`)
)

// Store persists AES-GCM ciphertext files under data-dir/secrets/.
type Store struct {
	dir  string
	aead cipher.AEAD
}

// Open loads or creates the host key under dataDir and returns a Store.
func Open(dataDir string) (*Store, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("data-dir is required")
	}
	if err := os.MkdirAll(filepath.Join(dataDir, DirName), 0o700); err != nil {
		return nil, err
	}
	key, err := loadOrCreateKey(filepath.Join(dataDir, KeyFileName))
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Store{dir: filepath.Join(dataDir, DirName), aead: aead}, nil
}

func loadOrCreateKey(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err == nil {
		key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
		if err != nil {
			return nil, fmt.Errorf("secrets key file: %w", err)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("secrets key file: want 32 bytes")
		}
		return key, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	enc := base64.StdEncoding.EncodeToString(key) + "\n"
	if err := os.WriteFile(path, []byte(enc), 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func ValidateName(name string) error {
	if !nameRE.MatchString(name) {
		return fmt.Errorf("invalid secret name %q", name)
	}
	return nil
}

func (s *Store) path(name string) string {
	return filepath.Join(s.dir, name+".enc")
}

// Set encrypts and writes value for name (overwrites).
func (s *Store) Set(name, value string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if value == "" {
		return fmt.Errorf("secret value is empty")
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	ct := s.aead.Seal(nonce, nonce, []byte(value), nil)
	return os.WriteFile(s.path(name), ct, 0o600)
}

// Get decrypts and returns the plaintext value.
func (s *Store) Get(name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	raw, err := os.ReadFile(s.path(name))
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNotFound
		}
		return "", err
	}
	ns := s.aead.NonceSize()
	if len(raw) < ns {
		return "", fmt.Errorf("secret %s: ciphertext too short", name)
	}
	pt, err := s.aead.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("secret %s: decrypt failed", name)
	}
	return string(pt), nil
}

// Delete removes a secret file.
func (s *Store) Delete(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	err := os.Remove(s.path(name))
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	return err
}

// List returns sorted secret names (no values).
func (s *Store) List() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".enc") {
			names = append(names, strings.TrimSuffix(n, ".enc"))
		}
	}
	sort.Strings(names)
	return names, nil
}

// ResolveEnv expands ${secret:name} references in env values.
// Errors mention the secret name only — never plaintext.
func ResolveEnv(env map[string]string, get func(string) (string, error)) (map[string]string, error) {
	if len(env) == 0 {
		return env, nil
	}
	if get == nil {
		return nil, fmt.Errorf("secret store not configured")
	}
	out := make(map[string]string, len(env))
	for k, v := range env {
		resolved, err := resolveValue(v, get)
		if err != nil {
			return nil, err
		}
		out[k] = resolved
	}
	return out, nil
}

func resolveValue(v string, get func(string) (string, error)) (string, error) {
	var first error
	out := refRE.ReplaceAllStringFunc(v, func(m string) string {
		sub := refRE.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		name := sub[1]
		val, err := get(name)
		if err != nil {
			if first == nil {
				if errors.Is(err, ErrNotFound) {
					first = fmt.Errorf("unknown secret %q", name)
				} else {
					first = fmt.Errorf("secret %q: %w", name, err)
				}
			}
			return m
		}
		return val
	})
	if first != nil {
		return "", first
	}
	return out, nil
}
