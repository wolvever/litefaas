// Package runner starts and replaces function containers on a single Docker host.
package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wolvever/litefaas/internal/dockercli"
	"github.com/wolvever/litefaas/internal/types"
)

const (
	containerPrefix = "litefaas-"
	defaultMemory   = 128
	healthWait      = 30 * time.Second
	healthEvery     = 250 * time.Millisecond
)

var ErrNotDeployed = errors.New("not deployed")

// Result is a running container reachable from the gateway.
type Result struct {
	Container string `json:"container"`
	Endpoint  string `json:"endpoint"`
}

// LogsOptions selects docker logs flags.
type LogsOptions struct {
	Follow bool
	Tail   int
}

// RemoveOpts controls optional behavior when removing a deployed resource.
type RemoveOpts struct {
	PruneVolumes bool
	Volumes      []types.VolumeMount
}

// Runner deploys images as containers and reports their local endpoint.
type Runner interface {
	Deploy(ctx context.Context, res types.Resource) (Result, error)
	Remove(ctx context.Context, name string, opts ...RemoveOpts) error
	Endpoint(ctx context.Context, name string) (string, error)
	Logs(ctx context.Context, name string, opts LogsOptions, w io.Writer) error
}

// Docker is a single-node runner that shells out to the docker CLI.
type Docker struct {
	HTTP *http.Client
}

func NewDocker() *Docker {
	return &Docker{HTTP: &http.Client{Timeout: 2 * time.Second}}
}

func ContainerName(name string) string { return containerPrefix + name }

func candidateName(name string) string { return containerPrefix + name + "-new" }

func (d *Docker) Deploy(ctx context.Context, res types.Resource) (Result, error) {
	if err := dockercli.Available(ctx); err != nil {
		return Result{}, err
	}
	if res.Image == "" {
		return Result{}, fmt.Errorf("image is required")
	}
	port := res.Port
	if port == 0 {
		port = 8080
	}
	mem := res.Memory
	if mem <= 0 {
		mem = defaultMemory
	}
	stable := ContainerName(res.Name)
	cand := candidateName(res.Name)
	_ = d.rmContainer(ctx, cand)
	if err := d.ensureVolumes(ctx, res); err != nil {
		return Result{}, err
	}
	restart := "unless-stopped"
	if !types.AlwaysOn(res.Kind) {
		restart = "no"
	}
	memFlag := fmt.Sprintf("%dm", mem)
	args := []string{
		"run", "-d",
		"--name", cand,
		"--restart", restart,
		"--label", "litefaas.managed=1",
		"--label", "litefaas.name=" + res.Name,
		"--label", "litefaas.kind=" + string(res.Kind),
		"--add-host", "host.docker.internal:host-gateway",
		"--memory", memFlag,
		"--memory-swap", memFlag,
		"-p", fmt.Sprintf("127.0.0.1::%d", port),
	}
	for _, v := range res.Volumes {
		vol := types.DockerVolumeName(res.Name, v.Name)
		spec := vol + ":" + v.Mount
		if v.ReadOnly {
			spec += ":ro"
		}
		args = append(args, "-v", spec)
	}
	for k, v := range res.Env {
		args = append(args, "-e", k+"="+v)
	}
	args = append(args, "-e", fmt.Sprintf("PORT=%d", port), res.Image)
	if _, err := dockercli.Output(ctx, "docker", args...); err != nil {
		return Result{}, fmt.Errorf("docker run: %w", err)
	}
	ep, err := d.endpointOf(ctx, cand)
	if err != nil {
		_ = d.rmContainer(ctx, cand)
		return Result{}, err
	}
	if err := d.waitHealthy(ctx, ep, res.Health); err != nil {
		_ = d.rmContainer(ctx, cand)
		return Result{}, err
	}
	_ = d.rmContainer(ctx, stable)
	if _, err := dockercli.Output(ctx, "docker", "rename", cand, stable); err != nil {
		return Result{}, fmt.Errorf("docker rename %s -> %s: %w (candidate left as %s)", cand, stable, err, cand)
	}
	ep, err = d.endpointOf(ctx, stable)
	if err != nil {
		return Result{}, err
	}
	return Result{Container: stable, Endpoint: ep}, nil
}

