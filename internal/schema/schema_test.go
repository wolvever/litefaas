package schema

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedMatchesPublishedSchema(t *testing.T) {
	pub, err := os.ReadFile(filepath.Join("..", "..", "schema", "litefaas.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(pub), bytes.TrimSpace(Embedded())) {
		t.Fatal("schema/litefaas.schema.json drifted from the embedded copy")
	}
}

func TestUnknownKeyWarnsAndTypesError(t *testing.T) {
	dir := t.TempDir()
	raw := []byte("name: api\nruntme: go\nrelease:\n  - echo ok\n  - 3\n")
	path := filepath.Join(dir, "litefaas.yaml")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	diags, err := ValidateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var warn, errd bool
	for _, d := range diags {
		if d.Severity == "warning" && strings.Contains(d.Path, "runtme") {
			warn = true
		}
		if d.Severity == "error" && strings.Contains(d.Message, "runtime") {
			errd = true
		}
		if d.Severity == "error" && strings.Contains(d.Path, "release[1]") {
			errd = true
		}
	}
	if !warn {
		t.Fatalf("missing unknown-key warning: %+v", diags)
	}
	if !errd {
		t.Fatalf("missing errors: %+v", diags)
	}
}

func TestStackServiceUnknownKey(t *testing.T) {
	dir := t.TempDir()
	raw := []byte("services:\n  api:\n    runtime: go\n    nope: 1\n")
	path := filepath.Join(dir, "stack.yaml")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	diags, err := ValidateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range diags {
		if d.Severity == "warning" && strings.Contains(d.Path, "nope") {
			found = true
		}
		if d.Severity == "error" && strings.Contains(d.Path, "name") {
			t.Fatalf("stack key should satisfy name: %+v", diags)
		}
	}
	if !found {
		t.Fatalf("diags=%+v", diags)
	}
}

func TestExternalSchemaWarns(t *testing.T) {
	dir := t.TempDir()
	raw := []byte("$schema: https://example.invalid/other.json\nname: api\nruntime: go\n")
	path := filepath.Join(dir, "litefaas.yaml")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	diags, err := ValidateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if HasErrors(diags) {
		t.Fatalf("unexpected errors %+v", diags)
	}
	ok := false
	for _, d := range diags {
		if d.Severity == "warning" && strings.Contains(d.Message, "not fetched") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("diags=%+v", diags)
	}
}

func TestExampleStackValid(t *testing.T) {
	diags, err := ValidateFile(filepath.Join("..", "..", "examples", "stack.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if HasErrors(diags) {
		t.Fatalf("%+v", diags)
	}
}
