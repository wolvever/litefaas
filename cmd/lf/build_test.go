package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wolvever/litefaas/internal/dockercli"
)

func TestCmdBuildDetectsAndOverrides(t *testing.T) {
	origExec, origOut := dockercli.Exec, dockercli.Output
	t.Cleanup(func() {
		dockercli.Exec = origExec
		dockercli.Output = origOut
	})
	dockercli.Output = func(context.Context, string, ...string) (string, error) { return "27.0.0", nil }
	dockercli.Exec = func(context.Context, io.Writer, io.Writer, string, ...string) error { return nil }

	dir := filepath.Join(t.TempDir(), "catalog")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("fastapi==0.115.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"build", dir}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "uvicorn") {
		t.Fatalf("detected Dockerfile = %s", raw)
	}

	other := filepath.Join(t.TempDir(), "forced")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "requirements.txt"), []byte("fastapi==0.115.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"build", other, "--stack", "go-gin-gorm"}); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(other, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "go-gin-gorm") {
		t.Fatalf("override Dockerfile = %s", raw)
	}
}

func TestCmdBuildUnknownStack(t *testing.T) {
	dir := t.TempDir()
	if err := run([]string{"build", dir, "--stack", "quarkus"}); err == nil {
		t.Fatal("expected unknown stack")
	}
}