func (d *Docker) ensureVolumes(ctx context.Context, res types.Resource) error {
	for _, v := range res.Volumes {
		if err := types.ValidateVolumeMount(v); err != nil {
			return err
		}
		name := types.DockerVolumeName(res.Name, v.Name)
		if _, err := dockercli.Output(ctx, "docker", "volume", "create", name); err != nil {
			msg := strings.ToLower(err.Error())
			if !strings.Contains(msg, "already") {
				return fmt.Errorf("docker volume create %s: %w", name, err)
			}
		}
	}
	return nil
}

func (d *Docker) rmContainer(ctx context.Context, cname string) error {
	_, err := dockercli.Output(ctx, "docker", "rm", "-f", cname)
	if err == nil || isMissingContainer(err) {
		return nil
	}
	return err
}

func (d *Docker) Remove(ctx context.Context, name string, opts ...RemoveOpts) error {
	// Preserve named volumes across delete unless PruneVolumes is set.
	_ = d.rmContainer(ctx, candidateName(name))
	if err := d.rmContainer(ctx, ContainerName(name)); err != nil {
		return err
	}
	var o RemoveOpts
	if len(opts) > 0 {
		o = opts[0]
	}
	if !o.PruneVolumes {
		return nil
	}
	for _, v := range o.Volumes {
		vol := types.DockerVolumeName(name, v.Name)
		if _, err := dockercli.Output(ctx, "docker", "volume", "rm", vol); err != nil {
			if isMissingVolume(err) {
				continue
			}
			return fmt.Errorf("docker volume rm %s: %w", vol, err)
		}
	}
	return nil
}

func isMissingVolume(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such volume") || strings.Contains(msg, "not found")
}

func (d *Docker) Logs(ctx context.Context, name string, opts LogsOptions, w io.Writer) error {
	if err := dockercli.Available(ctx); err != nil {
		return err
	}
	if w == nil {
		w = io.Discard
	}
	args := []string{"logs", "--timestamps"}
	if opts.Follow {
		args = append(args, "--follow")
	}
	tail := opts.Tail
	if tail <= 0 {
		tail = 100
	}
	args = append(args, "--tail", strconv.Itoa(tail), ContainerName(name))
	if err := dockercli.Exec(ctx, w, w, "docker", args...); err != nil {
		if isMissingContainer(err) {
			return ErrNotDeployed
		}
		return fmt.Errorf("docker logs: %w", err)
	}
	return nil
}

func (d *Docker) Endpoint(ctx context.Context, name string) (string, error) {
	return d.endpointOf(ctx, ContainerName(name))
}

func (d *Docker) endpointOf(ctx context.Context, cname string) (string, error) {
	out, err := dockercli.Output(ctx, "docker", "port", cname)
	if err != nil {
		if isMissingContainer(err) {
			return "", ErrNotDeployed
		}
		return "", err
	}
	hostPort, err := parseDockerPort(out)
	if err != nil {
		return "", err
	}
	return "http://127.0.0.1:" + hostPort, nil
}

func (d *Docker) waitHealthy(ctx context.Context, endpoint, health string) error {
	if health == "" {
		health = "/healthz"
	}
	if !strings.HasPrefix(health, "/") {
		health = "/" + health
	}
	url := strings.TrimRight(endpoint, "/") + health
	client := d.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	deadline := time.Now().Add(healthWait)
	var last error
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			last = fmt.Errorf("health %s: status %d", url, resp.StatusCode)
		} else {
			last = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(healthEvery):
		}
	}
	if last == nil {
		last = fmt.Errorf("health check timed out")
	}
	return fmt.Errorf("container not healthy: %w", last)
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
	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("docker port: parse %q", out)
	}
	return port, nil
}

func isMissingContainer(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such container") || strings.Contains(msg, "not found")
}
