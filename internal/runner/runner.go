// Package runner starts and stops function containers with Docker (RFC-0001 §5.1).
package runner

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/wolvever/litefaas/internal/dockerx"
	"github.com/wolvever/litefaas/internal/types"
)

const labelManaged = "litefaas.managed=1"

// Instance is a running workload the daemon can invoke.
type Instance struct {
	Name     string
	Image    string
	Endpoint string
}

// Runner deploys local Docker images on a single node.
type Runner interface {
	Deploy(ctx context.Context, res types.Resource, image string) (Instance, error)
	Stop(ctx context.Context, name string) error
	Lookup(ctx context.Context, name string) (Instance, error)
}

// Docker talks to the host docker CLI (same daemon as lf build).
type Docker struct{}

func ContainerName(name string) string {
	return "litefaas-" + name
}

func (Docker) Deploy(ctx context.Context, res types.Resource, image string) (Instance, error) {
	if image == "" {
		image = res.Image
	}
	if image == "" {
		return Instance{}, fmt.Errorf("image is required")
	}
	if err := dockerx.Available(ctx); err != nil {
		return Instance{}, err
	}
	_ = removeContainer(ctx, ContainerName(res.Name))

	port := res.Port
	if port == 0 {
		port = 8080
	}
	args := []string{
		"run", "-d",
		"--name", ContainerName(res.Name),
		"--label", labelManaged,
		"--label", "litefaas.name=" + res.Name,
		"--label", fmt.Sprintf("litefaas.port=%d", port),
		"-e", fmt.Sprintf("PORT=%d", port),
		"-p", fmt.Sprintf("127.0.0.1::%d", port),
		"--restart", "unless-stopped",
	}
	for k, v := range res.Env {
		if !envKeyOK(k) {
			return Instance{}, fmt.Errorf("invalid env name %q", k)
		}
		args = append(args, "-e", k+"="+v)
	}
	if res.Memory > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", res.Memory))
	}
	args = append(args, image)

	if _, err := dockerx.Output(ctx, args...); err != nil {
		return Instance{}, fmt.Errorf("docker run: %w", err)
	}

	inst, err := inspect(ctx, res.Name, port)
	if err != nil {
		_ = removeContainer(ctx, ContainerName(res.Name))
		return Instance{}, err
	}
	if err := waitHealthy(ctx, inst.Endpoint, res.Health); err != nil {
		_ = removeContainer(ctx, ContainerName(res.Name))
		return Instance{}, err
	}
	return inst, nil
}

func (Docker) Stop(ctx context.Context, name string) error {
	return removeContainer(ctx, ContainerName(name))
}

func (Docker) Lookup(ctx context.Context, name string) (Instance, error) {
	return inspect(ctx, name, 0)
}

func inspect(ctx context.Context, name string, fallbackPort int) (Instance, error) {
	cname := ContainerName(name)
	state, err := dockerx.Output(ctx, "inspect", "-f",
		`{{.State.Running}} {{.Config.Image}} {{index .Config.Labels "litefaas.port"}}`,
		cname,
	)
	if err != nil {
		return Instance{}, fmt.Errorf("function %q is not deployed", name)
	}
	fields := strings.Fields(state)
	if len(fields) < 2 {
		return Instance{}, fmt.Errorf("function %q: unexpected docker inspect output %q", name, state)
	}
	if fields[0] != "true" {
		return Instance{}, fmt.Errorf("function %q is not running", name)
	}
	image := fields[1]
	port := fallbackPort
	if len(fields) >= 3 {
		if _, err := fmt.Sscanf(fields[2], "%d", &port); err != nil {
			port = fallbackPort
		}
	}
	if port == 0 {
		port = 8080
	}
	published, err := dockerx.Output(ctx, "port", cname, fmt.Sprintf("%d/tcp", port))
	if err != nil {
		return Instance{}, fmt.Errorf("function %q: no published port: %w", name, err)
	}
	hostPort, err := parsePublished(published)
	if err != nil {
		return Instance{}, err
	}
	return Instance{
		Name:     name,
		Image:    image,
		Endpoint: "http://127.0.0.1:" + hostPort,
	}, nil
}

func parsePublished(s string) (string, error) {
	line := strings.TrimSpace(strings.Split(s, "\n")[0])
	_, port, err := net.SplitHostPort(line)
	if err != nil {
		return "", fmt.Errorf("parse docker port %q: %w", s, err)
	}
	return port, nil
}

func removeContainer(ctx context.Context, cname string) error {
	_, err := dockerx.Output(ctx, "rm", "-f", cname)
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "no such container") {
		return nil
	}
	return err
}

func waitHealthy(ctx context.Context, endpoint, health string) error {
	if health == "" {
		health = "/healthz"
	}
	if !strings.HasPrefix(health, "/") {
		health = "/" + health
	}
	u := strings.TrimRight(endpoint, "/") + health
	deadline := time.Now().Add(30 * time.Second)
	var last error
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			last = fmt.Errorf("GET %s: status %d", u, resp.StatusCode)
		} else {
			last = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
	return fmt.Errorf("container did not become healthy at %s: %v", u, last)
}

func envKeyOK(k string) bool {
	if k == "" || k == "PORT" {
		return false
	}
	for i, c := range k {
		ok := c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (i > 0 && c >= '0' && c <= '9')
		if !ok {
			return false
		}
	}
	return true
}
