// Package proxy is the tiny path-based edge router (RFC-0001 §12 / §17).
package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/wolvever/litefaas/internal/types"
)

type Route struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Endpoint    string `json:"endpoint,omitempty"`
	StripPrefix bool   `json:"strip_prefix,omitempty"`
	SPA         bool   `json:"spa,omitempty"`
}

func Match(path string, routes []Route) (Route, bool) {
	var best Route
	bestLen := -1
	for _, r := range routes {
		p := r.Path
		if p == "" {
			continue
		}
		if path == p || strings.HasPrefix(path, strings.TrimRight(p, "/")+"/") || (p == "/" && strings.HasPrefix(path, "/")) {
			if len(p) > bestLen {
				best = r
				bestLen = len(p)
			}
		}
	}
	if bestLen < 0 {
		return Route{}, false
	}
	return best, true
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
			req.URL.Path = StripPrefix(req.URL.Path, route.Path)
			if req.URL.RawPath != "" {
				req.URL.RawPath = StripPrefix(req.URL.RawPath, route.Path)
			}
		}
	}
}

// StripPrefix removes a trigger path prefix so a backend can mount under /api
// and still see /healthz and / as its own roots (RFC-0001 §7).
func StripPrefix(path, triggerPath string) string {
	prefix := strings.TrimRight(triggerPath, "/")
	if prefix == "" || prefix == "/" {
		return path
	}
	if path == prefix {
		return "/"
	}
	if strings.HasPrefix(path, prefix+"/") {
		out := strings.TrimPrefix(path, prefix)
		if out == "" {
			return "/"
		}
		return out
	}
	return path
}

func Handler(route Route) http.Handler {
	return &httputil.ReverseProxy{Director: Director(route)}
}

func FromResources(resources []types.Resource, endpoints map[string]string) []Route {
	var out []Route
	for _, res := range resources {
		ep := endpoints[res.Name]
		if ep == "" {
			continue
		}
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
	return out
}
