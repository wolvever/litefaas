package release

import (
	"context"
	"strings"
	"testing"

	"github.com/wolvever/litefaas/internal/dockercli"
	"github.com/wolvever/litefaas/internal/types"
)

func TestArgsNoPublishedPort(t *testing.T) {
	args := Args(types.Resource{
		Name: "api", Image: "api:cand", Port: 8080, Memory: 64,
		Env:     map[string]string{"B": "2", "A": "1"},
		Volumes: []types.VolumeMount{{Name: "data", Mount: "/data"}},
	}, "python manage.py migrate --noinput")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--rm") {
		t.Fatalf("missing --rm: %v", args)
	}
	for i, a := range args {
		if a == "-p" {
			t.Fatalf("published port at %d: %v", i, args)
		}
	}
	for _, want := range []string{
		"--name litefaas-api-release",
		"-v litefaas-api-data:/data",
		"-e A=1",
		"-e B=2",
		"-e PORT=8080",
		"api:cand",
		"sh",
		"-c",
		"python manage.py migrate --noinput",
	} {
		if !strings.Contains(joined, want) && !containsArg(args, want) {
			t.Fatalf("missing %q in %v", want, args)
		}
	}
	// env keys are sorted so A is before B
	ia := strings.Index(joined, "-e A=1")
	ib := strings.Index(joined, "-e B=2")
	if ia < 0 || ib < ia {
		t.Fatalf("env order: %s", joined)
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

func TestRunFailureDoesNotContinue(t *testing.T) {
	orig := dockercli.Output
	t.Cleanup(func() { dockercli.Output = orig })
	var runs [][]string
	dockercli.Output = func(_ context.Context, _ string, args ...string) (string, error) {
		runs = append(runs, append([]string{}, args...))
		if len(args) > 0 && args[0] == "run" {
			return "", &exitErr{"exit status 1"}
		}
		return "", nil
	}
	err := Run(context.Background(), types.Resource{
		Name: "api", Image: "api:1",
		Release: []string{"echo one", "echo two"},
	})
	if err == nil || !strings.Contains(err.Error(), "live version unchanged") {
		t.Fatalf("err=%v", err)
	}
	nRun := 0
	for _, a := range runs {
		if a[0] == "run" {
			nRun++
		}
	}
	if nRun != 1 {
		t.Fatalf("runs=%d %v", nRun, runs)
	}
}

func TestCommandsPrecedence(t *testing.T) {
	if got := Commands([]string{"from-manifest"}, []string{"from-pack"}); len(got) != 1 || got[0] != "from-manifest" {
		t.Fatalf("manifest win: %v", got)
	}
	if got := Commands(nil, []string{"from-pack"}); len(got) != 1 || got[0] != "from-pack" {
		t.Fatalf("pack fallback: %v", got)
	}
	if got := Commands(nil, nil); got != nil {
		t.Fatalf("empty: %v", got)
	}
}

type exitErr struct{ s string }

func (e *exitErr) Error() string { return e.s }
