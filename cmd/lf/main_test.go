package main

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCmdTokenFromFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte("filetok\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"token", "--config-dir", dir}); err != nil {
		t.Fatal(err)
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
