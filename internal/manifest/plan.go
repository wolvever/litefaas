package manifest

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wolvever/litefaas/internal/stackpack"
	"github.com/wolvever/litefaas/internal/types"
	"github.com/wolvever/litefaas/templates"
	"path"
)

// Plan is a dry-run description of what lf build would use (no Docker, no writes).
type Plan struct {
	Path             string                     `json:"path"`
	Name             string                     `json:"name"`
	Kind             string                     `json:"kind"`
	Runtime          string                     `json:"runtime"`
	Stack            string                     `json:"stack,omitempty"`
	Detected         bool                       `json:"detected"`
	Priority         int                        `json:"priority,omitempty"`
	Image            string                     `json:"image"`
	Health           string                     `json:"health"`
	Memory           int                        `json:"memory"`
	DockerfileOrigin string                     `json:"dockerfile_origin"` // local|pack|runtime|none
	DockerfileSource string                     `json:"dockerfile_source,omitempty"`
	Fingerprints     []stackpack.FingerprintHit `json:"fingerprints,omitempty"`
	Hints            stackpack.Hints            `json:"hints,omitempty"`
}

// PlanBundle is one or more plans (multi-service stack.yaml).
type PlanBundle struct {
	Path     string `json:"path"`
	Services []Plan `json:"services"`
}

