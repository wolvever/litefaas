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

func TestCmdInitRequiresRuntime(t *testing.T) {
	if err := run([]string{"init", "hello"}); err == nil {
		t.Fatal("expected usage error")
	}
}
