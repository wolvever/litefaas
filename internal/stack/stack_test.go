package stack

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDir(t *testing.T) {
	dir := t.TempDir()
	raw := "name: demo\nservices:\n  - path: web\n  - path: api\n"
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	s, root, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if root != dir || s.Name != "demo" || len(s.Services) != 2 {
		t.Fatalf("stack = %+v root=%q", s, root)
	}
	dirs := s.ServiceDirs(root)
	if dirs[0] != filepath.Join(dir, "web") || dirs[1] != filepath.Join(dir, "api") {
		t.Fatalf("dirs = %v", dirs)
	}
}

func TestLoadRejectsEmpty(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, FileName)
	if err := os.WriteFile(p, []byte("name: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("expected error")
	}
}
