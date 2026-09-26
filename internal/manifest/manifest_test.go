package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wolvever/litefaas/internal/types"
)

func TestLoadGoFunction(t *testing.T) {
	dir := t.TempDir()
	raw := []byte(`name: hello
kind: function
runtime: go
handler: .
image: hello:latest
port: 8080
timeout: 60s
health: /healthz
triggers:
  - type: http
    path: /fn/hello
`)
	if err := os.WriteFile(filepath.Join(dir, types.ManifestFile), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	m, gotDir, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if gotDir != dir {
		t.Fatalf("dir = %q", gotDir)
	}
	if m.Name != "hello" || m.Runtime != "go" || m.Image != "hello:latest" {
		t.Fatalf("manifest = %+v", m)
	}
	res := m.Resource()
	if res.Kind != types.KindFunction || res.Port != 8080 {
		t.Fatalf("resource = %+v", res)
	}
}

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, types.ManifestFile), []byte("name: echo\nruntime: go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != "function" || m.Image != "echo:latest" || m.Port != 8080 || m.Health != "/healthz" || m.Timeout != "60s" {
		t.Fatalf("defaults = %+v", m)
	}
}

func TestLoadRejectsBadName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, types.ManifestFile), []byte("name: 'bad name'\nruntime: go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(dir); err == nil {
		t.Fatal("expected error")
	}
}
