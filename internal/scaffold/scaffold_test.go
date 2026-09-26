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
	dest := t.TempDir()
	out, err := Init(Options{Name: "hello", Runtime: "go", Kind: "function", Dest: dest})
	if err != nil {
		t.Fatal(err)
	}
	if out != dest {
		t.Fatalf("dest = %q", out)
	}
	for _, name := range []string{"handler.go", "go.mod", "Dockerfile", types.ManifestFile, "template.yml", ".dockerignore"} {
		if _, err := os.Stat(filepath.Join(dest, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	m, _, err := manifest.Load(dest)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "hello" || m.Runtime != "go" || m.Image != "hello:latest" || m.Kind != "function" {
		t.Fatalf("manifest = %+v", m)
	}
	raw, err := os.ReadFile(filepath.Join(dest, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "module hello") {
		t.Fatalf("go.mod = %q", raw)
	}
}

func TestInitGoTemplateCompiles(t *testing.T) {
	dest := t.TempDir()
	if _, err := Init(Options{Name: "echo", Runtime: "go", Dest: dest}); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", filepath.Join(dest, "function"), ".")
	cmd.Dir = dest
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build template: %v\n%s", err, out)
	}
}

func TestInitRejectsLaterRuntimes(t *testing.T) {
	_, err := Init(Options{Name: "x", Runtime: "java", Dest: t.TempDir()})
	if err == nil {
		t.Fatal("expected java to be rejected")
	}
	_, err = Init(Options{Name: "x", Runtime: "go", Preset: "gin", Dest: t.TempDir()})
	if err == nil {
		t.Fatal("expected preset to be rejected")
	}
}

func TestInitRejectsNonEmpty(t *testing.T) {
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "keep"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(Options{Name: "hello", Runtime: "go", Dest: dest}); err == nil {
		t.Fatal("expected non-empty dest error")
	}
}
