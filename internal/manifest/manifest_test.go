package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wolvever/litefaas/internal/types"
)

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte("name: hello\nruntime: go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, root, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if root != dir {
		t.Fatalf("root = %q", root)
	}
	if m.Kind != string(types.KindFunction) {
		t.Fatalf("kind = %q", m.Kind)
	}
	if m.Image != "hello:latest" || m.Port != 8080 || m.Memory != 128 || m.Timeout != "30s" || m.Health != "/healthz" {
		t.Fatalf("defaults = %+v", m)
	}
	res := m.Resource()
	if res.Name != "hello" || res.Runtime != types.RuntimeGo {
		t.Fatalf("resource = %+v", res)
	}
}

func TestLoadRejectsBadTimeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte("name: hello\nruntime: go\ntimeout: nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestLoadBackendReplicasAndStripPrefix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	raw := "name: orders\nkind: backend\nruntime: dockerfile\ntriggers:\n  - type: http\n    path: /api\n    strip_prefix: true\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != string(types.KindBackend) || m.Replicas != 1 {
		t.Fatalf("backend defaults = %+v", m)
	}
	if len(m.Triggers) != 1 || m.Triggers[0].Path != "/api" || !m.Triggers[0].StripPrefix {
		t.Fatalf("triggers = %+v", m.Triggers)
	}
	res := m.Resource()
	if res.Replicas != 1 || !res.Triggers[0].StripPrefix {
		t.Fatalf("resource = %+v", res)
	}
}

func TestLoadExampleAPIStripPrefix(t *testing.T) {
	m, err := Load(filepath.Join("..", "..", "examples", "api", "litefaas.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != string(types.KindBackend) || m.Replicas != 1 {
		t.Fatalf("example api = %+v", m)
	}
	if len(m.Triggers) != 1 || !m.Triggers[0].StripPrefix || m.Triggers[0].Path != "/api" {
		t.Fatalf("example api triggers = %+v", m.Triggers)
	}
}

func TestParseTimeout(t *testing.T) {
	d, err := ParseTimeout("45s")
	if err != nil || d.Seconds() != 45 {
		t.Fatalf("got %v %v", d, err)
	}
	if _, err := ParseTimeout("0s"); err == nil {
		t.Fatal("expected error for 0s")
	}
}
