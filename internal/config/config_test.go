package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadAndResolve(t *testing.T) {
	t.Setenv("LITEFAAS_HOME", t.TempDir())
	t.Setenv("LITEFAAS_GATEWAY", "")
	t.Setenv("LITEFAAS_TOKEN", "")

	if err := Save(File{Gateway: "http://example:9090", Token: "abc"}); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Gateway != "http://example:9090" || got.Token != "abc" {
		t.Fatalf("load %+v", got)
	}
	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "config.json" {
		t.Fatalf("path %s", p)
	}

	if g := ResolveGateway("flag-gw", got); g != "flag-gw" {
		t.Fatalf("flag gateway %s", g)
	}
	if g := ResolveGateway("", got); g != "http://example:9090" {
		t.Fatalf("cfg gateway %s", g)
	}
	t.Setenv("LITEFAAS_TOKEN", "envtok")
	if tok := ResolveToken("", got); tok != "envtok" {
		t.Fatalf("env token %s", tok)
	}
}

func TestLoadMissing(t *testing.T) {
	t.Setenv("LITEFAAS_HOME", filepath.Join(t.TempDir(), "missing"))
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != (File{}) {
		t.Fatalf("expected empty, got %+v", got)
	}
	if os.Getenv("LITEFAAS_GATEWAY") == "" && ResolveGateway("", got) != DefaultGW {
		t.Fatalf("default gateway")
	}
}
