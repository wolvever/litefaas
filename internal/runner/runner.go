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
	healthWait      = 15 * time.Second
	healthEvery     = 250 * time.Millisecond
)

var (
	ErrNotDeployed = errors.New("not deployed")
)

// Result is a running container reachable from the gateway.
type Result struct {
	Container string `json:"container"`
	Endpoint  string `json:"endpoint"`
}

// Runner deploys images as containers and reports their local endpoint.
type Runner interface {
	Deploy(ctx context.Context, res types.Resource) (Result, error)
	Remove(ctx context.Context, name string) error
	Endpoint(ctx context.Context, name string) (string, error)
}

// Docker is a single-node runner that shells out to the docker CLI.
type Docker struct {
	HTTP *http.Client
}

func NewDocker() *Docker {
	return &Docker{HTTP: &http.Client{Timeout: 2 * time.Second}}
}

func ContainerName(name string) string {
	return containerPrefix + name
}

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
	cname := ContainerName(res.Name)
	_ = d.Remove(ctx, res.Name)

	// Backends and frontends are always-on (RFC-0001 §5.2): no scale-to-zero.
	// Functions use the same restart policy until an idle TTL exists (Phase 6).
	args := []string{
		"run", "-d",
		"--name", cname,
		"--restart", "unless-stopped",
		"--label", "litefaas.managed=1",
		"--label", "litefaas.name=" + res.Name,
		"--label", "litefaas.kind=" + string(res.Kind),
		"--memory", fmt.Sprintf("%dm", mem),
		"-p", fmt.Sprintf("127.0.0.1::%d", port),
	}
	for k, v := range res.Env {
		args = append(args, "-e", k+"="+v)
	}
	args = append(args, "-e", fmt.Sprintf("PORT=%d", port), res.Image)

	if _, err := dockercli.Output(ctx, "docker", args...); err != nil {
		return Result{}, fmt.Errorf("docker run: %w", err)
	}
	ep, err := d.Endpoint(ctx, res.Name)
	if err != nil {
		_ = d.Remove(ctx, res.Name)
		return Result{}, err
	}
	if err := d.waitHealthy(ctx, ep, res.Health); err != nil {
		_ = d.Remove(ctx, res.Name)
		return Result{}, err
	}
	return Result{Container: cname, Endpoint: ep}, nil
}

func (d *Docker) Remove(ctx context.Context, name string) error {
	_, err := dockercli.Output(ctx, "docker", "rm", "-f", ContainerName(name))
	if err == nil || isMissingContainer(err) {
		return nil
	}
	return err
}

func (d *Docker) Endpoint(ctx context.Context, name string) (string, error) {
	out, err := dockercli.Output(ctx, "docker", "port", ContainerName(name))
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
	// "8080/tcp -> 127.0.0.1:32768" or "127.0.0.1:32768"
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
	_, port, ok := strings.Cut(line, ":")
	if !ok {
		port = line
	}
	// last colon for [ipv6]:port
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
