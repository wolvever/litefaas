package stackpack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedPacks(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"go-gin-gorm", "java-spring-mybatis", "python-fastapi"}
	got := cat.IDs()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ids = %v want %v", got, want)
	}
	for _, id := range want {
		p, err := cat.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Dockerfile) == 0 || !strings.Contains(string(p.Dockerfile), "$PORT") && !strings.Contains(string(p.Dockerfile), "${PORT}") && !strings.Contains(string(p.Dockerfile), "PORT") {
			t.Fatalf("%s Dockerfile missing PORT contract", id)
		}
		if len(p.Match) == 0 {
			t.Fatalf("%s has no match rules", id)
		}
	}
}

func TestDetectExamples(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		dir string
		id  string
	}{
		{"../../examples/stacks/shop", "java-spring-mybatis"},
		{"../../examples/stacks/catalog", "python-fastapi"},
		{"../../examples/stacks/inventory", "go-gin-gorm"},
	}
	for _, tc := range cases {
		p, err := cat.Detect(tc.dir)
		if err != nil {
			t.Fatalf("%s: %v", tc.dir, err)
		}
		if p.ID != tc.id {
			t.Fatalf("%s: got %s want %s", tc.dir, p.ID, tc.id)
		}
	}
}

func TestDetectFastAPIPyproject(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\ndependencies = [\"fastapi>=0.115\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "python-fastapi" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestDetectRejectsNearMisses(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project><artifactId>spring-boot-starter-web</artifactId></project>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := cat.Detect(dir); err == nil {
		t.Fatal("spring without mybatis should not match")
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\nrequire github.com/gin-gonic/gin v1.10.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := cat.Detect(dir); err == nil {
		t.Fatal("gin without gorm should not match")
	}
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask==3.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := cat.Detect(dir); err == nil {
		t.Fatal("flask should not match fastapi pack")
	}
}

func TestDetectAmbiguous(t *testing.T) {
	cat := NewCatalog()
	a := &Pack{ID: "one", Runtime: "go", Priority: 10, Match: []RuleSet{{Files: []string{"go.mod"}}}}
	b := &Pack{ID: "two", Runtime: "go", Priority: 10, Match: []RuleSet{{Files: []string{"go.mod"}}}}
	if err := cat.Add(a); err != nil {
		t.Fatal(err)
	}
	if err := cat.Add(b); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := cat.Detect(dir)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("err = %v", err)
	}
}

func TestDetectPriorityBreaksTie(t *testing.T) {
	cat := NewCatalog()
	_ = cat.Add(&Pack{ID: "low", Runtime: "go", Priority: 1, Match: []RuleSet{{Files: []string{"go.mod"}}}})
	_ = cat.Add(&Pack{ID: "high", Runtime: "go", Priority: 50, Match: []RuleSet{{Files: []string{"go.mod"}}}})
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "high" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestNameFromDir(t *testing.T) {
	if g := NameFromDir("/tmp/catalog"); g != "catalog" {
		t.Fatalf("got %q", g)
	}
	if g := NameFromDir("/tmp/weird name!!"); g != "weird-name" {
		t.Fatalf("got %q", g)
	}
}

func TestWriteDockerfile(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	p, err := cat.Get("python-fastapi")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := p.WriteDockerfile(dir); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "uvicorn") {
		t.Fatalf("Dockerfile = %s", raw)
	}
	if _, err := os.Stat(filepath.Join(dir, ".dockerignore")); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownStack(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cat.Get("quarkus"); err == nil {
		t.Fatal("expected unknown stack")
	}
}

func TestMergeOverlay(t *testing.T) {
	base, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	extra := NewCatalog()
	_ = extra.Add(&Pack{
		ID:      "python-fastapi",
		Runtime: "python",
		Title:   "overlay",
		Match:   []RuleSet{{Files: []string{"requirements.txt"}}},
	})
	base.Merge(extra)
	p, err := base.Get("python-fastapi")
	if err != nil || p.Title != "overlay" {
		t.Fatalf("overlay = %+v err=%v", p, err)
	}
}
