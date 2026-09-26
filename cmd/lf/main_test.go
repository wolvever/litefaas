package main

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInitScaffoldsGoFunction(t *testing.T) {
	dest := t.TempDir()
	if err := run([]string{"init", "hello", "--runtime", "go", "--dir", dest}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "litefaas.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "Dockerfile")); err != nil {
		t.Fatal(err)
	}
}

func TestInitRequiresRuntime(t *testing.T) {
	if err := run([]string{"init", "hello"}); err == nil {
		t.Fatal("expected usage error")
	}
}

func TestParseMixedFlagsAfterName(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	gw := fs.String("gateway", "", "")
	dir := fs.String("config-dir", "", "")
	pos, err := parseMixed(fs, []string{"orders-api", "--gateway", "http://127.0.0.1:18080", "--config-dir", "/tmp/cfg"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pos, []string{"orders-api"}) {
		t.Fatalf("pos = %v", pos)
	}
	if *gw != "http://127.0.0.1:18080" || *dir != "/tmp/cfg" {
		t.Fatalf("gateway=%q config-dir=%q", *gw, *dir)
	}
}
