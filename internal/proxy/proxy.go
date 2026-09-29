// Package proxy is the tiny path-based edge router (RFC-0001 §12 / §17).
package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/wolvever/litefaas/internal/netlifycompat"
	"github.com/wolvever/litefaas/internal/types"
)

type Route struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Endpoint    string `json:"endpoint,omitempty"`
	StripPrefix bool   `json:"strip_prefix,omitempty"`
	SPA         bool   `json:"spa,omitempty"`
}

// EdgeRule is the gateway JSON shape for redirects/rewrites/headers (alias).
type EdgeRule = netlifycompat.EdgeRule

func matchPrefix(path, routePath string) bool {
	if routePath == "" {
		return false
	}
	if routePath == "/" {
		return strings.HasPrefix(path, "/")
	}
	p := strings.TrimRight(routePath, "/")
	return path == p || path == p+"/" || strings.HasPrefix(path, p+"/")
}

func Match(path string, routes []Route) (Route, bool) {
	var best Route
	bestLen := -1
	for _, r := range routes {
		if !matchPrefix(path, r.Path) {
			continue
		}
		n := len(strings.TrimRight(r.Path, "/"))
		if r.Path == "/" {
			n = 1
		}
		if n > bestLen {
			best = r
			bestLen = n
		}
	}
	if bestLen < 0 {
		return Route{}, false
	}
	return best, true
}

func stripPath(path, prefix string) string {
	if prefix == "" || prefix == "/" {
		return path
	}
	p := strings.TrimRight(prefix, "/")
	if path == p || path == p+"/" {
		return "/"
	}
	if strings.HasPrefix(path, p+"/") {
		out := strings.TrimPrefix(path, p)
		if out == "" {
			return "/"
		}
		return out
	}
	return path
}

func Director(route Route) func(*http.Request) {
	return func(req *http.Request) {
		target, err := url.Parse(route.Endpoint)
		if err != nil {
			return
		}
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
		if route.StripPrefix && route.Path != "/" {
			prefix := strings.TrimRight(route.Path, "/")
			req.Header.Set("X-Forwarded-Prefix", prefix)
			req.URL.Path = stripPath(req.URL.Path, prefix)
			if req.URL.RawPath != "" {
				req.URL.RawPath = stripPath(req.URL.RawPath, prefix)
			}
		}
	}
}

func Handler(route Route) http.Handler {
	return &httputil.ReverseProxy{Director: Director(route)}
}

func FromResources(resources []types.Resource, endpoints map[string]string) []Route {
	var out []Route
	for _, res := range resources {
		ep := endpoints[res.Name]
		for _, t := range res.Triggers {
			if t.Type != "" && t.Type != "http" {
				continue
			}
			if t.Path == "" {
				continue
			}
			out = append(out, Route{
				Path:        t.Path,
				Name:        res.Name,
				Endpoint:    ep,
				StripPrefix: t.StripPrefix,
				SPA:         t.SPA,
			})
		}
	}
	if out == nil {
		out = []Route{}
	}
	return out
}

// ApplyEdgeRules evaluates edge rules before resource routing.
// Redirect (3xx) writes the response and returns handled=true.
// Rewrite (200) mutates r.URL.Path and returns rewritten=true (not handled).
func ApplyEdgeRules(w http.ResponseWriter, r *http.Request, rules []EdgeRule) (handled bool, rewritten bool) {
	if len(rules) == 0 {
		return false, false
	}
	m, ok := netlifycompat.MatchPath(r.URL.Path, rules)
	if !ok {
		return false, false
	}
	switch {
	case m.Rule.Status >= 301 && m.Rule.Status <= 308:
		http.Redirect(w, r, m.Dest, m.Rule.Status)
		return true, false
	case m.Rule.Status == 200:
		r.URL.Path = m.Dest
		if r.URL.RawPath != "" {
			r.URL.RawPath = m.Dest
		}
		return false, true
	}
	return false, false
}

// MergeResponseHeaders sets matching header-only rule values on hdr.
func MergeResponseHeaders(hdr http.Header, path string, rules []EdgeRule) {
	for k, v := range netlifycompat.MatchHeaders(path, rules) {
		hdr.Set(k, v)
	}
}

// DraftPathPrefix is the built-in single-node draft/alias URL prefix.
const DraftPathPrefix = "/--draft/"

// ParseDraftPath extracts resource name from /--draft/<name> or /--draft/<name>/….
func ParseDraftPath(path string) (name string, ok bool) {
	if !strings.HasPrefix(path, DraftPathPrefix) {
		return "", false
	}
	rest := strings.TrimPrefix(path, DraftPathPrefix)
	if rest == "" {
		return "", false
	}
	name, _, _ = strings.Cut(rest, "/")
	if name == "" || strings.Contains(name, "..") {
		return "", false
	}
	return name, true
}

// DraftPrefix returns the strip-prefix route path for a resource (/--draft/<name>).
func DraftPrefix(name string) string {
	return DraftPathPrefix + name
}

// DraftURL builds the printable draft URL for a gateway + resource name.
func DraftURL(gateway, name string) string {
	gw := strings.TrimRight(gateway, "/")
	return gw + DraftPrefix(name) + "/"
}
