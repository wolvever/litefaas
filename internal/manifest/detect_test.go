package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDetectExamples(t *testing.T) {
	cases := []struct {
		dir, stack, name string
	}{
		{"../../examples/stacks/shop", "java-spring-mybatis", "shop"},
		{"../../examples/stacks/catalog", "python-fastapi", "catalog"},
		{"../../examples/stacks/inventory", "go-gin-gorm", "inventory"},
		{"../../examples/stacks/notes", "python-flask-sqlalchemy", "notes"},
		{"../../examples/stacks/tickets", "node-express-prisma", "tickets"},
		{"../../examples/stacks/library", "java-spring-jpa", "library"},
		{"../../examples/stacks/portal", "node-nextjs", "portal"},
	}
	for _, tc := range cases {
		res, err := ResolveDetect(tc.dir, "")
		if err != nil {
			t.Fatalf("%s: %v", tc.dir, err)
		}
		if res.Multi != nil || !res.Detected || res.Pack == nil || res.Pack.ID != tc.stack {
			t.Fatalf("%s: %+v", tc.dir, res)
		}
		if res.Manifest.Name != tc.name || res.Manifest.Stack != tc.stack || res.Manifest.Kind != "backend" {
			t.Fatalf("%s manifest = %+v", tc.dir, res.Manifest)
		}
		if res.Manifest.Runtime == "" || res.Manifest.Image != tc.name+":latest" {
			t.Fatalf("%s defaults = %+v", tc.dir, res.Manifest)
		}
	}
}

func TestResolveDetectStackFlagWins(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("fastapi==0.115.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := ResolveDetect(dir, "go-gin-gorm")
	if err != nil {
		t.Fatal(err)
	}
	if res.Detected || res.Pack == nil || res.Pack.ID != "go-gin-gorm" || res.Manifest.Runtime != "go" {
		t.Fatalf("override = %+v", res)
	}
}

func TestResolveDetectOneLineYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("stack: python-fastapi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := ResolveDetect(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Detected || res.Manifest.Stack != "python-fastapi" || res.Manifest.Runtime != "python" {
		t.Fatalf("one-line = %+v", res)
	}
	if res.Manifest.Name == "" {
		t.Fatal("expected name from dir")
	}
}

func TestResolveDetectExplicitYAMLWins(t *testing.T) {
	dir := t.TempDir()
	raw := "name: hello\nruntime: go\nkind: function\n"
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("fastapi==0.115.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := ResolveDetect(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Detected || res.Pack != nil || res.Manifest.Runtime != "go" || res.Manifest.Name != "hello" {
		t.Fatalf("explicit = %+v", res)
	}
}

func TestResolveDetectFlagBeatsYAMLStack(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("stack: python-fastapi\nname: api\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := ResolveDetect(dir, "java-spring-mybatis")
	if err != nil {
		t.Fatal(err)
	}
	if res.Manifest.Runtime != "java" || res.Pack.ID != "java-spring-mybatis" || res.Manifest.Name != "api" {
		t.Fatalf("flag vs yaml = %+v", res)
	}
}

func TestResolveDetectUnknownStack(t *testing.T) {
	dir := t.TempDir()
	if _, err := ResolveDetect(dir, "quarkus"); err == nil || !strings.Contains(err.Error(), "unknown stack") {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveDetectNoMatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("nothing"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveDetect(dir, ""); err == nil || !strings.Contains(err.Error(), "no stack pack") {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveDetectRejectsStackYAMLWithFlag(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, StackFileName), []byte("services:\n  web:\n    runtime: static\n    kind: frontend\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveDetect(dir, "python-fastapi"); err == nil || !strings.Contains(err.Error(), "--stack") {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveDetectExamplesMultiService(t *testing.T) {
	res, err := ResolveDetect("../../examples", "")
	if err != nil || res.Multi == nil || res.Manifest != nil {
		t.Fatalf("examples = %+v err=%v", res, err)
	}
	if len(res.Multi.Services) != 3 {
		t.Fatalf("services = %d", len(res.Multi.Services))
	}
}

func TestResolveStillRequiresManifest(t *testing.T) {
	dir := t.TempDir()
	if _, _, _, err := Resolve(dir); err == nil {
		t.Fatal("Resolve without yaml should still fail")
	}
}

func TestShopMemoryFromPack(t *testing.T) {
	res, err := ResolveDetect("../../examples/stacks/shop", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Manifest.Memory != 512 {
		t.Fatalf("shop memory = %d", res.Manifest.Memory)
	}
}
