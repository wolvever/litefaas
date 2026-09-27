package secret

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Set("db", "postgres://secret@x/y"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("db")
	if err != nil || got != "postgres://secret@x/y" {
		t.Fatalf("got %q err=%v", got, err)
	}
	// ciphertext must not contain plaintext
	raw, err := os.ReadFile(filepath.Join(dir, DirName, "db.enc"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "postgres://secret") {
		t.Fatal("plaintext leaked into ciphertext file")
	}
	keyPath := filepath.Join(dir, KeyFileName)
	st, err := os.Stat(keyPath)
	if err != nil || st.Mode().Perm()&0o077 != 0 {
		t.Fatalf("key perms = %v err=%v", st.Mode(), err)
	}
}

func TestKeyPersists(t *testing.T) {
	dir := t.TempDir()
	s1, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = s1.Set("a", "one")
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s2.Get("a")
	if err != nil || got != "one" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestListDelete(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Set("b", "2")
	_ = s.Set("a", "1")
	names, err := s.List()
	if err != nil || strings.Join(names, ",") != "a,b" {
		t.Fatalf("list = %v err=%v", names, err)
	}
	if err := s.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveEnv(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Set("db", "s3cr3t")
	out, err := ResolveEnv(map[string]string{
		"DATABASE_URL": "${secret:db}",
		"PLAIN":        "ok",
		"MIXED":        "prefix-${secret:db}-suffix",
	}, s.Get)
	if err != nil {
		t.Fatal(err)
	}
	if out["DATABASE_URL"] != "s3cr3t" || out["PLAIN"] != "ok" || out["MIXED"] != "prefix-s3cr3t-suffix" {
		t.Fatalf("out = %#v", out)
	}
	_, err = ResolveEnv(map[string]string{"X": "${secret:missing}"}, s.Get)
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "s3cr3t") {
		t.Fatal("plaintext in error")
	}
}

func TestValidateName(t *testing.T) {
	if err := ValidateName("../etc"); err == nil {
		t.Fatal("expected invalid")
	}
}
