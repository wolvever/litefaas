package token

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreate(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "token")
	a, created, err := LoadOrCreate(p)
	if err != nil || !created || len(a) < 32 {
		t.Fatalf("create = %q created=%v err=%v", a, created, err)
	}
	b, created, err := LoadOrCreate(p)
	if err != nil || created || b != a {
		t.Fatalf("reload = %q created=%v err=%v", b, created, err)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("perm = %o", st.Mode().Perm())
	}
}

func TestResolveInsecureAndExplicit(t *testing.T) {
	tok, created, err := Resolve("", "/no/such", true)
	if err != nil || created || tok != "" {
		t.Fatalf("insecure = %q %v %v", tok, created, err)
	}
	tok, created, err = Resolve("abc", "/no/such", false)
	if err != nil || created || tok != "abc" {
		t.Fatalf("explicit = %q %v %v", tok, created, err)
	}
}
