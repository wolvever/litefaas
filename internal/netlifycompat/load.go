package netlifycompat

import (
	"fmt"
	"os"
	"path/filepath"
)

// LoadProject reads root _redirects and/or netlify.toml when present.
// Order: _redirects first, then netlify.toml (later From keys can override at merge time).
func LoadProject(dir string) ([]EdgeRule, error) {
	if dir == "" {
		dir = "."
	}
	var out []EdgeRule
	rd := filepath.Join(dir, "_redirects")
	if raw, err := os.ReadFile(rd); err == nil {
		rules, err := ParseRedirects(string(raw))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rd, err)
		}
		out = append(out, rules...)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	nt := filepath.Join(dir, "netlify.toml")
	if raw, err := os.ReadFile(nt); err == nil {
		rules, err := ParseNetlifyTOML(string(raw))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", nt, err)
		}
		out = append(out, rules...)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return out, nil
}

// MergeByFrom appends rules; later rules with the same From replace earlier ones.
// Header-only rules (Status==0) merge headers into an existing headers-only rule with the same From.
func MergeByFrom(rules []EdgeRule) []EdgeRule {
	order := make([]string, 0, len(rules))
	idx := map[string]int{}
	var out []EdgeRule
	for _, r := range rules {
		key := r.From
		if r.Status == 0 {
			key = "hdr:" + r.From
		} else {
			key = "redir:" + r.From
		}
		if i, ok := idx[key]; ok {
			if r.Status == 0 && out[i].Status == 0 {
				if out[i].Headers == nil {
					out[i].Headers = map[string]string{}
				}
				for k, v := range r.Headers {
					out[i].Headers[k] = v
				}
				out[i].Source = r.Source
				continue
			}
			out[i] = r
			continue
		}
		idx[key] = len(out)
		order = append(order, key)
		out = append(out, r)
	}
	_ = order
	return out
}
