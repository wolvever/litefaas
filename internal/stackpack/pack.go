// Package stackpack loads fingerprint + Dockerfile packs for Phase 7 detection.
// Packs are YAML data (embedded or on disk). The daemon does not import this package.
package stackpack

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wolvever/litefaas/templates"
	"gopkg.in/yaml.v3"
)

const FileName = "stack.yml"

// Pack is one stack recipe: fingerprints, defaults, optional sidecar hints, Dockerfile.
type Pack struct {
	ID          string    `yaml:"id"`
	Title       string    `yaml:"title"`
	Language    string    `yaml:"language"`
	Runtime     string    `yaml:"runtime"`
	Kind        string    `yaml:"kind"`
	Memory      int       `yaml:"memory,omitempty"`
	Priority    int       `yaml:"priority,omitempty"`
	Description string    `yaml:"description,omitempty"`
	Match       []RuleSet `yaml:"match"`
	Hints       Hints     `yaml:"hints,omitempty"`
	Host        Host      `yaml:"host,omitempty"`
	Verify      Verify    `yaml:"verify,omitempty"`
	// Release is optional idempotent commands. Packs must not ship a framework migrate by default.
	Release []string `yaml:"release,omitempty"`

	Dockerfile   []byte `yaml:"-"`
	Dockerignore []byte `yaml:"-"`
}

// Host holds optional host-side preflight recipes (argv lists; cwd = service root).
type Host struct {
	Build []string `yaml:"build,omitempty"`
}

// Verify holds optional smoke/health defaults for lf check.
type Verify struct {
	Health string `yaml:"health,omitempty"`
}

// RuleSet is an AND-group. A pack matches if any RuleSet fully matches.
type RuleSet struct {
	Files    []string   `yaml:"files"`
	Contains []Contains `yaml:"contains"`
}

// Contains requires file to exist and include at least one of Any (substring).
type Contains struct {
	File string   `yaml:"file"`
	Any  []string `yaml:"any"`
}

// Hints are documented / printed soft defaults. They are not injected into the
// container and the platform does not start these sidecars.
type Hints struct {
	Sidecars []string          `yaml:"sidecars,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
}

// Catalog is the loaded set of packs (id → pack).
type Catalog struct {
	packs map[string]*Pack
}

func NewCatalog() *Catalog {
	return &Catalog{packs: map[string]*Pack{}}
}

func (c *Catalog) Add(p *Pack) error {
	if p == nil || strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("stack pack: id is required")
	}
	p.ID = strings.TrimSpace(p.ID)
	if p.Runtime == "" {
		return fmt.Errorf("stack pack %s: runtime is required", p.ID)
	}
	if len(p.Match) == 0 {
		return fmt.Errorf("stack pack %s: match is required", p.ID)
	}
	if p.Kind == "" {
		p.Kind = "backend"
	}
	c.packs[p.ID] = p
	return nil
}

func (c *Catalog) Get(id string) (*Pack, error) {
	p, ok := c.packs[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("unknown stack %q (want %s)", id, strings.Join(c.IDs(), "|"))
	}
	return p, nil
}

func (c *Catalog) IDs() []string {
	ids := make([]string, 0, len(c.packs))
	for id := range c.packs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (c *Catalog) All() []*Pack {
	out := make([]*Pack, 0, len(c.packs))
	for _, id := range c.IDs() {
		out = append(out, c.packs[id])
	}
	return out
}

// LoadFS reads <root>/*/stack.yml from an embed or disk fs.
func LoadFS(fsys fs.FS, root string) (*Catalog, error) {
	c := NewCatalog()
	entries, err := fs.ReadDir(fsys, root)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := path.Join(root, e.Name())
		p, err := loadPack(fsys, dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if err := c.Add(p); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// LoadDir reads packs from a filesystem directory (LITEFAAS_STACKS_DIR overlay).
func LoadDir(dir string) (*Catalog, error) {
	return LoadFS(os.DirFS(dir), ".")
}

func loadPack(fsys fs.FS, dir string) (*Pack, error) {
	raw, err := fs.ReadFile(fsys, path.Join(dir, FileName))
	if err != nil {
		return nil, err
	}
	var p Pack
	if err := yaml.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path.Join(dir, FileName), err)
	}
	if p.ID == "" {
		p.ID = path.Base(dir)
	}
	if df, err := fs.ReadFile(fsys, path.Join(dir, "Dockerfile")); err == nil {
		p.Dockerfile = df
	}
	if ig, err := fs.ReadFile(fsys, path.Join(dir, ".dockerignore")); err == nil {
		p.Dockerignore = ig
	}
	return &p, nil
}

// Merge copies packs from other, replacing the same id.
func (c *Catalog) Merge(other *Catalog) {
	if other == nil {
		return
	}
	for id, p := range other.packs {
		c.packs[id] = p
	}
}

// OpenEmbedded loads first-party packs from templates/stacks.
func OpenEmbedded() (*Catalog, error) {
	return LoadFS(templates.Stacks, "stacks")
}

// Open loads embedded packs, then optional LITEFAAS_STACKS_DIR (same id wins).
func Open() (*Catalog, error) {
	c, err := OpenEmbedded()
	if err != nil {
		return nil, err
	}
	if dir := strings.TrimSpace(os.Getenv("LITEFAAS_STACKS_DIR")); dir != "" {
		extra, err := LoadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("LITEFAAS_STACKS_DIR: %w", err)
		}
		c.Merge(extra)
	}
	return c, nil
}

func (p *Pack) WriteDockerfile(dest string) error {
	if len(p.Dockerfile) == 0 {
		return fmt.Errorf("stack %s has no Dockerfile", p.ID)
	}
	if err := os.WriteFile(filepath.Join(dest, "Dockerfile"), p.Dockerfile, 0o644); err != nil {
		return err
	}
	if len(p.Dockerignore) == 0 {
		return nil
	}
	ig := filepath.Join(dest, ".dockerignore")
	if _, err := os.Stat(ig); err == nil {
		return nil
	}
	return os.WriteFile(ig, p.Dockerignore, 0o644)
}
