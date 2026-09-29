// Package check implements lf check: host preflight → image build → one-shot smoke.
package check

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/wolvever/litefaas/internal/builder"
	"github.com/wolvever/litefaas/internal/dockercli"
	"github.com/wolvever/litefaas/internal/manifest"
	"github.com/wolvever/litefaas/internal/runner"
	"github.com/wolvever/litefaas/internal/stackpack"
)

const outRel = ".litefaas/out"

// HostExec runs a host-side command (cwd + env). Tests swap this.
var HostExec = func(ctx context.Context, dir string, env []string, stdout, stderr io.Writer, name string, args ...string) error {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}


// Options configures a check run.
type Options struct {
	Dir       string
	StackID   string
	SkipHost  bool
	SkipSmoke bool
	JSON      bool
	Stdout    io.Writer
	Stderr    io.Writer
	HTTP      *http.Client
}

// StageResult is one stage outcome (host / image / smoke) for JSON output.
type StageResult struct {
	Name    string `json:"name"`
	Service string `json:"service,omitempty"`
	OK      bool   `json:"ok"`
	Skipped bool   `json:"skipped,omitempty"`
	Detail  string `json:"detail,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Result aggregates stage outcomes.
type Result struct {
	Stages []StageResult `json:"stages"`
	OK     bool          `json:"ok"`
}

type unit struct {
	Name     string
	Dir      string
	Manifest *manifest.Manifest
	Pack     *stackpack.Pack
	StackID  string
}

// Run executes host → image → smoke for each service sequentially.
// First failure aborts. Stage names go to stderr; success one-liner to stdout
// (unless JSON, which writes the Result to stdout).
func Run(ctx context.Context, opts Options) (*Result, error) {
	if opts.Dir == "" {
		opts.Dir = "."
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	res, err := manifest.ResolveDetect(opts.Dir, opts.StackID)
	if err != nil {
		return nil, err
	}
	units, err := unitsFrom(res, opts.StackID)
	if err != nil {
		return nil, err
	}
	out := &Result{OK: true}
	for _, u := range units {
		if err := checkUnit(ctx, opts, out, u); err != nil {
			out.OK = false
			if opts.JSON {
				_ = writeJSON(opts.Stdout, out)
			}
			return out, err
		}
	}
	if opts.JSON {
		if err := writeJSON(opts.Stdout, out); err != nil {
			return out, err
		}
	} else {
		fmt.Fprintln(opts.Stdout, "check passed")
	}
	return out, nil
}

func writeJSON(w io.Writer, r *Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func unitsFrom(res *manifest.Resolution, stackID string) ([]unit, error) {
	if res.Multi != nil {
		var out []unit
		for i := range res.Multi.Services {
			svc := &res.Multi.Services[i]
			dir := manifest.ServiceDir(res.Root, svc)
			pack, err := lookupPack(dir, svc, "")
			if err != nil {
				return nil, fmt.Errorf("service %s: %w", svc.Name, err)
			}
			out = append(out, unit{Name: svc.Name, Dir: dir, Manifest: svc, Pack: pack})
		}
		return out, nil
	}
	name := ""
	if res.Manifest != nil {
		name = res.Manifest.Name
	}
	return []unit{{
		Name:     name,
		Dir:      res.Root,
		Manifest: res.Manifest,
		Pack:     res.Pack,
		StackID:  stackID,
	}}, nil
}

func lookupPack(dir string, m *manifest.Manifest, stackID string) (*stackpack.Pack, error) {
	id := strings.TrimSpace(stackID)
	if id == "" && m != nil {
		id = strings.TrimSpace(m.Stack)
	}
	cat, err := stackpack.Open()
	if err != nil {
		return nil, err
	}
	if id != "" {
		return cat.Get(id)
	}
	if m != nil && strings.TrimSpace(m.Runtime) != "" {
		// Explicit runtime without pack — no host recipe.
		return nil, nil
	}
	p, err := cat.Detect(dir)
	if err != nil {
		// No pack match is fine for check; host simply skipped.
		if strings.Contains(err.Error(), "no matching") || strings.Contains(err.Error(), "no stack") {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

func checkUnit(ctx context.Context, opts Options, out *Result, u unit) error {
	svcLabel := u.Name
	pack := u.Pack
	if pack == nil {
		var err error
		pack, err = lookupPack(u.Dir, u.Manifest, u.StackID)
		if err != nil {
			return err
		}
		u.Pack = pack
	}

	// --- host ---
	hostStage := StageResult{Name: "host", Service: svcLabel}
	buildCmd := packHostBuild(pack)
	if opts.SkipHost || len(buildCmd) == 0 {
		hostStage.OK = true
		hostStage.Skipped = true
		if opts.SkipHost {
			hostStage.Detail = "skipped (--skip-host)"
		} else {
			hostStage.Detail = "skipped (no host.build)"
		}
		fmt.Fprintf(opts.Stderr, "host: %s\n", hostStage.Detail)
		out.Stages = append(out.Stages, hostStage)
	} else {
		fmt.Fprintf(opts.Stderr, "host: %s\n", strings.Join(buildCmd, " "))
		if err := runHost(ctx, opts, u.Dir, buildCmd); err != nil {
			hostStage.OK = false
			hostStage.Error = err.Error()
			out.Stages = append(out.Stages, hostStage)
			return fmt.Errorf("host: %w", err)
		}
		hostStage.OK = true
		hostStage.Detail = strings.Join(buildCmd, " ")
		out.Stages = append(out.Stages, hostStage)
	}

	// --- image ---
	imgStage := StageResult{Name: "image", Service: svcLabel}
	fmt.Fprintf(opts.Stderr, "building %s\n", u.Dir)
	buildOut := opts.Stdout
	if opts.JSON {
		buildOut = io.Discard
	}
	built, err := builder.BuildResolved(ctx, &manifest.Resolution{
		Root:     u.Dir,
		Manifest: u.Manifest,
		Pack:     pack,
	}, buildOut, opts.Stderr)
	if err != nil {
		imgStage.OK = false
		imgStage.Error = err.Error()
		out.Stages = append(out.Stages, imgStage)
		return fmt.Errorf("image: %w", err)
	}
	imgStage.OK = true
	imgStage.Detail = built.Image
	out.Stages = append(out.Stages, imgStage)

	// --- smoke ---
	smokeStage := StageResult{Name: "smoke", Service: svcLabel}
	if opts.SkipSmoke {
		smokeStage.OK = true
		smokeStage.Skipped = true
		smokeStage.Detail = "skipped (--skip-smoke)"
		fmt.Fprintf(opts.Stderr, "smoke: %s\n", smokeStage.Detail)
		out.Stages = append(out.Stages, smokeStage)
		return nil
	}
	health := resolveHealth(u.Manifest, pack)
	name := u.Name
	if name == "" {
		name = "app"
	}
	fmt.Fprintf(opts.Stderr, "smoke: image %s health=%s\n", built.Image, health)
	if err := Smoke(ctx, SmokeOpts{
		Image:   built.Image,
		Name:    name,
		Health:  health,
		Port:    manifestPort(u.Manifest),
		HTTP:    opts.HTTP,
		Stderr:  opts.Stderr,
	}); err != nil {
		smokeStage.OK = false
		smokeStage.Error = err.Error()
		out.Stages = append(out.Stages, smokeStage)
		return fmt.Errorf("smoke: %w", err)
	}
	smokeStage.OK = true
	smokeStage.Detail = fmt.Sprintf("%s health=%s ok", built.Image, health)
	fmt.Fprintf(opts.Stderr, "smoke: %s\n", smokeStage.Detail)
	out.Stages = append(out.Stages, smokeStage)
	return nil
}

func packHostBuild(p *stackpack.Pack) []string {
	if p == nil || len(p.Host.Build) == 0 {
		return nil
	}
	out := make([]string, len(p.Host.Build))
	copy(out, p.Host.Build)
	return out
}

func resolveHealth(m *manifest.Manifest, p *stackpack.Pack) string {
	if p != nil {
		if h := strings.TrimSpace(p.Verify.Health); h != "" {
			if !strings.HasPrefix(h, "/") {
				h = "/" + h
			}
			return h
		}
	}
	if m != nil {
		if h := strings.TrimSpace(m.Health); h != "" {
			if !strings.HasPrefix(h, "/") {
				h = "/" + h
			}
			return h
		}
	}
	return "/healthz"
}

func manifestPort(m *manifest.Manifest) int {
	if m != nil && m.Port > 0 {
		return m.Port
	}
	return 8080
}

func runHost(ctx context.Context, opts Options, dir string, argv []string) error {
	if len(argv) == 0 {
		return nil
	}
	outDir := filepath.Join(dir, outRel)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}
	env := append(os.Environ(), "LITEFAAS_OUT="+absOut)
	hostOut := opts.Stdout
	if opts.JSON {
		hostOut = io.Discard
	}
	return HostExec(ctx, dir, env, hostOut, opts.Stderr, argv[0], argv[1:]...)
}

// SmokeOpts configures a one-shot container smoke.
type SmokeOpts struct {
	Image  string
	Name   string
	Health string
	Port   int
	HTTP   *http.Client
	Stderr io.Writer
}

// Smoke runs the image once, waits for health, then removes the container.
func Smoke(ctx context.Context, opts SmokeOpts) error {
	if opts.Image == "" {
		return fmt.Errorf("image is required")
	}
	if opts.Port <= 0 {
		opts.Port = 8080
	}
	if opts.Health == "" {
		opts.Health = "/healthz"
	}
	if err := dockercli.Available(ctx); err != nil {
		return err
	}
	cname := fmt.Sprintf("litefaas-check-%s-%d", sanitizeName(opts.Name), os.Getpid())
	_ = rmContainer(ctx, cname)
	defer func() { _ = rmContainer(ctx, cname) }()

	args := []string{
		"run", "-d",
		"--name", cname,
		"--label", "litefaas.check=1",
		"--add-host", "host.docker.internal:host-gateway",
		"-p", fmt.Sprintf("127.0.0.1::%d", opts.Port),
		"-e", fmt.Sprintf("PORT=%d", opts.Port),
		opts.Image,
	}
	if _, err := dockercli.Output(ctx, "docker", args...); err != nil {
		return fmt.Errorf("docker run: %w", err)
	}
	ep, err := endpointOf(ctx, cname)
	if err != nil {
		return err
	}
	client := opts.HTTP
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second}
	}
	return runner.WaitHTTP(ctx, client, ep, opts.Health)
}

func sanitizeName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "app"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := b.String()
	if out == "" {
		return "app"
	}
	return out
}

func rmContainer(ctx context.Context, cname string) error {
	_, err := dockercli.Output(ctx, "docker", "rm", "-f", cname)
	return err
}

func endpointOf(ctx context.Context, cname string) (string, error) {
	out, err := dockercli.Output(ctx, "docker", "port", cname)
	if err != nil {
		return "", err
	}
	hostPort, err := parseDockerPort(out)
	if err != nil {
		return "", err
	}
	return "http://127.0.0.1:" + hostPort, nil
}

func parseDockerPort(out string) (string, error) {
	line := strings.TrimSpace(out)
	if line == "" {
		return "", fmt.Errorf("docker port: empty output")
	}
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	if i := strings.LastIndex(line, "->"); i >= 0 {
		line = strings.TrimSpace(line[i+2:])
	}
	port := line
	if i := strings.LastIndex(line, ":"); i >= 0 {
		port = line[i+1:]
	}
	port = strings.TrimSpace(port)
	if port == "" {
		return "", fmt.Errorf("docker port: parse %q", out)
	}
	return port, nil
}
