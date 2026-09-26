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

func TestMaterializeDockerfileRequiresUserFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "litefaas.yaml"), []byte("name: echo\nkind: backend\nruntime: dockerfile\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, _, err := manifest.LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := materialize(dir, m); err == nil {
		t.Fatal("expected missing Dockerfile error")
	}

	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := materialize(dir, m); err != nil {
		t.Fatal(err)
	}
}
