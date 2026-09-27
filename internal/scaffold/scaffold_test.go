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

func TestInitJavaPythonAndPresets(t *testing.T) {
	parent := t.TempDir()
	cases := []struct {
		name   string
		rt     types.Runtime
		preset string
		file   string
	}{
		{"hello-java", types.RuntimeJava, "", "Handler.java"},
		{"hello-py", types.RuntimePython, "", "handler.py"},
		{"orders", types.RuntimeJava, "spring-boot", "src/main/java/hello/Application.java"},
		{"api", types.RuntimePython, "fastapi", "main.py"},
		{"web", types.RuntimeStatic, "", "index.html"},
		{"orders-df", types.RuntimeDockerfile, "", "handler.py"},
	}
	for _, tc := range cases {
		dest, err := Init(Options{Name: tc.name, Runtime: tc.rt, Preset: tc.preset, Dir: parent})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if _, err := os.Stat(filepath.Join(dest, tc.file)); err != nil {
			t.Fatalf("%s missing %s: %v", tc.name, tc.file, err)
		}
		if _, err := os.Stat(filepath.Join(dest, "Dockerfile")); err != nil {
			t.Fatalf("%s missing Dockerfile: %v", tc.name, err)
		}
		m, _, err := manifest.LoadDir(dest)
		if err != nil {
			t.Fatalf("%s manifest: %v", tc.name, err)
		}
		if m.Runtime != string(tc.rt) || m.Preset != tc.preset || m.Name != tc.name {
			t.Fatalf("%s manifest = %+v", tc.name, m)
		}
		raw, err := os.ReadFile(filepath.Join(dest, tc.file))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "{{name}}") {
			t.Fatalf("%s still has placeholders", tc.name)
		}
	}
}

func TestInitDockerfileDefaultsBackend(t *testing.T) {
	parent := t.TempDir()
	dest, err := Init(Options{Name: "orders", Runtime: types.RuntimeDockerfile, Dir: parent})
	if err != nil {
		t.Fatal(err)
	}
	m, _, err := manifest.LoadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	if m.Runtime != "dockerfile" || m.Kind != "backend" {
		t.Fatalf("manifest = %+v", m)
	}
	if len(m.Triggers) == 0 || m.Triggers[0].Path != "/orders" || !m.Triggers[0].StripPrefix {
		t.Fatalf("triggers = %+v", m.Triggers)
	}
}

func TestInitRejectsUnknownRuntime(t *testing.T) {
	_, err := Init(Options{Name: "x", Runtime: types.Runtime("rust"), Dir: t.TempDir()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInitNodeHTTP(t *testing.T) {
	parent := t.TempDir()
	dest, err := Init(Options{Name: "hello-node", Runtime: types.RuntimeNode, Dir: parent})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"server.js", "package.json", "Dockerfile", "litefaas.yaml"} {
		if _, err := os.Stat(filepath.Join(dest, f)); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
	}
	m, err := manifest.Load(filepath.Join(dest, "litefaas.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Runtime != "node" || m.Name != "hello-node" {
		t.Fatalf("manifest = %+v", m)
	}
	if m.Kind != string(types.KindBackend) && m.Kind != "backend" {
		t.Fatalf("kind = %q want backend", m.Kind)
	}
	srv, err := os.ReadFile(filepath.Join(dest, "server.js"))
	if err != nil || !strings.Contains(string(srv), "healthz") {
		t.Fatalf("server.js missing healthz: %v", err)
	}
	df, err := os.ReadFile(filepath.Join(dest, "Dockerfile"))
	if err != nil || !strings.Contains(string(df), "PORT") {
		t.Fatalf("Dockerfile missing PORT: %v", err)
	}
}

func TestInitRejectsUnknownPreset(t *testing.T) {
	_, err := Init(Options{Name: "x", Runtime: types.RuntimeJava, Preset: "quarkus", Dir: t.TempDir()})
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
