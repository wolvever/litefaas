// Package runner starts and stops workload containers via the Docker CLI.
package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/wolvever/litefaas/internal/types"
)

// Runner deploys images as named containers on the local Docker engine.
type Runner interface {
	Deploy(res types.Resource) (types.Instance, error)
	Stop(name string) error
}

// Docker uses the `docker` CLI (local image names, no registry).
type Docker struct {
	Bin string
}

func NewDocker() *Docker {
	return &Docker{Bin: "docker"}
}

func (d *Docker) bin() string {
	if d == nil || d.Bin == "" {
		return "docker"
	}
	return d.Bin
}

func (d *Docker) Deploy(res types.Resource) (types.Instance, error) {
	if res.Image == "" {
		return types.Instance{}, fmt.Errorf("image is required")
	}
	cname := res.ContainerName()
	_ = d.Stop(res.Name)

	port := strconv.Itoa(res.ContainerPort())
	args := []string{
		"run", "-d",
		"--name", cname,
		"--label", "litefaas.name=" + res.Name,
		"-e", "PORT=" + port,
		"-p", "127.0.0.1::" + port,
	}
	if res.Memory > 0 {
		args = append(args, "--memory", strconv.Itoa(res.Memory)+"m")
	}
	for k, v := range res.Env {
		args = append(args, "-e", k+"="+v)
	}
	args = append(args, res.Image)
	id, err := d.output(args...)
	if err != nil {
		return types.Instance{}, fmt.Errorf("docker run: %w", err)
	}
	hostPort, err := d.hostPort(cname, port)
	if err != nil {
		_ = d.Stop(res.Name)
		return types.Instance{}, err
	}
	inst := types.Instance{
		Name:        res.Name,
		ContainerID: strings.TrimSpace(id),
		Endpoint:    "http://127.0.0.1:" + hostPort,
		Image:       res.Image,
		Status:      "running",
	}
	health := inst.Endpoint + res.HealthPath()
	if err := waitHTTP(health, 45*time.Second); err != nil {
		logs, _ := d.output("logs", "--tail", "40", cname)
		_ = d.Stop(res.Name)
		return types.Instance{}, fmt.Errorf("container unhealthy: %w\n%s", err, logs)
	}
	return inst, nil
}

func (d *Docker) Stop(name string) error {
	cname := "litefaas-" + name
	_, _ = d.output("rm", "-f", cname)
	return nil
}

func (d *Docker) hostPort(container, ctrPort string) (string, error) {
	raw, err := d.output("inspect", container)
	if err != nil {
		return "", fmt.Errorf("docker inspect: %w", err)
	}
	var infos []struct {
		NetworkSettings struct {
			Ports map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"Ports"`
		} `json:"NetworkSettings"`
	}
	if err := json.Unmarshal([]byte(raw), &infos); err != nil || len(infos) == 0 {
		return "", fmt.Errorf("inspect ports: %w", err)
	}
	binds := infos[0].NetworkSettings.Ports[ctrPort+"/tcp"]
	if len(binds) == 0 || binds[0].HostPort == "" {
		return "", fmt.Errorf("no host port published for %s/tcp", ctrPort)
	}
	return binds[0].HostPort, nil
}

func (d *Docker) output(args ...string) (string, error) {
	cmd := exec.Command(d.bin(), args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return stdout.String(), nil
}

func waitHTTP(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
			last = fmt.Errorf("status %d", resp.StatusCode)
		} else {
			last = err
		}
		time.Sleep(200 * time.Millisecond)
	}
	if last == nil {
		last = fmt.Errorf("timeout")
	}
	return last
}
