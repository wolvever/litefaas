package main

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShouldRebuild(t *testing.T) {
	cases := []struct {
		rel  string
		want bool
	}{
		{"handler.go", true},
		{"src/App.tsx", true},
		{"litefaas.yaml", true},
		{"pkg/foo/bar.go", true},
		{".git/HEAD", false},
		{"node_modules/x", false},
		{"node_modules/pkg/index.js", false},
		{"dist/a.js", false},
		{"app/.next/cache", false},
		{"build/out", false},
		{"target/classes/X.class", false},
		{"__pycache__/x.pyc", false},
		{".venv/lib/python", false},
		{"venv/bin/activate", false},
		{"foo.pyc", false},
		{"foo.pyo", false},
		{".DS_Store", false},
		{"subdir/.DS_Store", false},
		{".idea/workspace.xml", false},
		{".vscode/settings.json", false},
		{".pytest_cache/v", false},
		{"", false},
		{".", false},
	}
	for _, tc := range cases {
		got := ShouldRebuild(tc.rel, nil)
		if got != tc.want {
			t.Errorf("ShouldRebuild(%q)=%v want %v", tc.rel, got, tc.want)
		}
	}
}

func TestShouldRebuildExtraIgnore(t *testing.T) {
	extra := []string{"tmp", "*.log", "vendor"}
	if ShouldRebuild("tmp/x", extra) {
		t.Fatal("segment tmp should be ignored")
	}
	if ShouldRebuild("app.log", extra) {
		t.Fatal("*.log should be ignored")
	}
	if ShouldRebuild("vendor/pkg/x.go", extra) {
		t.Fatal("vendor should be ignored")
	}
	if !ShouldRebuild("main.go", extra) {
		t.Fatal("main.go should rebuild")
	}
	if ShouldRebuild("logs/app.log", extra) {
		t.Fatal("glob against suffix path should ignore")
	}
}

func TestCollectWatchDirsSkipsIgnored(t *testing.T) {
	root := t.TempDir()
	mustMk := func(parts ...string) {
		p := filepath.Join(append([]string{root}, parts...)...)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustMk("src")
	mustMk("node_modules", "pkg")
	mustMk(".git", "objects")
	mustMk("dist", "assets")
	mustMk("tmp", "cache") // custom via --ignore

	dirs, err := collectWatchDirs(root, []string{"tmp"})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, d := range dirs {
		rel, err := filepath.Rel(root, d)
		if err != nil {
			t.Fatal(err)
		}
		seen[filepath.ToSlash(rel)] = true
	}
	if !seen["."] && !seen[""] {
		// root itself is listed as abs path; Rel of root to root is "."
		if !seen["."] {
			// check root is present as absolute
			foundRoot := false
			for _, d := range dirs {
				if d == root {
					foundRoot = true
					break
				}
			}
			if !foundRoot {
				t.Fatalf("root missing from %v", dirs)
			}
		}
	}
	for _, bad := range []string{"node_modules", "node_modules/pkg", ".git", ".git/objects", "dist", "dist/assets", "tmp", "tmp/cache"} {
		for _, d := range dirs {
			rel, _ := filepath.Rel(root, d)
			if filepath.ToSlash(rel) == bad {
				t.Fatalf("should not watch %s; got %v", bad, dirs)
			}
		}
	}
	foundSrc := false
	for _, d := range dirs {
		rel, _ := filepath.Rel(root, d)
		if filepath.ToSlash(rel) == "src" {
			foundSrc = true
		}
	}
	if !foundSrc {
		t.Fatalf("expected src in watch set: %v", dirs)
	}
}

func TestDebouncerCoalesce(t *testing.T) {
	d := NewDebouncer(80 * time.Millisecond)
	defer d.Stop()
	for i := 0; i < 5; i++ {
		d.Hit()
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case <-d.C:
		// first fire
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected one coalesced fire")
	}
	// No second fire without more hits.
	select {
	case <-d.C:
		t.Fatal("unexpected second fire")
	case <-time.After(120 * time.Millisecond):
	}
}

func TestWatchFlagParse(t *testing.T) {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	stack := fs.String("stack", "", "")
	debounce := fs.Duration("debounce", 500*time.Millisecond, "")
	var ignore stringList
	fs.Var(&ignore, "ignore", "")
	noUp := fs.Bool("no-up", false, "")
	once := fs.Bool("once", false, "")
	rest, err := parseMixed(fs, []string{
		"examples/hello",
		"--stack", "go-function",
		"--debounce", "250ms",
		"--ignore", "tmp",
		"--ignore", "*.log",
		"--no-up",
		"--once",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rest) != 1 || rest[0] != "examples/hello" {
		t.Fatalf("rest=%v", rest)
	}
	if *stack != "go-function" {
		t.Fatalf("stack=%s", *stack)
	}
	if *debounce != 250*time.Millisecond {
		t.Fatalf("debounce=%s", *debounce)
	}
	if len(ignore) != 2 || ignore[0] != "tmp" || ignore[1] != "*.log" {
		t.Fatalf("ignore=%v", ignore)
	}
	if !*noUp || !*once {
		t.Fatalf("no-up=%v once=%v", *noUp, *once)
	}
}

func TestIsIgnoredDirName(t *testing.T) {
	if !isIgnoredDirName("node_modules", nil) {
		t.Fatal("node_modules")
	}
	if isIgnoredDirName("src", nil) {
		t.Fatal("src should not be ignored")
	}
	if !isIgnoredDirName("vendor", []string{"vendor"}) {
		t.Fatal("extra vendor")
	}
}
