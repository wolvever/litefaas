package runner

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestDockerDeployHealthGatedCutover(t *testing.T) {
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
		calls = append(calls, append([]string{}, args...))
		switch args[0] {
		case "version":
			return "27.0.0", nil
		case "rm", "rename", "volume":
			return "", nil
		case "run":
			return "abc123", nil
		case "port":
			return "8080/tcp -> 127.0.0.1:" + hostPort, nil
		case "image":
			return "sha256:abc", nil
		default:
			t.Fatalf("unexpected %v", args)
		}
		return "", nil
	}

	d := NewDocker()
	res, err := d.Deploy(context.Background(), types.Resource{
		Name: "hello", Kind: types.KindBackend, Image: "hello:latest",
		Port: 8080, Memory: 64, Health: "/healthz",
		Env:     map[string]string{"GREETING": "hi"},
		Volumes: []types.VolumeMount{{Name: "data", Mount: "/app/data"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Container != "litefaas-hello" || res.Endpoint != health.URL || res.ImageID != "sha256:abc" {
		t.Fatalf("result = %+v", res)
	}
	var run []string
	var renamed bool
	all := ""
	for _, c := range calls {
		all += strings.Join(c, " ") + "\n"
		if c[0] == "run" {
			run = c
		}
		if c[0] == "rename" {
			renamed = true
		}
	}
	if !renamed {
		t.Fatal("expected rename")
	}
	if !strings.Contains(all, "rm -f litefaas-hello") {
		t.Fatalf("expected rm of stable name after healthy candidate: %s", all)
	}
	joined := strings.Join(run, " ")
	for _, want := range []string{"--name litefaas-hello-new", "--add-host host.docker.internal:host-gateway", "-v litefaas-hello-data:/app/data", "-e GREETING=hi"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %v", want, run)
		}
	}
	if !strings.Contains(all, "volume create litefaas-hello-data") {
		t.Fatalf("no volume create: %s", all)
	}
}

func TestDockerDeployRemovesCandidateOnHealthFail(t *testing.T) {
	health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(health.Close)
	hostPort := strings.TrimPrefix(health.URL, "http://127.0.0.1:")
	origOut := dockercli.Output
	t.Cleanup(func() { dockercli.Output = origOut })
	var rms []string
	var renamed bool
	dockercli.Output = func(_ context.Context, _ string, args ...string) (string, error) {
		switch args[0] {
		case "version":
			return "27.0.0", nil
		case "rm":
			rms = append(rms, args[len(args)-1])
			return "", nil
		case "run":
			return "cand", nil
		case "port":
			return "8080/tcp -> 127.0.0.1:" + hostPort, nil
		case "rename":
			renamed = true
			return "", nil
		default:
			t.Fatalf("unexpected %v", args)
		}
		return "", nil
	}
	d := NewDocker()
	d.HTTP = &http.Client{Timeout: 200 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	if _, err := d.Deploy(ctx, types.Resource{Name: "hello", Kind: types.KindBackend, Image: "x", Health: "/healthz"}); err == nil {
		t.Fatal("expected failure")
	}
	if renamed {
		t.Fatal("must not rename")
	}
	found := false
	for _, n := range rms {
		if n == "litefaas-hello-new" {
			found = true
		}
	}
	if !found {
		t.Fatalf("candidate not removed: %v", rms)
	}
	for _, n := range rms {
		if n == "litefaas-hello" {
			t.Fatalf("must not remove previous revision: %v", rms)
		}
	}
}

func TestDockerFunctionRestartAndLogs(t *testing.T) {
	health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(health.Close)
	hostPort := strings.TrimPrefix(health.URL, "http://127.0.0.1:")
	origOut, origExec := dockercli.Output, dockercli.Exec
	t.Cleanup(func() { dockercli.Output = origOut; dockercli.Exec = origExec })
	var run, logArgs []string
	dockercli.Output = func(_ context.Context, _ string, args ...string) (string, error) {
		switch args[0] {
		case "version", "rm", "rename":
			return "", nil
		case "run":
			run = append([]string{}, args...)
			return "id", nil
		case "port":
			return "8080/tcp -> 127.0.0.1:" + hostPort, nil
		case "image":
			return "sha256:fn", nil
		default:
			t.Fatalf("unexpected %v", args)
		}
		return "", nil
	}
	dockercli.Exec = func(_ context.Context, _, _ io.Writer, _ string, args ...string) error {
		logArgs = append([]string{}, args...)
		return nil
	}
	d := NewDocker()
	if _, err := d.Deploy(context.Background(), types.Resource{Name: "hello", Kind: types.KindFunction, Image: "hello:latest"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(run, " "), "--restart no") {
		t.Fatalf("run = %v", run)
	}
	var buf strings.Builder
	if err := d.Logs(context.Background(), "hello", LogsOptions{Follow: true, Tail: 20}, &buf); err != nil {
		t.Fatal(err)
	}
	want := "logs --timestamps --follow --tail 20 litefaas-hello"
	if strings.Join(logArgs, " ") != want {
		t.Fatalf("logs = %v", logArgs)
	}
}

func TestEndpointMissing(t *testing.T) {
	orig := dockercli.Output
	t.Cleanup(func() { dockercli.Output = orig })
	dockercli.Output = func(context.Context, string, ...string) (string, error) {
		return "", &strErr{"Error: No such container: litefaas-missing"}
	}
	if _, err := NewDocker().Endpoint(context.Background(), "missing"); err != ErrNotDeployed {
		t.Fatalf("err=%v", err)
	}
}

func TestRemoveDoesNotDeleteVolumes(t *testing.T) {
	orig := dockercli.Output
	t.Cleanup(func() { dockercli.Output = orig })
	dockercli.Output = func(_ context.Context, _ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "volume" {
			t.Fatalf("Remove must not touch volumes: %v", args)
		}
		return "", nil
	}
	if err := NewDocker().Remove(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
}

type strErr struct{ s string }

func (e *strErr) Error() string { return e.s }

func TestRemovePrunesVolumes(t *testing.T) {
	orig := dockercli.Output
	t.Cleanup(func() { dockercli.Output = orig })
	var saw [][]string
	dockercli.Output = func(_ context.Context, _ string, args ...string) (string, error) {
		saw = append(saw, append([]string{}, args...))
		return "", nil
	}
	opts := RemoveOpts{
		PruneVolumes: true,
		Volumes:      []types.VolumeMount{{Name: "data", Mount: "/app/data"}},
	}
	if err := NewDocker().Remove(context.Background(), "hello", opts); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, args := range saw {
		if len(args) >= 3 && args[0] == "volume" && args[1] == "rm" && args[2] == "litefaas-hello-data" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected volume rm litefaas-hello-data; saw %v", saw)
	}
}

func TestRemovePruneIgnoresMissingVolume(t *testing.T) {
	orig := dockercli.Output
	t.Cleanup(func() { dockercli.Output = orig })
	dockercli.Output = func(_ context.Context, _ string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "volume" && args[1] == "rm" {
			return "", &strErr{"Error: No such volume: litefaas-hello-data"}
		}
		return "", nil
	}
	opts := RemoveOpts{
		PruneVolumes: true,
		Volumes:      []types.VolumeMount{{Name: "data", Mount: "/data"}},
	}
	if err := NewDocker().Remove(context.Background(), "hello", opts); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseFailureDoesNotCutOver(t *testing.T) {
	orig := dockercli.Output
	t.Cleanup(func() { dockercli.Output = orig })
	var renamed bool
	var startedCandidate bool
	dockercli.Output = func(_ context.Context, _ string, args ...string) (string, error) {
		switch args[0] {
		case "version", "rm":
			return "", nil
		case "run":
			if containsArg(args, "--rm") {
				return "", &strErr{"exit status 1"}
			}
			startedCandidate = true
			return "id", nil
		case "rename":
			renamed = true
			return "", nil
		default:
			t.Fatalf("unexpected %v", args)
		}
		return "", nil
	}
	_, err := NewDocker().Deploy(context.Background(), types.Resource{
		Name: "api", Kind: types.KindBackend, Image: "api:2",
		Release: []string{"python manage.py migrate --noinput"},
	})
	if err == nil || !strings.Contains(err.Error(), "live version unchanged") {
		t.Fatalf("err=%v", err)
	}
	if renamed || startedCandidate {
		t.Fatalf("cutover happened renamed=%v candidate=%v", renamed, startedCandidate)
	}
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestDeployDraftDoesNotTouchProdName(t *testing.T) {
	health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(health.Close)
	hostPort := strings.TrimPrefix(health.URL, "http://127.0.0.1:")
	orig := dockercli.Output
	t.Cleanup(func() { dockercli.Output = orig })
	var cmds []string
	dockercli.Output = func(_ context.Context, _ string, args ...string) (string, error) {
		cmds = append(cmds, strings.Join(args, " "))
		switch args[0] {
		case "version", "rm":
			return "", nil
		case "run":
			return "id", nil
		case "port":
			return "8080/tcp -> 127.0.0.1:" + hostPort, nil
		case "rename":
			return "", nil
		case "image":
			return "sha256:draft", nil
		default:
			t.Fatalf("unexpected %v", args)
		}
		return "", nil
	}
	res, err := NewDocker().DeployDraft(context.Background(), types.Resource{
		Name: "hello", Kind: types.KindBackend, Image: "hello:draft", Release: []string{"echo no"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Container != "litefaas-hello-draft" {
		t.Fatalf("container=%s", res.Container)
	}
	all := strings.Join(cmds, "\n")
	if !strings.Contains(all, "rename litefaas-hello-draft-new litefaas-hello-draft") {
		t.Fatalf("rename missing: %s", all)
	}
	if strings.Contains(all, "litefaas-hello-new") || strings.Contains(all, "rename litefaas-hello-draft-new litefaas-hello\n") {
		t.Fatalf("prod name touched: %s", all)
	}
	if strings.Contains(all, "--rm") {
		t.Fatalf("release ran: %s", all)
	}
}
