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

// ParseVercelJSON parses a vercel.json subset: redirects, rewrites, headers.
// builds/routes/cleanUrls/functions/crons/images are ignored.
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
		to := strings.TrimSpace(r.Destination)
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
		to := strings.TrimSpace(r.Destination)
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
	return out, nil
}

// normalizeVercelSource ensures a leading slash and maps trailing :name* → /* splat.
func normalizeVercelSource(source string) (string, error) {
	s := strings.TrimSpace(source)
	if s == "" {
		return "", fmt.Errorf("source is required")
	}
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
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
