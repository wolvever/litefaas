package initfn

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wolvever/litefaas/internal/manifest"
	"github.com/wolvever/litefaas/internal/types"
)

func TestInitGoHTTP(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "hello")
	if err := Init(dest, "hello", types.RuntimeGo, types.KindFunction, ""); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"handler.go", "Dockerfile", "go.mod", "litefaas.yaml"} {
		if _, err := os.Stat(filepath.Join(dest, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	res, err := manifest.Load(dest)
	if err != nil {
		t.Fatal(err)
	}
	if res.Runtime != types.RuntimeGo || res.Kind != types.KindFunction || res.Image != "litefaas/hello:latest" {
		t.Fatalf("manifest = %+v", res)
	}
}

func TestInitRejectsUnknownRuntime(t *testing.T) {
	if err := Init(t.TempDir(), "x", types.RuntimeJava, types.KindFunction, ""); err == nil {
		t.Fatal("expected error")
	}
}
