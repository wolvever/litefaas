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
	DefaultEnv  = "default"
)

var (
	ErrNotFound = errors.New("secret not found")
	nameRE      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	envRE       = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
	refRE       = regexp.MustCompile(`\$\{secret:([A-Za-z0-9][A-Za-z0-9._-]{0,63})\}`)
)

// Store persists AES-GCM ciphertext files under data-dir/secrets/<env>/.
type Store struct {
	dir  string
	aead cipher.AEAD
}

// Open loads or creates the host key under dataDir and returns a Store.
// Flat secrets/*.enc files are migrated once into secrets/default/.
func Open(dataDir string) (*Store, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("data-dir is required")
	}
	secretsRoot := filepath.Join(dataDir, DirName)
	if err := os.MkdirAll(secretsRoot, 0o700); err != nil {
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
	s := &Store{dir: secretsRoot, aead: aead}
	if err := s.migrateFlatToDefault(); err != nil {
		return nil, err
	}
	return s, nil
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

// ValidateEnv checks a secret env bag name (not CLI gateway context).
func ValidateEnv(env string) error {
	if env == "" {
		return fmt.Errorf("secret env is empty")
	}
	if !envRE.MatchString(env) {
		return fmt.Errorf("invalid secret env %q", env)
	}
	return nil
}

// NormalizeEnv returns DefaultEnv when env is empty; otherwise validates.
func NormalizeEnv(env string) (string, error) {
	if env == "" {
		return DefaultEnv, nil
	}
	if err := ValidateEnv(env); err != nil {
		return "", err
	}
	return env, nil
}

func (s *Store) migrateFlatToDefault() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	var flat []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".enc") {
			flat = append(flat, n)
		}
	}
	if len(flat) == 0 {
		return nil
	}
	destDir := filepath.Join(s.dir, DefaultEnv)
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return err
	}
	for _, n := range flat {
		src := filepath.Join(s.dir, n)
		dst := filepath.Join(destDir, n)
		if _, err := os.Stat(dst); err == nil {
			// Already migrated; drop leftover flat file.
			_ = os.Remove(src)
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) path(env, name string) string {
	return filepath.Join(s.dir, env, name+".enc")
}

// Set encrypts and writes value for name in the default env (overwrites).
func (s *Store) Set(name, value string) error {
	return s.SetEnv(DefaultEnv, name, value)
}

// SetEnv encrypts and writes value for name in env.
func (s *Store) SetEnv(env, name, value string) error {
	env, err := NormalizeEnv(env)
	if err != nil {
		return err
	}
	if err := ValidateName(name); err != nil {
		return err
	}
	if value == "" {
		return fmt.Errorf("secret value is empty")
	}
	if err := os.MkdirAll(filepath.Join(s.dir, env), 0o700); err != nil {
		return err
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	ct := s.aead.Seal(nonce, nonce, []byte(value), nil)
	return os.WriteFile(s.path(env, name), ct, 0o600)
}

// Get decrypts and returns the plaintext value from the default env.
func (s *Store) Get(name string) (string, error) {
	return s.GetEnv(DefaultEnv, name)
}

// GetEnv decrypts and returns the plaintext value from env.
func (s *Store) GetEnv(env, name string) (string, error) {
	env, err := NormalizeEnv(env)
	if err != nil {
		return "", err
	}
	if err := ValidateName(name); err != nil {
		return "", err
	}
	raw, err := os.ReadFile(s.path(env, name))
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

// Delete removes a secret file from the default env.
func (s *Store) Delete(name string) error {
	return s.DeleteEnv(DefaultEnv, name)
}

// DeleteEnv removes a secret file from env.
func (s *Store) DeleteEnv(env, name string) error {
	env, err := NormalizeEnv(env)
	if err != nil {
		return err
	}
	if err := ValidateName(name); err != nil {
		return err
	}
	err = os.Remove(s.path(env, name))
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	return err
}

// List returns sorted secret names in the default env (no values).
func (s *Store) List() ([]string, error) {
	return s.ListEnv(DefaultEnv)
}

// ListEnv returns sorted secret names in env (no values).
func (s *Store) ListEnv(env string) ([]string, error) {
	env, err := NormalizeEnv(env)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(s.dir, env))
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
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

// GetterFor returns a Get closure scoped to env (for ResolveEnv).
func (s *Store) GetterFor(env string) (func(string) (string, error), error) {
	env, err := NormalizeEnv(env)
	if err != nil {
		return nil, err
	}
	e := env
	return func(name string) (string, error) {
		return s.GetEnv(e, name)
	}, nil
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

// MergeBag copies secret bag entries into dst for keys not already present.
// Existing keys in dst win. Errors mention names only — never plaintext values.
func MergeBag(dst map[string]string, names []string, get func(string) (string, error)) (map[string]string, error) {
	if get == nil {
		return nil, fmt.Errorf("secret store not configured")
	}
	out := make(map[string]string, len(dst)+len(names))
	for k, v := range dst {
		out[k] = v
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		if _, ok := out[name]; ok {
			continue
		}
		val, err := get(name)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, fmt.Errorf("unknown secret %q", name)
			}
			return nil, fmt.Errorf("secret %q: %w", name, err)
		}
		out[name] = val
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

// EnvRef is an env key that references a secret by name (value never included).
type EnvRef struct {
	Key  string
	Name string // secret name inside ${secret:name}
}

// ListRefs returns env keys whose values contain ${secret:…}, sorted by key.
// Plaintext values are omitted (never returned).
func ListRefs(env map[string]string) []EnvRef {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []EnvRef
	for _, k := range keys {
		v := env[k]
		for _, sub := range refRE.FindAllStringSubmatch(v, -1) {
			if len(sub) < 2 {
				continue
			}
			out = append(out, EnvRef{Key: k, Name: sub[1]})
		}
	}
	return out
}

// FormatRef is KEY=${secret:name} for summary printing.
func (r EnvRef) FormatRef() string {
	return r.Key + "=${secret:" + r.Name + "}"
}
