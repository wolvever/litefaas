package netlifycompat

import (
	"strings"
)

// MatchResult is the outcome of matching a request path against an EdgeRule.
type MatchResult struct {
	Rule   EdgeRule
	Splat  string
	Params map[string]string
	// Dest is To with :splat / :param substituted (redirect/rewrite only).
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
		ok, splat, params, n := matchFrom(path, r.From)
		if !ok {
			continue
		}
		if n > bestLen {
			best = MatchResult{
				Rule:   r,
				Splat:  splat,
				Params: params,
				Dest:   ExpandDest(r.To, splat, params),
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
		ok, _, _, _ := matchFrom(path, r.From)
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

// ExpandDest substitutes :splat and :param placeholders in a destination.
func ExpandDest(to, splat string, params map[string]string) string {
	out := NormalizeSplat(to, splat)
	for k, v := range params {
		out = strings.ReplaceAll(out, ":"+k, v)
	}
	return out
}

// matchFrom supports exact paths, trailing /* splat, and single-segment :param placeholders.
// Returns matched, splat value, named params, and specificity (literal rune count).
func matchFrom(path, from string) (bool, string, map[string]string, int) {
	if from == "" {
		return false, "", nil, 0
	}
	if strings.HasSuffix(from, "/*") {
		prefix := strings.TrimSuffix(from, "/*")
		if path == prefix || path == prefix+"/" {
			return true, "", nil, len(prefix)
		}
		if prefix == "" {
			if strings.HasPrefix(path, "/") {
				return true, strings.TrimPrefix(path, "/"), nil, 1
			}
			return false, "", nil, 0
		}
		if strings.HasPrefix(path, prefix+"/") {
			return true, strings.TrimPrefix(path, prefix+"/"), nil, len(prefix)
		}
		return false, "", nil, 0
	}
	if strings.Contains(from, "/:") || strings.HasPrefix(from, ":") {
		return matchParams(path, from)
	}
	p := strings.TrimRight(from, "/")
	if p == "" {
		p = "/"
	}
	if path == from || path == p || path == p+"/" {
		return true, "", nil, len(p)
	}
	return false, "", nil, 0
}

func matchParams(path, from string) (bool, string, map[string]string, int) {
	fromParts := splitPath(from)
	pathParts := splitPath(path)
	if len(fromParts) != len(pathParts) {
		return false, "", nil, 0
	}
	params := map[string]string{}
	score := 0
	for i := range fromParts {
		fp, pp := fromParts[i], pathParts[i]
		if strings.HasPrefix(fp, ":") {
			name := strings.TrimPrefix(fp, ":")
			if name == "" || strings.Contains(name, "*") {
				return false, "", nil, 0
			}
			params[name] = pp
			continue
		}
		if fp != pp {
			return false, "", nil, 0
		}
		score += len(fp)
	}
	return true, "", params, score
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}
