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

func TestParseTimeout(t *testing.T) {
	d, err := ParseTimeout("45s")
	if err != nil || d.Seconds() != 45 {
		t.Fatalf("got %v %v", d, err)
	}
	if _, err := ParseTimeout("0s"); err == nil {
		t.Fatal("expected error for 0s")
	}
}