// PlanDetect resolves packs/manifest without docker build or Dockerfile writes.
func PlanDetect(path, stackID string) (*PlanBundle, error) {
	res, err := ResolveDetect(path, stackID)
	if err != nil {
		return nil, err
	}
	root := path
	if root == "" {
		root = "."
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	out := &PlanBundle{Path: root}
	if res.Multi != nil {
		for i := range res.Multi.Services {
			svc := &res.Multi.Services[i]
			ctxDir := ServiceDir(res.Root, svc)
			p, err := planOne(ctxDir, &Resolution{Root: ctxDir, Manifest: svc}, stackID)
			if err != nil {
				return nil, fmt.Errorf("service %s: %w", svc.Name, err)
			}
			out.Services = append(out.Services, *p)
		}
		return out, nil
	}
	p, err := planOne(res.Root, res, stackID)
	if err != nil {
		return nil, err
	}
	out.Services = []Plan{*p}
	return out, nil
}

func planOne(ctxDir string, res *Resolution, _ string) (*Plan, error) {
	if res == nil || res.Manifest == nil {
		return nil, fmt.Errorf("manifest is required")
	}
	m := res.Manifest
	p := &Plan{
		Path:     ctxDir,
		Name:     m.Name,
		Kind:     m.Kind,
		Runtime:  m.Runtime,
		Stack:    m.Stack,
		Detected: res.Detected,
		Image:    m.Image,
		Health:   m.Health,
		Memory:   m.Memory,
	}
	pack := res.Pack
	if pack == nil && m.Stack != "" {
		if cat, err := stackpack.Open(); err == nil {
			pack, _ = cat.Get(m.Stack)
		}
	}
	if pack != nil {
		p.Priority = pack.Priority
		p.Hints = pack.Hints
		if ok, hits := pack.ExplainMatch(ctxDir); ok {
			p.Fingerprints = hits
		}
	}
	p.DockerfileOrigin, p.DockerfileSource = dockerfileOrigin(ctxDir, pack, m.Runtime)
	return p, nil
}

func dockerfileOrigin(dir string, pack *stackpack.Pack, runtime string) (origin, source string) {
	df := filepath.Join(dir, "Dockerfile")
	if st, err := os.Stat(df); err == nil && !st.IsDir() {
		return "local", "./Dockerfile"
	}
	if pack != nil && len(pack.Dockerfile) > 0 {
		return "pack", pack.ID
	}
	rt, err := types.ParseRuntime(runtime)
	if err == nil && runtimeHasDockerfile(rt) {
		return "runtime", string(rt)
	}
	return "none", ""
}

func runtimeHasDockerfile(rt types.Runtime) bool {
	var fsys fs.FS
	var root string
	switch rt {
	case types.RuntimeGo:
		fsys, root = templates.Runtimes, "runtimes/go/http"
	case types.RuntimeJava:
		fsys, root = templates.Runtimes, "runtimes/java/http"
	case types.RuntimePython:
		fsys, root = templates.Runtimes, "runtimes/python/http"
	case types.RuntimeNode:
		fsys, root = templates.Runtimes, "runtimes/node/http"
	case types.RuntimeStatic:
		fsys, root = templates.Frontend, "frontend/static"
	case types.RuntimeDockerfile:
		fsys, root = templates.Meta, "meta/dockerfile"
	default:
		return false
	}
	_, err := fs.ReadFile(fsys, path.Join(root, "Dockerfile"))
	return err == nil
}

// FormatPlan writes human-readable plan text.
func FormatPlan(w io.Writer, b *PlanBundle) error {
	if b == nil || len(b.Services) == 0 {
		return fmt.Errorf("empty plan")
	}
	if len(b.Services) > 1 {
		fmt.Fprintf(w, "stack.yaml: %d services\n\n", len(b.Services))
	}
	for i, p := range b.Services {
		if i > 0 {
			fmt.Fprintln(w)
		}
		if err := formatOnePlan(w, p); err != nil {
			return err
		}
	}
	return nil
}

func formatOnePlan(w io.Writer, p Plan) error {
	stackLabel := p.Stack
	if stackLabel == "" {
		stackLabel = "-"
	}
	det := ""
	if p.Detected && p.Stack != "" {
		det = fmt.Sprintf("          (detected, priority %d)", p.Priority)
	} else if p.Stack != "" && !p.Detected {
		det = "          (manifest/forced)"
	} else {
		det = "          (manifest runtime/preset)"
	}
	fmt.Fprintf(w, "path:        %s\n", displayPath(p.Path))
	fmt.Fprintf(w, "name:        %s\n", p.Name)
	fmt.Fprintf(w, "kind:        %s\n", p.Kind)
	fmt.Fprintf(w, "runtime:     %s\n", p.Runtime)
	fmt.Fprintf(w, "stack:       %s%s\n", stackLabel, det)
	fmt.Fprintf(w, "image:       %s\n", p.Image)
	fmt.Fprintf(w, "health:      %s\n", p.Health)
	fmt.Fprintf(w, "memory:      %d\n", p.Memory)
	fmt.Fprintln(w)
	if len(p.Fingerprints) > 0 {
		fmt.Fprintln(w, "fingerprints:")
		for i, hit := range p.Fingerprints {
			fmt.Fprintf(w, "  match[%d]:\n", i)
			if len(hit.Files) > 0 {
				fmt.Fprintf(w, "    files:     %s\n", strings.Join(hit.Files, ", "))
			}
			for _, c := range hit.Contains {
				fmt.Fprintf(w, "    contains:  %s ∋ %s\n", c.File, c.Matched)
			}
		}
		fmt.Fprintln(w)
	}
	switch p.DockerfileOrigin {
	case "local":
		fmt.Fprintf(w, "dockerfile:  local:%s (unchanged)\n", p.DockerfileSource)
	case "pack":
		fmt.Fprintf(w, "dockerfile:  pack:%s     (would write Dockerfile if missing)\n", p.DockerfileSource)
	case "runtime":
		fmt.Fprintf(w, "dockerfile:  runtime:%s scaffold if missing\n", p.DockerfileSource)
	default:
		fmt.Fprintln(w, "dockerfile:  none")
	}
	fmt.Fprintln(w)
	if len(p.Hints.Sidecars) == 0 && len(p.Hints.Env) == 0 {
		fmt.Fprintln(w, "hints:       -")
		return nil
	}
	fmt.Fprintln(w, "hints:       (not started by litefaas)")
	if len(p.Hints.Sidecars) > 0 {
		fmt.Fprintf(w, "  sidecars:  %s\n", strings.Join(p.Hints.Sidecars, ", "))
	}
	if len(p.Hints.Env) > 0 {
		keys := make([]string, 0, len(p.Hints.Env))
		for k := range p.Hints.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(w, "  env:       %s=%s\n", k, p.Hints.Env[k])
		}
	}
	return nil
}

func displayPath(p string) string {
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, p); err == nil && !strings.HasPrefix(rel, "..") {
			if rel == "." {
				return "."
			}
			return "./" + filepath.ToSlash(rel)
		}
	}
	return p
}

// WritePlanJSON encodes the plan bundle.
func WritePlanJSON(w io.Writer, b *PlanBundle) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(b)
}
