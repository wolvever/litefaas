package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDotEnv(t *testing.T) {
	src := `
# comment
FOO=bar

export BAZ=qux
QUOTED="hello world"
SINGLE='a=b=c'
EMPTY=
TRAIL=val # comment
`
	pairs, err := parseDotEnv(src)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"FOO":    "bar",
		"BAZ":    "qux",
		"QUOTED": "hello world",
		"SINGLE": "a=b=c",
		"EMPTY":  "",
		"TRAIL":  "val",
	}
	if len(pairs) != len(want) {
		t.Fatalf("len=%d pairs=%+v", len(pairs), pairs)
	}
	for _, p := range pairs {
		if want[p.Name] != p.Value {
			t.Fatalf("%s: got %q want %q", p.Name, p.Value, want[p.Name])
		}
		delete(want, p.Name)
	}
}

func TestParseDotEnvInvalidName(t *testing.T) {
	_, err := parseDotEnv("../etc=supersecret\n")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if strings.Contains(msg, "supersecret") {
		t.Fatalf("error leaked value: %v", err)
	}
	if !strings.Contains(msg, "invalid secret name") {
		t.Fatalf("want invalid secret name, got %v", err)
	}
}

func TestParseDotEnvBadLine(t *testing.T) {
	_, err := parseDotEnv("NOEQUALS\n")
	if err == nil || !strings.Contains(err.Error(), "KEY=VALUE") {
		t.Fatalf("got %v", err)
	}
}

func TestParseDotEnvUnclosedQuote(t *testing.T) {
	_, err := parseDotEnv("FOO=\"bar\n")
	if err == nil || !strings.Contains(err.Error(), "unclosed quote") {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(err.Error(), "bar") {
		t.Fatalf("leaked value in error: %v", err)
	}
}

func TestSecretImportDryRunNoAPI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("A=secret-value-1\nB=secret-value-2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// dry-run must not dial a gateway; use a nonsense gateway so any Put would fail hard.
	err := secretImport([]string{path, "--dry-run", "--gateway", "http://127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
}
