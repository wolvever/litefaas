package token

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateLength(t *testing.T) {
	a, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	b, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 64 || len(b) != 64 {
		t.Fatalf("len %d %d", len(a), len(b))
	}
	if a == b {
		t.Fatal("expected unique tokens")
	}
}

func TestLoadOrCreatePersists(t *testing.T) {
	dir := t.TempDir()
	p := Path(dir)
	tok, created, err := LoadOrCreate(p)
	if err != nil || !created || tok == "" {
		t.Fatalf("first = %q created=%v err=%v", tok, created, err)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("perm = %o", st.Mode().Perm())
	}
	again, created, err := LoadOrCreate(p)
	if err != nil || created || again != tok {
		t.Fatalf("second = %q created=%v err=%v", again, created, err)
	}
}

func TestResolveDaemon(t *testing.T) {
	tok, info, err := ResolveDaemon("s3cret", t.TempDir(), false)
	if err != nil || tok != "s3cret" || info.Mode != "explicit" {
		t.Fatalf("explicit = %q %+v %v", tok, info, err)
	}
	tok, info, err = ResolveDaemon("", t.TempDir(), true)
	if err != nil || tok != "" || info.Mode != "off" {
		t.Fatalf("off = %q %+v %v", tok, info, err)
	}
	dir := t.TempDir()
	tok, info, err = ResolveDaemon("", dir, false)
	if err != nil || info.Mode != "file" || !info.Generated || tok == "" {
		t.Fatalf("file = %q %+v %v", tok, info, err)
	}
	if info.Path != filepath.Join(dir, FileName) {
		t.Fatalf("path = %s", info.Path)
	}
}

func TestResolveClientPrecedence(t *testing.T) {
	dir := t.TempDir()
	if err := WriteFile(Path(dir), "fromfile"); err != nil {
		t.Fatal(err)
	}
	if got := ResolveClient("flag", "env", "ctx", dir); got != "flag" {
		t.Fatalf("flag = %q", got)
	}
	if got := ResolveClient("", "env", "ctx", dir); got != "env" {
		t.Fatalf("env = %q", got)
	}
	if got := ResolveClient("", "", "ctx", dir); got != "ctx" {
		t.Fatalf("ctx = %q", got)
	}
	if got := ResolveClient("", "", "", dir); got != "fromfile" {
		t.Fatalf("file = %q", got)
	}
	if got := ResolveClient("", "", "", t.TempDir()); got != "" {
		t.Fatalf("missing = %q", got)
	}
}
