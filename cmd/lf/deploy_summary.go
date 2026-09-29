package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/wolvever/litefaas/internal/client"
	"github.com/wolvever/litefaas/internal/secret"
	"github.com/wolvever/litefaas/internal/types"
)

// formatDeploySummary writes an operator-facing block after a successful deploy.
func formatDeploySummary(w io.Writer, gateway string, res types.Resource, dep client.DeployResult) {
	fmt.Fprintln(w, "── deploy ────────────────────────────────────────")
	fmt.Fprintf(w, "name:      %s\n", res.Name)
	fmt.Fprintf(w, "kind:      %s\n", res.Kind)
	image := dep.Image
	if image == "" {
		image = res.Image
	}
	fmt.Fprintf(w, "image:     %s\n", image)
	fmt.Fprintf(w, "revision:  %d\n", dep.ID)
	status := dep.Status
	if status == "" {
		status = "ready"
	}
	fmt.Fprintf(w, "status:    %s\n", status)
	health := res.Health
	if health == "" {
		health = "/healthz"
	}
	fmt.Fprintf(w, "health:    ok  GET %s\n", health)
	if dep.Container != "" {
		fmt.Fprintf(w, "container: %s\n", dep.Container)
	}
	if dep.Endpoint != "" {
		fmt.Fprintf(w, "endpoint:  %s\n", dep.Endpoint)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "edge:")
	gw := strings.TrimRight(gateway, "/")
	wrote := false
	if res.Kind == types.KindFunction {
		fmt.Fprintf(w, "  invoke:  POST %s\n", invokeURL(gw, res.Name))
		wrote = true
	}
	for _, tr := range res.Triggers {
		typ := strings.ToLower(strings.TrimSpace(tr.Type))
		if typ != "" && typ != "http" {
			continue
		}
		path := tr.Path
		if path == "" {
			path = "/"
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		url := gw + path
		ann := ""
		var parts []string
		if tr.StripPrefix {
			parts = append(parts, "strip_prefix=true")
		}
		if tr.SPA {
			parts = append(parts, "spa=true")
		}
		if path != "/" {
			parts = append([]string{"path=" + path}, parts...)
		}
		if len(parts) > 0 {
			ann = "     (" + strings.Join(parts, " ") + ")"
		}
		fmt.Fprintf(w, "  http:    %s%s\n", url, ann)
		wrote = true
	}
	if !wrote {
		fmt.Fprintln(w, "  (none)")
	}
	refs := secret.ListRefs(res.Env)
	if len(refs) > 0 {
		fmt.Fprintln(w)
		parts := make([]string, 0, len(refs))
		seen := map[string]bool{}
		for _, r := range refs {
			line := r.FormatRef()
			if seen[line] {
				continue
			}
			seen[line] = true
			parts = append(parts, line)
		}
		fmt.Fprintf(w, "secrets:   %s\n", strings.Join(parts, "  "))
	}
	fmt.Fprintln(w, "──────────────────────────────────────────────────")
}

// edgeHTTPURLs returns gateway+path for each http trigger (same rules as the deploy summary).
func edgeHTTPURLs(gateway string, res types.Resource) []string {
	gw := strings.TrimRight(gateway, "/")
	var out []string
	for _, tr := range res.Triggers {
		typ := strings.ToLower(strings.TrimSpace(tr.Type))
		if typ != "" && typ != "http" {
			continue
		}
		path := tr.Path
		if path == "" {
			path = "/"
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		out = append(out, gw+path)
	}
	return out
}

// invokeURL is the POST invoke endpoint for a function resource.
func invokeURL(gateway, name string) string {
	gw := strings.TrimRight(gateway, "/")
	return gw + "/v1/invoke/" + name
}

// primaryEdgeURL picks the first http trigger URL, else the invoke URL for functions.
// kind is "http" or "invoke". Returns an error when neither exists.
func primaryEdgeURL(gateway string, res types.Resource) (url string, kind string, err error) {
	urls := edgeHTTPURLs(gateway, res)
	if len(urls) > 0 {
		return urls[0], "http", nil
	}
	if res.Kind == types.KindFunction {
		return invokeURL(gateway, res.Name), "invoke", nil
	}
	return "", "", fmt.Errorf("resource %q has no http trigger", res.Name)
}
