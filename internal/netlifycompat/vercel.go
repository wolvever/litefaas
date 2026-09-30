package netlifycompat

import (
	"encoding/json"
	"fmt"
	"strings"
)

type vercelFile struct {
	Redirects []vercelRedirect `json:"redirects"`
	Rewrites  []vercelRewrite  `json:"rewrites"`
	Headers   []vercelHeaders  `json:"headers"`
	Routes    []vercelRoute    `json:"routes"`
	CleanUrls *bool            `json:"cleanUrls"`
}

type vercelRedirect struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Permanent   *bool  `json:"permanent"`
	StatusCode  int    `json:"statusCode"`
}

type vercelRewrite struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

type vercelHeaders struct {
	Source  string         `json:"source"`
	Headers []vercelHeader `json:"headers"`
}

type vercelHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Legacy vercel.json routes[] subset (src/dest/status/headers).
type vercelRoute struct {
	Src     string            `json:"src"`
	Dest    string            `json:"dest"`
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
}

// ParseVercelJSON parses a vercel.json subset: redirects, rewrites, headers, routes,
// and optional cleanUrls (single-segment :page.html → /:page).
// builds/functions/crons/images/middleware/trailingSlash are ignored.
func ParseVercelJSON(src string) ([]EdgeRule, error) {
	var f vercelFile
	if err := json.Unmarshal([]byte(src), &f); err != nil {
		return nil, fmt.Errorf("vercel.json: %w", err)
	}
	var out []EdgeRule
	for i, r := range f.Redirects {
		from, err := normalizeVercelSource(r.Source)
		if err != nil {
			return nil, fmt.Errorf("vercel.json redirects[%d]: %w", i, err)
		}
		to := normalizeDestCaptures(strings.TrimSpace(r.Destination))
		if to == "" {
			return nil, fmt.Errorf("vercel.json redirects[%d]: destination is required", i)
		}
		status := 302
		if r.StatusCode != 0 {
			status = r.StatusCode
		} else if r.Permanent != nil && *r.Permanent {
			status = 301
		}
		if !validRedirectStatus(status) || status == 200 {
			return nil, fmt.Errorf("vercel.json redirects[%d]: unsupported status %d", i, status)
		}
		out = append(out, EdgeRule{
			From:   from,
			To:     to,
			Status: status,
			Source: "vercel.json",
		})
	}
	for i, r := range f.Rewrites {
		from, err := normalizeVercelSource(r.Source)
		if err != nil {
			return nil, fmt.Errorf("vercel.json rewrites[%d]: %w", i, err)
		}
		to := normalizeDestCaptures(strings.TrimSpace(r.Destination))
		if to == "" {
			return nil, fmt.Errorf("vercel.json rewrites[%d]: destination is required", i)
		}
		out = append(out, EdgeRule{
			From:   from,
			To:     to,
			Status: 200,
			Source: "vercel.json",
		})
	}
	for i, h := range f.Headers {
		from, err := normalizeVercelSource(h.Source)
		if err != nil {
			return nil, fmt.Errorf("vercel.json headers[%d]: %w", i, err)
		}
		if len(h.Headers) == 0 {
			continue
		}
		vals := make(map[string]string, len(h.Headers))
		for _, hv := range h.Headers {
			k := strings.TrimSpace(hv.Key)
			if k == "" {
				continue
			}
			vals[k] = hv.Value
		}
		if len(vals) == 0 {
			continue
		}
		out = append(out, EdgeRule{
			From:    from,
			Status:  0,
			Headers: vals,
			Source:  "vercel.json",
		})
	}
	for i, r := range f.Routes {
		src := strings.TrimSpace(r.Src)
		if src == "" {
			return nil, fmt.Errorf("vercel.json routes[%d]: src is required", i)
		}
		from, err := normalizeVercelSource(src)
		if err != nil {
			return nil, fmt.Errorf("vercel.json routes[%d]: %w", i, err)
		}
		if len(r.Headers) > 0 && strings.TrimSpace(r.Dest) == "" && r.Status == 0 {
			out = append(out, EdgeRule{
				From:    from,
				Status:  0,
				Headers: cloneStringMap(r.Headers),
				Source:  "vercel.json",
			})
			continue
		}
		to := normalizeDestCaptures(strings.TrimSpace(r.Dest))
		status := r.Status
		if status == 0 {
			if to == "" {
				return nil, fmt.Errorf("vercel.json routes[%d]: dest or headers required", i)
			}
			status = 200
		}
		if !validRedirectStatus(status) {
			return nil, fmt.Errorf("vercel.json routes[%d]: unsupported status %d", i, status)
		}
		rule := EdgeRule{
			From:   from,
			To:     to,
			Status: status,
			Source: "vercel.json",
		}
		if len(r.Headers) > 0 {
			rule.Headers = cloneStringMap(r.Headers)
		}
		out = append(out, rule)
	}
	if f.CleanUrls != nil && *f.CleanUrls {
		out = append(out, EdgeRule{
			From:   "/:page.html",
			To:     "/:page",
			Status: 301,
			Source: "vercel.json:cleanUrls",
		})
	}
	return out, nil
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// normalizeVercelSource ensures a leading slash, maps trailing :name* → /*,
// and maps a trailing /(.*) capture group to /*.
func normalizeVercelSource(source string) (string, error) {
	s := strings.TrimSpace(source)
	if s == "" {
		return "", fmt.Errorf("source is required")
	}
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	s = normalizeCaptureGroup(s)
	parts := strings.Split(s, "/")
	last := parts[len(parts)-1]
	if strings.HasPrefix(last, ":") && strings.HasSuffix(last, "*") {
		prefixParts := parts[:len(parts)-1]
		prefix := strings.Join(prefixParts, "/")
		if prefix == "" {
			return "/*", nil
		}
		return prefix + "/*", nil
	}
	return s, nil
}

// normalizeCaptureGroup maps a trailing /(.*) or /(.*)[/]? to /*.
func normalizeCaptureGroup(s string) string {
	for _, suf := range []string{"/(.*)/", "/(.*)", "(.*)"} {
		if strings.HasSuffix(s, suf) {
			prefix := strings.TrimSuffix(s, suf)
			if suf == "(.*)" && strings.HasSuffix(prefix, "/") {
				prefix = strings.TrimSuffix(prefix, "/")
			}
			if prefix == "" || prefix == "/" {
				return "/*"
			}
			if !strings.HasPrefix(prefix, "/") {
				prefix = "/" + prefix
			}
			return prefix + "/*"
		}
	}
	return s
}

// normalizeDestCaptures maps $1 → :splat for capture-group destinations.
func normalizeDestCaptures(to string) string {
	return strings.ReplaceAll(to, "$1", ":splat")
}
