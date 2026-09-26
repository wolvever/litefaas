package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wolvever/litefaas/internal/stackpack"
	"gopkg.in/yaml.v3"
)

// Resolution is a single-service or multi-service unit after optional detection.
type Resolution struct {
	Root     string
	Manifest *Manifest
	Multi    *Stack
	Pack     *stackpack.Pack
	Detected bool
}

// ResolveDetect is Resolve plus Phase 7 stack-pack detection.
// Precedence: --stackID > litefaas.yaml stack: > litefaas.yaml runtime/preset >
// fingerprint detection. stack.yaml (multi-service) stays explicit.
func ResolveDetect(path, stackID string) (*Resolution, error) {
	return resolveDetect(path, stackID, nil)
}

func resolveDetect(path, stackID string, cat *stackpack.Catalog) (*Resolution, error) {
	if path == "" {
		path = "."
	}
	stackID = strings.TrimSpace(stackID)
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		base := filepath.Base(path)
		if isStackName(base) {
			if stackID != "" {
				return nil, fmt.Errorf("cannot use --stack with %s", base)
			}
			s, err := LoadStack(path)
			return &Resolution{Root: filepath.Dir(path), Multi: s}, err
		}
		m, err := loadPartial(path)
		if err != nil {
			return nil, err
		}
		return finalizeOne(filepath.Dir(path), m, stackID, cat)
	}

	if _, err := os.Stat(filepath.Join(path, FileName)); err == nil {
		m, err := loadPartial(filepath.Join(path, FileName))
		if err != nil {
			return nil, err
		}
		return finalizeOne(path, m, stackID, cat)
	}
	for _, name := range []string{StackFileName, StackFileNameAlt} {
		p := filepath.Join(path, name)
		if _, err := os.Stat(p); err == nil {
			if stackID != "" {
				return nil, fmt.Errorf("cannot use --stack with %s", name)
			}
			s, err := LoadStack(p)
			return &Resolution{Root: path, Multi: s}, err
		}
	}
	return finalizeOne(path, &Manifest{}, stackID, cat)
}

func loadPartial(path string) (*Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &m, nil
}

func finalizeOne(dir string, m *Manifest, stackID string, cat *stackpack.Catalog) (*Resolution, error) {
	if m == nil {
		m = &Manifest{}
	}
	out := &Resolution{Root: dir, Manifest: m}

	needPack := stackID != "" || strings.TrimSpace(m.Stack) != "" || strings.TrimSpace(m.Runtime) == ""
	if !needPack {
		if err := m.normalize(); err != nil {
			return nil, err
		}
		return out, nil
	}

	if cat == nil {
		var err error
		cat, err = stackpack.Open()
		if err != nil {
			return nil, err
		}
	}

	id := stackID
	if id == "" {
		id = strings.TrimSpace(m.Stack)
	}
	var pack *stackpack.Pack
	if id != "" {
		p, err := cat.Get(id)
		if err != nil {
			return nil, err
		}
		pack = p
	} else {
		p, err := cat.Detect(dir)
		if err != nil {
			return nil, err
		}
		pack = p
		out.Detected = true
	}
	applyPack(m, pack, dir)
	out.Pack = pack
	if err := m.normalize(); err != nil {
		return nil, err
	}
	return out, nil
}

func applyPack(m *Manifest, p *stackpack.Pack, dir string) {
	if m == nil || p == nil {
		return
	}
	if strings.TrimSpace(m.Name) == "" {
		m.Name = stackpack.NameFromDir(dir)
	}
	if strings.TrimSpace(m.Kind) == "" {
		m.Kind = p.Kind
	}
	if strings.TrimSpace(m.Runtime) == "" {
		m.Runtime = p.Runtime
	}
	if strings.TrimSpace(m.Stack) == "" {
		m.Stack = p.ID
	}
	if m.Memory == 0 && p.Memory > 0 {
		m.Memory = p.Memory
	}
}
