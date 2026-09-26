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

func TestInitJavaPythonAndPresets(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		name   string
		rt     types.Runtime
		preset string
		file   string
	}{
		{"java-http", types.RuntimeJava, "", "Handler.java"},
		{"py-http", types.RuntimePython, "", "handler.py"},
		{"spring", types.RuntimeJava, "spring-boot", "src/main/java/hello/Application.java"},
		{"fast", types.RuntimePython, "fastapi", "main.py"},
	}
	for _, tc := range cases {
		dest := filepath.Join(root, tc.name)
		if err := Init(dest, tc.name, tc.rt, types.KindFunction, tc.preset); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if _, err := os.Stat(filepath.Join(dest, tc.file)); err != nil {
			t.Fatalf("%s missing %s: %v", tc.name, tc.file, err)
		}
		res, err := manifest.Load(dest)
		if err != nil {
			t.Fatal(err)
		}
		if res.Runtime != tc.rt || res.Preset != tc.preset {
			t.Fatalf("%s manifest = %+v", tc.name, res)
		}
	}
}

func TestInitRejectsUnknownRuntime(t *testing.T) {
	if err := Init(t.TempDir(), "x", types.RuntimeDockerfile, types.KindFunction, ""); err == nil {
		t.Fatal("expected error")
	}
}
