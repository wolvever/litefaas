package manifest

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanDetectInventory(t *testing.T) {
	b, err := PlanDetect("../../examples/stacks/inventory", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Services) != 1 {
		t.Fatalf("services = %d", len(b.Services))
	}
	p := b.Services[0]
	if p.Stack != "go-gin-gorm" || !p.Detected {
		t.Fatalf("plan = %+v", p)
	}
	if p.Name != "inventory" || p.Kind != "backend" || p.Runtime != "go" {
		t.Fatalf("identity = %+v", p)
	}
	foundPG := false
	for _, s := range p.Hints.Sidecars {
		if s == "postgres" {
			foundPG = true
		}
	}
	if !foundPG {
		t.Fatalf("hints = %+v", p.Hints)
	}
	if p.DockerfileOrigin != "pack" && p.DockerfileOrigin != "local" {
		t.Fatalf("dockerfile origin = %s", p.DockerfileOrigin)
	}
	if len(p.Fingerprints) == 0 {
		t.Fatal("expected fingerprint hits")
	}
}

func TestPlanDetectLocalDockerfile(t *testing.T) {
	dir := t.TempDir()
	yaml := "name: hello\nkind: function\nruntime: go\n"
	if err := os.WriteFile(filepath.Join(dir, "litefaas.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := PlanDetect(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	p := b.Services[0]
	if p.Detected || p.Stack != "" {
		t.Fatalf("expected no pack, got %+v", p)
	}
	if p.DockerfileOrigin != "local" {
		t.Fatalf("origin = %s", p.DockerfileOrigin)
	}
}

func TestPlanDetectEmptyDir(t *testing.T) {
	dir := t.TempDir()
	_, err := PlanDetect(dir, "")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "lf stacks") || !strings.Contains(msg, "--stack") {
		t.Fatalf("error = %v", err)
	}
}

func TestPlanDetectForcedStack(t *testing.T) {
	dir := t.TempDir()
	// minimal go.mod so pack Dockerfile materialize origin is pack even without match
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := PlanDetect(dir, "go-gin-gorm")
	if err != nil {
		t.Fatal(err)
	}
	p := b.Services[0]
	if p.Detected || p.Stack != "go-gin-gorm" {
		t.Fatalf("plan = %+v", p)
	}
	if p.DockerfileOrigin != "pack" {
		t.Fatalf("origin = %s", p.DockerfileOrigin)
	}
}

func TestFormatPlanAndJSON(t *testing.T) {
	b, err := PlanDetect("../../examples/stacks/inventory", "")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := FormatPlan(&buf, b); err != nil {
		t.Fatal(err)
	}
	text := buf.String()
	for _, want := range []string{"go-gin-gorm", "inventory", "fingerprints:", "hints:", "not started by litefaas"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	buf.Reset()
	if err := WritePlanJSON(&buf, b); err != nil {
		t.Fatal(err)
	}
	var round PlanBundle
	if err := json.Unmarshal(buf.Bytes(), &round); err != nil {
		t.Fatal(err)
	}
	if len(round.Services) != 1 || round.Services[0].Stack != "go-gin-gorm" {
		t.Fatalf("json = %+v", round)
	}
}
