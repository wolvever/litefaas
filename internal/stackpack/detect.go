package stackpack

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxFingerprintBytes = 1 << 20 // 1 MiB

// Detect returns the unique matching pack for dir, or an error if none/ambiguous.
func (c *Catalog) Detect(dir string) (*Pack, error) {
	if c == nil || len(c.packs) == 0 {
		return nil, fmt.Errorf("no stack packs loaded")
	}
	var hits []*Pack
	for _, p := range c.All() {
		if p.Matches(dir) {
			hits = append(hits, p)
		}
	}
	if len(hits) == 0 {
		return nil, fmt.Errorf(
			"no stack pack matched %s\n\nTried fingerprint packs (embedded + LITEFAAS_STACKS_DIR). None matched.\n\nNext:\n  • lf stacks                          # see pack ids\n  • lf build . --stack <id>            # force a pack\n  • echo 'stack: <id>' > litefaas.yaml # pin\n  • lf init <name> --runtime …         # or write a full litefaas.yaml\n\nHints: ensure dependency manifests exist (package.json, requirements.txt, pom.xml, go.mod, …)\nand contain the framework strings the pack expects (see templates/stacks/*/stack.yml).",
			dir,
		)
	}
	if len(hits) == 1 {
		return hits[0], nil
	}
	best := hits[0]
	tie := false
	for _, p := range hits[1:] {
		if p.Priority > best.Priority {
			best = p
			tie = false
			continue
		}
		if p.Priority == best.Priority {
			tie = true
		}
	}
	if tie {
		tied := make([]*Pack, 0, len(hits))
		for _, p := range hits {
			if p.Priority == best.Priority {
				tied = append(tied, p)
			}
		}
		parts := make([]string, 0, len(tied))
		for _, p := range tied {
			parts = append(parts, fmt.Sprintf("%s (priority %d)", p.ID, p.Priority))
		}
		example := tied[0].ID
		return nil, fmt.Errorf(
			"ambiguous stack in %s: %s\n\nMultiple packs matched with the same priority. Pin one:\n  lf build . --stack %s\n  # or\n  # litefaas.yaml → stack: %s",
			dir, strings.Join(parts, ", "), example, example,
		)
	}
	return best, nil
}

// Matches is true if any rule-set fully matches dir.
func (p *Pack) Matches(dir string) bool {
	for _, rs := range p.Match {
		if rs.matches(dir) {
			return true
		}
	}
	return false
}

func (rs RuleSet) matches(dir string) bool {
	for _, f := range rs.Files {
		if f == "" {
			continue
		}
		if !fileExists(filepath.Join(dir, filepath.FromSlash(f))) {
			return false
		}
	}
	for _, c := range rs.Contains {
		if !containsAny(dir, c) {
			return false
		}
	}
	return true
}

func containsAny(dir string, c Contains) bool {
	if c.File == "" || len(c.Any) == 0 {
		return false
	}
	raw, err := readCapped(filepath.Join(dir, filepath.FromSlash(c.File)))
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(raw))
	for _, needle := range c.Any {
		if needle == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func readCapped(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, maxFingerprintBytes+1)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return nil, err
	}
	if n > maxFingerprintBytes {
		n = maxFingerprintBytes
	}
	return buf[:n], nil
}

// FormatHints is a one-line sidecar/env reminder for CLI output.
func (p *Pack) FormatHints() string {
	if p == nil {
		return ""
	}
	var parts []string
	if len(p.Hints.Sidecars) > 0 {
		parts = append(parts, "sidecars="+strings.Join(p.Hints.Sidecars, ","))
	}
	if len(p.Hints.Env) > 0 {
		keys := make([]string, 0, len(p.Hints.Env))
		for k := range p.Hints.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts = append(parts, "env="+strings.Join(keys, ","))
	}
	return strings.Join(parts, " ")
}

// FingerprintHit records which files/contains needles matched for one RuleSet.
type FingerprintHit struct {
	Files    []string      `json:"files,omitempty"`
	Contains []ContainsHit `json:"contains,omitempty"`
}

// ContainsHit is one successful contains check (matched needle only).
type ContainsHit struct {
	File    string `json:"file"`
	Matched string `json:"matched"`
}

// ExplainMatch is like Matches but records evidence for the first matching RuleSet.
// Match semantics are unchanged.
func (p *Pack) ExplainMatch(dir string) (matched bool, hits []FingerprintHit) {
	if p == nil {
		return false, nil
	}
	for _, rs := range p.Match {
		ok, hit := rs.explain(dir)
		if ok {
			return true, []FingerprintHit{hit}
		}
	}
	return false, nil
}

func (rs RuleSet) explain(dir string) (bool, FingerprintHit) {
	hit := FingerprintHit{}
	for _, f := range rs.Files {
		if f == "" {
			continue
		}
		if !fileExists(filepath.Join(dir, filepath.FromSlash(f))) {
			return false, FingerprintHit{}
		}
		hit.Files = append(hit.Files, f)
	}
	for _, c := range rs.Contains {
		matched, needle := containsWhich(dir, c)
		if !matched {
			return false, FingerprintHit{}
		}
		hit.Contains = append(hit.Contains, ContainsHit{File: c.File, Matched: needle})
	}
	return true, hit
}

func containsWhich(dir string, c Contains) (bool, string) {
	if c.File == "" || len(c.Any) == 0 {
		return false, ""
	}
	raw, err := readCapped(filepath.Join(dir, filepath.FromSlash(c.File)))
	if err != nil {
		return false, ""
	}
	lower := strings.ToLower(string(raw))
	for _, needle := range c.Any {
		if needle == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true, needle
		}
	}
	return false, ""
}
