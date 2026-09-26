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
		Kind:   types.KindBackend,
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
	for _, want := range []string{"--name litefaas-hello", "--restart unless-stopped", "--label litefaas.kind=backend", "--memory 64m", "--memory-swap 64m", "-p 127.0.0.1::8080", "-e GREETING=hi", "-e PORT=8080", "hello:latest"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("run missing %q in %v", want, run)
		}
	}
}

func TestDockerLogs(t *testing.T) {
	origOut := dockercli.Output
	origExec := dockercli.Exec
	t.Cleanup(func() {
		dockercli.Output = origOut
		dockercli.Exec = origExec
	})
	dockercli.Output = func(_ context.Context, name string, args ...string) (string, error) {
		if args[0] == "version" {
			return "27.0.0", nil
		}
		if args[0] == "port" {
			return "8080/tcp -> 127.0.0.1:32768", nil
		}
		t.Fatalf("unexpected output %v", args)
		return "", nil
	}
	var got []string
	dockercli.Exec = func(_ context.Context, stdout, _ io.Writer, name string, args ...string) error {
		got = append([]string{name}, args...)
		_, _ = io.WriteString(stdout, "line\n")
		return nil
	}
	var buf strings.Builder
	d := NewDocker()
	if err := d.Logs(context.Background(), "hello", 50, true, &buf); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "logs --tail 50 --follow litefaas-hello") {
		t.Fatalf("exec = %v", got)
	}
	if buf.String() != "line\n" {
		t.Fatalf("buf = %q", buf.String())
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
