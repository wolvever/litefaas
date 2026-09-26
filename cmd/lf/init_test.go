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

func TestCmdInitDockerfileBackend(t *testing.T) {
	parent := t.TempDir()
	if err := run([]string{"init", "orders", "--runtime", "dockerfile", "--dir", parent}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(parent, "orders")
	m, _, err := manifest.LoadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	if m.Runtime != "dockerfile" || m.Kind != "backend" {
		t.Fatalf("manifest = %+v", m)
	}
	if _, err := os.Stat(filepath.Join(dest, "Dockerfile")); err != nil {
		t.Fatal(err)
	}
}

func TestCmdUpRequiresManifest(t *testing.T) {
	if err := run([]string{"up", t.TempDir()}); err == nil {
		t.Fatal("expected missing stack/manifest error")
	}
}

func TestCmdTokenNone(t *testing.T) {
	if err := run([]string{"token", "--config-dir", t.TempDir()}); err != nil {
		t.Fatal(err)
	}
}

func TestCmdInitRequiresRuntime(t *testing.T) {
	if err := run([]string{"init", "hello"}); err == nil {
		t.Fatal("expected usage error")
	}
}
