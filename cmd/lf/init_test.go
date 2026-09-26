package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wolvever/litefaas/internal/manifest"
)

func TestCmdInitGo(t *testing.T) {
	parent := t.TempDir()
	if err := run([]string{"init", "hello", "--runtime", "go", "--dir", parent}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(parent, "hello")
	m, _, err := manifest.LoadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "hello" || m.Runtime != "go" || m.Kind != "function" {
		t.Fatalf("manifest = %+v", m)
	}
	if _, err := os.Stat(filepath.Join(dest, "handler.go")); err != nil {
		t.Fatal(err)
	}
}

func TestCmdInitJavaPython(t *testing.T) {
	parent := t.TempDir()
	if err := run([]string{"init", "hello-java", "--runtime", "java", "--dir", parent}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(parent, "hello-java")
	m, _, err := manifest.LoadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	if m.Runtime != "java" {
		t.Fatalf("manifest = %+v", m)
	}
	if _, err := os.Stat(filepath.Join(dest, "Handler.java")); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"init", "hello-py", "--runtime", "python", "--preset", "fastapi", "--dir", parent}); err != nil {
		t.Fatal(err)
	}
	m, _, err = manifest.LoadDir(filepath.Join(parent, "hello-py"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Runtime != "python" || m.Preset != "fastapi" {
		t.Fatalf("fastapi manifest = %+v", m)
	}
}

func TestCmdInitStaticFrontend(t *testing.T) {
	parent := t.TempDir()
	if err := run([]string{"init", "web", "--runtime", "static", "--dir", parent}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(parent, "web")
	m, _, err := manifest.LoadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	if m.Runtime != "static" || m.Kind != "frontend" || m.Health != "/" {
		t.Fatalf("manifest = %+v", m)
	}
	if len(m.Triggers) == 0 || m.Triggers[0].Path != "/" || !m.Triggers[0].SPA {
		t.Fatalf("triggers = %+v", m.Triggers)
	}
	if _, err := os.Stat(filepath.Join(dest, "index.html")); err != nil {
		t.Fatal(err)
	}
}

func TestCmdInitDockerfileAndKindBackend(t *testing.T) {
	parent := t.TempDir()
	if err := run([]string{"init", "echo", "--runtime", "dockerfile", "--dir", parent}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(parent, "echo")
	m, _, err := manifest.LoadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	if m.Runtime != "dockerfile" || m.Kind != "backend" || m.Replicas != 1 {
		t.Fatalf("dockerfile manifest = %+v", m)
	}
	if len(m.Triggers) == 0 || m.Triggers[0].Path != "/echo" || !m.Triggers[0].StripPrefix {
		t.Fatalf("triggers = %+v", m.Triggers)
	}
	if _, err := os.Stat(filepath.Join(dest, "Dockerfile")); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"init", "orders", "--runtime", "python", "--kind", "backend", "--dir", parent}); err != nil {
		t.Fatal(err)
	}
	m, _, err = manifest.LoadDir(filepath.Join(parent, "orders"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != "backend" || m.Runtime != "python" || m.Replicas != 1 {
		t.Fatalf("kind backend manifest = %+v", m)
	}
	if len(m.Triggers) == 0 || !m.Triggers[0].StripPrefix || m.Triggers[0].Path != "/orders" {
		t.Fatalf("backend triggers = %+v", m.Triggers)
	}
}

func TestCmdInitRequiresRuntime(t *testing.T) {
	if err := run([]string{"init", "hello"}); err == nil {
		t.Fatal("expected usage error")
	}
}
