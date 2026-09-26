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
