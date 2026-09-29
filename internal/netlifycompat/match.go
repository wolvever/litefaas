package netlifycompat

import (
	"strings"
)

// MatchResult is the outcome of matching a request path against an EdgeRule.
type MatchResult struct {
	Rule  EdgeRule
	Splat string
	// Dest is To with :splat substituted (redirect/rewrite only).
	Dest string
}

// MatchPath finds the best (longest From) matching rule among redirects/rewrites
// (Status != 0). Header-only rules are handled by MatchHeaders.
func MatchPath(path string, rules []EdgeRule) (MatchResult, bool) {
	var best MatchResult
	bestLen := -1
	for _, r := range rules {
		if r.Status == 0 {
			continue
		}
		ok, splat, n := matchFrom(path, r.From)
		if !ok {
			continue
		}
		if n > bestLen {
			best = MatchResult{
				Rule:  r,
				Splat: splat,
				Dest:  NormalizeSplat(r.To, splat),
			}
			bestLen = n
		}
	}
	if bestLen < 0 {
		return MatchResult{}, false
	}
	return best, true
}

// MatchHeaders returns merged headers from all matching header-only rules.
func MatchHeaders(path string, rules []EdgeRule) map[string]string {
	var out map[string]string
	for _, r := range rules {
		if r.Status != 0 || len(r.Headers) == 0 {
			continue
		}
		ok, _, _ := matchFrom(path, r.From)
		if !ok {
			continue
		}
		if out == nil {
			out = map[string]string{}
		}
		for k, v := range r.Headers {
			out[k] = v
		}
	}
	return out
}

// matchFrom supports exact paths and a single trailing /* splat (Netlify-style).
// Returns matched, splat value, and specificity score (length of literal prefix).
func matchFrom(path, from string) (bool, string, int) {
	if from == "" {
		return false, "", 0
	}
	if strings.HasSuffix(from, "/*") {
		prefix := strings.TrimSuffix(from, "/*")
		if prefix == "" {
			prefix = ""
		}
		if path == prefix || path == prefix+"/" {
			return true, "", len(prefix)
		}
		if prefix == "" {
			if strings.HasPrefix(path, "/") {
				return true, strings.TrimPrefix(path, "/"), 1
			}
			return false, "", 0
		}
		if strings.HasPrefix(path, prefix+"/") {
			return true, strings.TrimPrefix(path, prefix+"/"), len(prefix)
		}
		return false, "", 0
	}
	if from == "/*" {
		if strings.HasPrefix(path, "/") {
			return true, strings.TrimPrefix(path, "/"), 1
		}
		return false, "", 0
	}
	p := strings.TrimRight(from, "/")
	if p == "" {
		p = "/"
	}
	if path == from || path == p || path == p+"/" {
		return true, "", len(p)
	}
	return false, "", 0
}
