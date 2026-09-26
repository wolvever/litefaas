package builder

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wolvever/litefaas/internal/scaffold"
)

func TestBuildUsesDockerCLI(t *testing.T) {
	bin := t.TempDir()
	logf := filepath.Join(t.TempDir(), "docker.log")
	script := "#!/bin/sh\n" +
		"echo \"$0 $@\" >> " + logf + "\n" +
		"if [ \"$1\" = \"version\" ]; then echo 24.0.0; exit 0; fi\n" +
		"if [ \"$1\" = \"build\" ]; then exit 0; fi\n" +
		"exit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	dest := t.TempDir()
	if _, err := scaffold.Init(scaffold.Options{Name: "hello", Runtime: "go", Dest: dest}); err != nil {
		t.Fatal(err)
	}
	var buf strings.Builder
	image, err := Build(context.Background(), dest, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if image != "hello:latest" {
		t.Fatalf("image = %q", image)
	}
	raw, err := os.ReadFile(logf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "build -t hello:latest") {
		t.Fatalf("docker log = %s", raw)
	}
}

func TestBuildRequiresManifest(t *testing.T) {
	_, err := Build(context.Background(), t.TempDir(), nil)
	if err == nil {
		t.Fatal("expected missing manifest error")
	}
}
