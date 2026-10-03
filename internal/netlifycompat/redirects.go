// Package netlifycompat parses a Netlify _redirects / netlify.toml subset into edge rules.
package netlifycompat

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// EdgeRule is a gateway-handled redirect, rewrite, or header attachment.
type EdgeRule struct {
	From    string            `json:"from"`
	To      string            `json:"to,omitempty"`
	Status  int               `json:"status,omitempty"` // 0 = headers-only; 200 = rewrite; 3xx = redirect
	Headers map[string]string `json:"headers,omitempty"`
	Force   bool              `json:"force,omitempty"`  // accepted then ignored in MVP
	Source  string            `json:"source,omitempty"` // _redirects|netlify.toml
	// Project is provenance stamped by the gateway. Empty means the legacy unscoped bucket.
	Project string `json:"project,omitempty"`
}

// ParseRedirects parses Netlify _redirects lines: from to [status] [!].
// Role/query conditions and force (!) are ignored (documented skip).
func ParseRedirects(src string) ([]EdgeRule, error) {
	var out []EdgeRule
	sc := bufio.NewScanner(strings.NewReader(src))
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Drop trailing force marker for tokenization; ignore it.
		force := false
		if strings.HasSuffix(line, "!") {
			force = true
			line = strings.TrimSpace(strings.TrimSuffix(line, "!"))
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("_redirects line %d: want 'from to [status]'", lineNo)
		}
		from, to := fields[0], fields[1]
		if !strings.HasPrefix(from, "/") {
			return nil, fmt.Errorf("_redirects line %d: from must start with /", lineNo)
		}
		status := 301
		if len(fields) >= 3 {
			n, err := strconv.Atoi(fields[2])
			if err != nil {
				// Non-numeric third field (Role=… etc.) — skip conditions in MVP.
				status = 301
			} else {
				status = n
			}
		}
		if !validRedirectStatus(status) {
			return nil, fmt.Errorf("_redirects line %d: unsupported status %d", lineNo, status)
		}
		// Ignore extra condition tokens (Role=, Query=) beyond status.
		from = normalizeCaptureGroup(from)
		to = normalizeDestCaptures(to)
		out = append(out, EdgeRule{
			From:   from,
			To:     to,
			Status: status,
			Force:  force,
			Source: "_redirects",
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func validRedirectStatus(s int) bool {
	switch s {
	case 200, 301, 302, 303, 307, 308:
		return true
	default:
		return false
	}
}

// NormalizeSplat rewrites Netlify :splat placeholders to a consistent form.
func NormalizeSplat(to, splat string) string {
	to = strings.ReplaceAll(to, ":splat", splat)
	// Bare trailing /* in destination is uncommon; leave as-is.
	return to
}
