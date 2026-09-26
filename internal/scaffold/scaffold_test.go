package scaffold

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wolvever/litefaas/internal/manifest"
	"github.com/wolvever/litefaas/internal/types"
)

func TestInitGoHTTP(t *testing.T) {
	parent := t.TempDir()
	dest, err := Init(Options{Name: "hello", Runtime: types.RuntimeGo, Dir: parent})
	if err != nil {
		t.Fatal(err)
	}
	if dest != filepath.Join(parent, "hello") {
		t.Fatalf("dest = %q", dest)
	}
	for _, name := range []string{"litefaas.yaml", "Dockerfile", "handler.go", "go.mod"} {
		if _, err := os.Stat(filepath.Join(dest, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dest, "template.yml")); !os.IsNotExist(err) {
		t.Fatal("template.yml should not be copied")
	}
	m, _, err := manifest.LoadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "hello" || m.Kind != "function" || m.Runtime != "go" || m.Image != "hello:latest" {
		t.Fatalf("manifest = %+v", m)
	}
	mod, err := os.ReadFile(filepath.Join(dest, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mod), "module hello") {
		t.Fatalf("go.mod = %s", mod)
	}
	src, err := os.ReadFile(filepath.Join(dest, "handler.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src), "{{name}}") {
		t.Fatal("handler still has placeholders")
	}

	out := filepath.Join(t.TempDir(), "fn")
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = dest
	cmd.Env = append(os.Environ(), "GOFLAGS=")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated handler did not compile: %v\n%s", err, b)
	}
}

func TestInitRejectsOtherRuntime(t *testing.T) {
	_, err := Init(Options{Name: "x", Runtime: types.RuntimeJava, Dir: t.TempDir()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInitRefuseNonEmpty(t *testing.T) {
	parent := t.TempDir()
	dest := filepath.Join(parent, "hello")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "keep"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(Options{Name: "hello", Runtime: types.RuntimeGo, Dir: parent}); err == nil {
		t.Fatal("expected not-empty error")
	}
	if _, err := Init(Options{Name: "hello", Runtime: types.RuntimeGo, Dir: parent, Force: true}); err != nil {
		t.Fatal(err)
	}
}
