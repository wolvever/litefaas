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
	Path        string
	Name        string
	Endpoint    string
	StripPrefix bool
	SPA         bool
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
			prefix := strings.TrimRight(route.Path, "/")
			if req.URL.Path == prefix {
				req.URL.Path = "/"
			} else if strings.HasPrefix(req.URL.Path, prefix+"/") {
				req.URL.Path = strings.TrimPrefix(req.URL.Path, prefix)
				if req.URL.Path == "" {
					req.URL.Path = "/"
				}
			}
		}
	}
}

func Handler(route Route) http.Handler {
	return &httputil.ReverseProxy{Director: Director(route)}
}

func FromResources(resources []types.Resource, instances map[string]types.Instance) []Route {
	var out []Route
	for _, res := range resources {
		inst, ok := instances[res.Name]
		if !ok || inst.Endpoint == "" {
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
				Endpoint:    inst.Endpoint,
				StripPrefix: t.StripPrefix,
				SPA:         t.SPA,
			})
		}
	}
	return out
}
