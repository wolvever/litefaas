package netlifycompat

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

type netlifyFile struct {
	Redirects []tomlRedirect `toml:"redirects"`
	Headers   []tomlHeader   `toml:"headers"`
}

type tomlRedirect struct {
	From   string `toml:"from"`
	To     string `toml:"to"`
	Status int    `toml:"status"`
	Force  bool   `toml:"force"`
}

type tomlHeader struct {
	For    string            `toml:"for"`
	Values map[string]string `toml:"values"`
}

// ParseNetlifyTOML parses a thin netlify.toml: [[redirects]] and [[headers]] only.
// build/plugins/functions/edge tables are ignored.
func ParseNetlifyTOML(src string) ([]EdgeRule, error) {
	var f netlifyFile
	meta, err := toml.Decode(src, &f)
	if err != nil {
		return nil, fmt.Errorf("netlify.toml: %w", err)
	}
	_ = meta // undecoded keys (build, plugins, …) intentionally ignored
	var out []EdgeRule
	for i, r := range f.Redirects {
		from := strings.TrimSpace(r.From)
		to := strings.TrimSpace(r.To)
		if from == "" || to == "" {
			return nil, fmt.Errorf("netlify.toml redirects[%d]: from and to are required", i)
		}
		if !strings.HasPrefix(from, "/") {
			return nil, fmt.Errorf("netlify.toml redirects[%d]: from must start with /", i)
		}
		status := r.Status
		if status == 0 {
			status = 301
		}
		if !validRedirectStatus(status) {
			return nil, fmt.Errorf("netlify.toml redirects[%d]: unsupported status %d", i, status)
		}
		out = append(out, EdgeRule{
			From:   from,
			To:     to,
			Status: status,
			Force:  r.Force, // ignored at serve time
			Source: "netlify.toml",
		})
	}
	for i, h := range f.Headers {
		forPath := strings.TrimSpace(h.For)
		if forPath == "" {
			return nil, fmt.Errorf("netlify.toml headers[%d]: for is required", i)
		}
		if !strings.HasPrefix(forPath, "/") {
			return nil, fmt.Errorf("netlify.toml headers[%d]: for must start with /", i)
		}
		if len(h.Values) == 0 {
			continue
		}
		vals := make(map[string]string, len(h.Values))
		for k, v := range h.Values {
			vals[k] = v
		}
		out = append(out, EdgeRule{
			From:    forPath,
			Status:  0,
			Headers: vals,
			Source:  "netlify.toml",
		})
	}
	return out, nil
}
