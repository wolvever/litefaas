package dirty

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashIgnoresVendorAndSeesEdits(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "vendor", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "vendor", "x", "a.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := HashDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "vendor", "x", "a.go"), []byte("package x\n// changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := HashDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("vendor change affected hash")
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n// edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := HashDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if a == c {
		t.Fatal("source edit did not change hash")
	}
}

func TestLockfileForcesAllServices(t *testing.T) {
	root := t.TempDir()
	api := filepath.Join(root, "api")
	web := filepath.Join(root, "web")
	for _, d := range []string{api, web} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	svcs := []Service{{Name: "api", Dir: api}, {Name: "web", Dir: web}}
	rebuild, next, err := Plan(root, State{}, svcs)
	if err != nil {
		t.Fatal(err)
	}
	if !rebuild["api"] || !rebuild["web"] {
		t.Fatalf("first plan = %+v", rebuild)
	}
	prev := next
	rebuild, next, err = Plan(root, prev, svcs)
	if err != nil {
		t.Fatal(err)
	}
	if len(rebuild) != 0 {
		t.Fatalf("clean plan rebuilt %+v", rebuild)
	}
	if err := os.WriteFile(filepath.Join(root, "go.sum"), []byte("example v1.0.0 h1:abc=\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rebuild, _, err = Plan(root, prev, svcs)
	if err != nil {
		t.Fatal(err)
	}
	if !rebuild["api"] || !rebuild["web"] {
		t.Fatalf("lockfile plan = %+v", rebuild)
	}
	// service-only edit
	if err := os.WriteFile(filepath.Join(api, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// refresh prev to include lockfile
	_, prev, err = Plan(root, State{}, svcs)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(api, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rebuild, _, err = Plan(root, prev, svcs)
	if err != nil {
		t.Fatal(err)
	}
	if !rebuild["api"] || rebuild["web"] {
		t.Fatalf("partial = %+v", rebuild)
	}
}
