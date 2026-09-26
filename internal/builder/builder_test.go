package builder

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/wolvever/litefaas/internal/dockercli"
	"github.com/wolvever/litefaas/internal/manifest"
	"github.com/wolvever/litefaas/internal/scaffold"
	"github.com/wolvever/litefaas/internal/types"
)

func TestBuildTagsImage(t *testing.T) {
	parent := t.TempDir()
	dest, err := scaffold.Init(scaffold.Options{Name: "hello", Runtime: types.RuntimeGo, Dir: parent})
	if err != nil {
		t.Fatal(err)
	}

	origExec, origOut := dockercli.Exec, dockercli.Output
	t.Cleanup(func() {
		dockercli.Exec = origExec
		dockercli.Output = origOut
	})
	var buildArgs []string
	dockercli.Output = func(context.Context, string, ...string) (string, error) { return "27.0.0", nil }
	dockercli.Exec = func(_ context.Context, _, _ io.Writer, name string, args ...string) error {
		if name != "docker" {
			t.Fatalf("name = %s", name)
		}
		buildArgs = args
		return nil
	}

	res, err := Build(context.Background(), dest, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if res.Image != "hello:latest" {
		t.Fatalf("image = %q", res.Image)
	}
	want := []string{"build", "-t", "hello:latest", dest}
	if !reflect.DeepEqual(buildArgs, want) {
		t.Fatalf("args = %v want %v", buildArgs, want)
	}
}

func TestBuildStack(t *testing.T) {
	dir := t.TempDir()
	web := filepath.Join(dir, "web")
	if err := os.MkdirAll(web, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw := "services:\n  web:\n    kind: frontend\n    runtime: static\n    handler: ./web\n    image: web:latest\n"
	if err := os.WriteFile(filepath.Join(dir, "stack.yaml"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	origExec, origOut := dockercli.Exec, dockercli.Output
	t.Cleanup(func() {
		dockercli.Exec = origExec
		dockercli.Output = origOut
	})
	var builds [][]string
	dockercli.Output = func(context.Context, string, ...string) (string, error) { return "27.0.0", nil }
	dockercli.Exec = func(_ context.Context, _, _ io.Writer, name string, args ...string) error {
		builds = append(builds, append([]string{}, args...))
		return nil
	}

	out, err := BuildStack(context.Background(), dir, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Image != "web:latest" || out[0].Context != web {
		t.Fatalf("stack build = %+v", out)
	}
	if len(builds) != 1 || !reflect.DeepEqual(builds[0], []string{"build", "-t", "web:latest", web}) {
		t.Fatalf("builds = %v", builds)
	}
}

func TestMaterializeWritesDockerfile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "litefaas.yaml"), []byte("name: hello\nruntime: go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, _, err := manifest.LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := materialize(dir, m); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); err != nil {
		t.Fatal(err)
	}
}
