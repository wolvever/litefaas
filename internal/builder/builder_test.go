package builder

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"strings"

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
	if err := materialize(dir, m, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); err != nil {
		t.Fatal(err)
	}
}

func mockDocker(t *testing.T) *[][]string {
	t.Helper()
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
	return &builds
}

func copyTree(t *testing.T, src, dest string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		out := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		return os.WriteFile(out, raw, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBuildDetectsFastAPI(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "catalog")
	copyTree(t, "../../examples/stacks/catalog", dir)
	builds := mockDocker(t)
	out, err := Build(context.Background(), dir, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Detected || out.Stack != "python-fastapi" || out.Image != "catalog:latest" {
		t.Fatalf("result = %+v", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "uvicorn") {
		t.Fatalf("expected pack Dockerfile, got %s", raw)
	}
	if len(*builds) != 1 || !reflect.DeepEqual((*builds)[0], []string{"build", "-t", "catalog:latest", dir}) {
		t.Fatalf("builds = %v", *builds)
	}
}

func TestBuildDetectsFlaskSQLAlchemy(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "notes")
	copyTree(t, "../../examples/stacks/notes", dir)
	builds := mockDocker(t)
	out, err := Build(context.Background(), dir, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Detected || out.Stack != "python-flask-sqlalchemy" || out.Image != "notes:latest" {
		t.Fatalf("result = %+v", out)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "gunicorn") {
		t.Fatalf("expected flask pack Dockerfile, got %s", raw)
	}
	if len(*builds) != 1 {
		t.Fatalf("builds = %v", *builds)
	}
}

func TestBuildDetectsExpressPrisma(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tickets")
	copyTree(t, "../../examples/stacks/tickets", dir)
	builds := mockDocker(t)
	out, err := Build(context.Background(), dir, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Detected || out.Stack != "node-express-prisma" || out.Image != "tickets:latest" {
		t.Fatalf("result = %+v", out)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "prisma generate") {
		t.Fatalf("expected node pack Dockerfile, got %s", raw)
	}
	if len(*builds) != 1 {
		t.Fatalf("builds = %v", *builds)
	}
}

func TestBuildDetectsSpringJPA(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "library")
	copyTree(t, "../../examples/stacks/library", dir)
	builds := mockDocker(t)
	out, err := Build(context.Background(), dir, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Detected || out.Stack != "java-spring-jpa" || out.Image != "library:latest" {
		t.Fatalf("result = %+v", out)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "java-spring-jpa") {
		t.Fatalf("expected jpa pack Dockerfile, got %s", raw)
	}
	if len(*builds) != 1 {
		t.Fatalf("builds = %v", *builds)
	}
}

func TestBuildDetectsNextJS(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "portal")
	copyTree(t, "../../examples/stacks/portal", dir)
	builds := mockDocker(t)
	out, err := Build(context.Background(), dir, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Detected || out.Stack != "node-nextjs" || out.Image != "portal:latest" {
		t.Fatalf("result = %+v", out)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "next") {
		t.Fatalf("expected next pack Dockerfile, got %s", raw)
	}
	if len(*builds) != 1 {
		t.Fatalf("builds = %v", *builds)
	}
}

func TestBuildDetectsDjango(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "blog")
	copyTree(t, "../../examples/stacks/blog", dir)
	_ = mockDocker(t)
	out, err := Build(context.Background(), dir, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Detected || out.Stack != "python-django" {
		t.Fatalf("result = %+v", out)
	}
}

func TestBuildDetectsNestJS(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tasks")
	copyTree(t, "../../examples/stacks/tasks", dir)
	_ = mockDocker(t)
	out, err := Build(context.Background(), dir, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Detected || out.Stack != "node-nestjs-prisma" {
		t.Fatalf("result = %+v", out)
	}
}

func TestBuildStackFlag(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module leftover\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = mockDocker(t)
	out, err := BuildWith(context.Background(), dir, "go-gin-gorm", io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if out.Detected || out.Stack != "go-gin-gorm" {
		t.Fatalf("result = %+v", out)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "go-gin-gorm") {
		t.Fatalf("Dockerfile = %s", raw)
	}
}

func TestBuildKeepsExistingDockerfile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("fastapi==0.115.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\n# user\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = mockDocker(t)
	if _, err := Build(context.Background(), dir, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "FROM scratch\n# user\n" {
		t.Fatalf("Dockerfile overwritten: %s", raw)
	}
}
