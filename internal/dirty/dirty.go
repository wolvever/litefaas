// Package dirty hashes service directories so stack builds can skip unchanged services.
package dirty

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RootLockfiles live at the stack root. Any content or presence change rebuilds every service.
var RootLockfiles = []string{
	"package-lock.json",
	"pnpm-lock.yaml",
	"yarn.lock",
	"bun.lock",
	"go.sum",
	"poetry.lock",
	"uv.lock",
	"Cargo.lock",
	"composer.lock",
	"Gemfile.lock",
}

var skipDir = map[string]struct{}{
	".git":         {},
	"node_modules": {},
	"vendor":       {},
	"__pycache__":  {},
	".litefaas":    {},
}

// State is persisted at <root>/.litefaas/service-hashes.json.
type State struct {
	Lock     string            `json:"lock"`
	Services map[string]string `json:"services"`
}

func Load(path string) (State, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{Services: map[string]string{}}, nil
		}
		return State{}, err
	}
	var st State
	if err := json.Unmarshal(raw, &st); err != nil {
		return State{}, err
	}
	if st.Services == nil {
		st.Services = map[string]string{}
	}
	return st, nil
}

func Save(path string, st State) error {
	if st.Services == nil {
		st.Services = map[string]string{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o644)
}

// HashDir is a stable content hash of dir. Symlinks are not followed.
func HashDir(dir string) (string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != dir {
				if _, skip := skipDir[name]; skip {
					return filepath.SkipDir
				}
			}
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	h := sha256.New()
	for _, rel := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		info, err := os.Lstat(full)
		if err != nil {
			return "", err
		}
		_, _ = io.WriteString(h, rel)
		h.Write([]byte{0})
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(full)
			if err != nil {
				return "", err
			}
			_, _ = io.WriteString(h, "link:"+target)
		} else {
			f, err := os.Open(full)
			if err != nil {
				return "", err
			}
			_, err = io.Copy(h, f)
			_ = f.Close()
			if err != nil {
				return "", err
			}
		}
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// LockHash covers root lockfile bytes and which of them exist.
func LockHash(root string) (string, error) {
	h := sha256.New()
	for _, name := range RootLockfiles {
		_, _ = io.WriteString(h, name)
		h.Write([]byte{0})
		full := filepath.Join(root, name)
		raw, err := os.ReadFile(full)
		if err != nil {
			if os.IsNotExist(err) {
				_, _ = io.WriteString(h, "-")
				continue
			}
			return "", err
		}
		_, _ = io.WriteString(h, "+")
		h.Write(raw)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Service is one stack.yaml unit.
type Service struct {
	Name string
	Dir  string
}

// Plan reports which services need a rebuild. A root lockfile change marks all of them.
func Plan(root string, prev State, svcs []Service) (rebuild map[string]bool, next State, err error) {
	lock, err := LockHash(root)
	if err != nil {
		return nil, State{}, err
	}
	force := prev.Lock != "" && prev.Lock != lock
	next = State{Lock: lock, Services: map[string]string{}}
	rebuild = map[string]bool{}
	for _, svc := range svcs {
		sum, err := HashDir(svc.Dir)
		if err != nil {
			return nil, State{}, err
		}
		next.Services[svc.Name] = sum
		if force || prev.Services[svc.Name] != sum {
			rebuild[svc.Name] = true
		}
	}
	return rebuild, next, nil
}

// StatePath is the cache file for a stack root.
func StatePath(root string) string {
	return filepath.Join(root, ".litefaas", "service-hashes.json")
}

// ChangedLock is true when the saved lock hash differs from root (and a previous hash exists).
func ChangedLock(prev State, lock string) bool {
	return prev.Lock != "" && prev.Lock != lock && !strings.EqualFold(prev.Lock, lock)
}
