package runner

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wolvever/litefaas/internal/dockercli"
	"github.com/wolvever/litefaas/internal/types"
)

func TestParseDockerPort(t *testing.T) {
	cases := map[string]string{
		"8080/tcp -> 127.0.0.1:32768": "32768",
		"127.0.0.1:32000":             "32000",
		"0.0.0.0:9":                   "9",
	}
	for in, want := range cases {
		got, err := parseDockerPort(in)
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if got != want {
			t.Fatalf("%q => %q want %q", in, got, want)
		}
	}
}

func TestDockerDeployCommands(t *testing.T) {
	health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(health.Close)
	hostPort := strings.TrimPrefix(health.URL, "http://127.0.0.1:")

	origOut := dockercli.Output
	t.Cleanup(func() { dockercli.Output = origOut })

	var calls [][]string
	dockercli.Output = func(_ context.Context, name string, args ...string) (string, error) {
		if name != "docker" {
			t.Fatalf("name = %s", name)
		}
		calls = append(calls, append([]string{}, args...))
		switch args[0] {
		case "version":
			return "27.0.0", nil
		case "rm":
			return "", nil
		case "run":
			return "abc123", nil
		case "port":
			return "8080/tcp -> 127.0.0.1:" + hostPort, nil
		default:
			t.Fatalf("unexpected %v", args)
		}
		return "", nil
	}

	d := NewDocker()
	res, err := d.Deploy(context.Background(), types.Resource{
		Name:   "hello",
		Image:  "hello:latest",
		Port:   8080,
		Memory: 64,
		Health: "/healthz",
		Env:    map[string]string{"GREETING": "hi"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Container != "litefaas-hello" || res.Endpoint != health.URL {
		t.Fatalf("result = %+v", res)
	}

	var run []string
	for _, c := range calls {
		if len(c) > 0 && c[0] == "run" {
			run = c
		}
	}
	if run == nil {
		t.Fatal("docker run not called")
	}
	joined := strings.Join(run, " ")
	for _, want := range []string{"--name litefaas-hello", "--restart no", "--memory 64m", "-p 127.0.0.1::8080", "-e GREETING=hi", "-e PORT=8080", "hello:latest"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("run missing %q in %v", want, run)
		}
	}
}

func TestDockerDeployBackendAlwaysOn(t *testing.T) {
	health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(health.Close)
	hostPort := strings.TrimPrefix(health.URL, "http://127.0.0.1:")

	origOut := dockercli.Output
	t.Cleanup(func() { dockercli.Output = origOut })

	var run []string
	dockercli.Output = func(_ context.Context, name string, args ...string) (string, error) {
		if name != "docker" {
			t.Fatalf("name = %s", name)
		}
		if len(args) > 0 && args[0] == "run" {
			run = append([]string{}, args...)
		}
		switch args[0] {
		case "version", "rm", "run":
			return "ok", nil
		case "port":
			return "8080/tcp -> 127.0.0.1:" + hostPort, nil
		default:
			t.Fatalf("unexpected %v", args)
		}
		return "", nil
	}

	d := NewDocker()
	_, err := d.Deploy(context.Background(), types.Resource{
		Name:     "orders",
		Kind:     types.KindBackend,
		Runtime:  types.RuntimeDockerfile,
		Image:    "orders:latest",
		Replicas: 1,
		Health:   "/healthz",
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(run, " ")
	for _, want := range []string{"--restart unless-stopped", "--label litefaas.kind=backend", "--label litefaas.always-on=1"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("backend run missing %q in %v", want, run)
		}
	}
}

func TestEndpointMissing(t *testing.T) {
	orig := dockercli.Output
	t.Cleanup(func() { dockercli.Output = orig })
	dockercli.Output = func(context.Context, string, ...string) (string, error) {
		return "", fmtError("Error: No such container: litefaas-missing")
	}
	d := NewDocker()
	_, err := d.Endpoint(context.Background(), "missing")
	if err != ErrNotDeployed {
		t.Fatalf("err = %v", err)
	}
}

func fmtError(s string) error { return &strErr{s} }

type strErr struct{ s string }

func (e *strErr) Error() string { return e.s }
