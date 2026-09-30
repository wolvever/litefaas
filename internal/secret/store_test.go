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
	raw, err := os.ReadFile(filepath.Join(dir, DirName, DefaultEnv, "db.enc"))
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


func TestCorruptCiphertext(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Set("x", "ok"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, DirName, DefaultEnv, "x.enc")
	if err := os.WriteFile(path, []byte("not-valid-gcm"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = s.Get("x")
	if err == nil {
		t.Fatal("expected decrypt error")
	}
	if strings.Contains(err.Error(), "ok") {
		t.Fatal("plaintext in error")
	}
}

func TestResolveEnvNilGet(t *testing.T) {
	_, err := ResolveEnv(map[string]string{"A": "plain"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSetEmpty(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Set("x", ""); err == nil {
		t.Fatal("expected empty reject")
	}
}

func TestListRefs(t *testing.T) {
	refs := ListRefs(map[string]string{
		"DATABASE_URL": "${secret:db}",
		"PLAIN":        "x",
		"MIXED":        "prefix-${secret:tok}-suffix",
	})
	if len(refs) != 2 {
		t.Fatalf("refs = %+v", refs)
	}
	got := map[string]string{}
	for _, r := range refs {
		got[r.Key] = r.FormatRef()
	}
	if got["DATABASE_URL"] != "DATABASE_URL=${secret:db}" {
		t.Fatalf("got %v", got)
	}
	if _, ok := got["PLAIN"]; ok {
		t.Fatal("plaintext should be omitted")
	}
}


func TestEnvBagsIsolated(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetEnv("dev", "DB", "dev-val"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetEnv("prod", "DB", "prod-val"); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetEnv("dev", "DB")
	if err != nil || got != "dev-val" {
		t.Fatalf("dev=%q err=%v", got, err)
	}
	got, err = s.GetEnv("prod", "DB")
	if err != nil || got != "prod-val" {
		t.Fatalf("prod=%q err=%v", got, err)
	}
	if _, err := s.GetEnv("dev", "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
	names, err := s.ListEnv("dev")
	if err != nil || strings.Join(names, ",") != "DB" {
		t.Fatalf("list=%v err=%v", names, err)
	}
}

func TestMigrateFlatSecrets(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, DirName)
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	// Write a fake flat ciphertext via Open+Set then move up — or create via first Open.
	s0, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s0.Set("legacy", "v1"); err != nil {
		t.Fatal(err)
	}
	// Simulate pre-env layout: move default/legacy.enc to secrets/legacy.enc
	src := filepath.Join(root, DefaultEnv, "legacy.enc")
	dst := filepath.Join(root, "legacy.enc")
	if err := os.Rename(src, dst); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(root, DefaultEnv)) // may fail if not empty; ok
	s1, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s1.Get("legacy")
	if err != nil || got != "v1" {
		t.Fatalf("got %q err=%v", got, err)
	}
	if _, err := os.Stat(filepath.Join(root, "legacy.enc")); !os.IsNotExist(err) {
		t.Fatalf("flat file should be gone: %v", err)
	}
}

func TestValidateEnv(t *testing.T) {
	if err := ValidateEnv("Dev"); err == nil {
		t.Fatal("uppercase rejected")
	}
	if err := ValidateEnv("prod"); err != nil {
		t.Fatal(err)
	}
}

func TestMergeBag(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetEnv("prod", "DB", "prod-db"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetEnv("prod", "TOKEN", "prod-tok"); err != nil {
		t.Fatal(err)
	}
	get, err := s.GetterFor("prod")
	if err != nil {
		t.Fatal(err)
	}
	names, err := s.ListEnv("prod")
	if err != nil {
		t.Fatal(err)
	}
	out, err := MergeBag(map[string]string{"TOKEN": "manifest-wins", "EXTRA": "keep"}, names, get)
	if err != nil {
		t.Fatal(err)
	}
	if out["TOKEN"] != "manifest-wins" {
		t.Fatalf("precedence: %#v", out)
	}
	if out["DB"] != "prod-db" {
		t.Fatalf("injected: %#v", out)
	}
	if out["EXTRA"] != "keep" {
		t.Fatalf("kept: %#v", out)
	}
	// error path must not include plaintext
	_, err = MergeBag(nil, []string{"missing"}, get)
	if err == nil || strings.Contains(err.Error(), "prod-") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseSecretRef(t *testing.T) {
	bag, name, cross := ParseSecretRef("prod.DB")
	if !cross || bag != "prod" || name != "DB" {
		t.Fatalf("%q %q %v", bag, name, cross)
	}
	bag, name, cross = ParseSecretRef("prod/DB")
	if !cross || bag != "prod" || name != "DB" {
		t.Fatalf("slash %q %q %v", bag, name, cross)
	}
	bag, name, cross = ParseSecretRef("DB")
	if cross || name != "DB" {
		t.Fatalf("bare %q %q %v", bag, name, cross)
	}
	// Uppercase first segment is not an env bag → bare token
	_, name, cross = ParseSecretRef("PROD.DB")
	if cross || name != "PROD.DB" {
		t.Fatalf("upper %q %v", name, cross)
	}
}

func TestResolveEnvCross(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetEnv("prod", "DB", "prod-db-url"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetEnv("staging", "DB", "staging-db-url"); err != nil {
		t.Fatal(err)
	}
	get, err := s.GetterFor("staging")
	if err != nil {
		t.Fatal(err)
	}
	out, err := ResolveEnv(map[string]string{
		"A": "${secret:prod.DB}",
		"B": "${secret:prod/DB}",
		"C": "${secret:DB}",
	}, get)
	if err != nil {
		t.Fatal(err)
	}
	if out["A"] != "prod-db-url" || out["B"] != "prod-db-url" || out["C"] != "staging-db-url" {
		t.Fatalf("%#v", out)
	}
	_, err = ResolveEnv(map[string]string{"X": "${secret:prod.MISSING}"}, get)
	if err == nil || !strings.Contains(err.Error(), `unknown secret "MISSING" in env "prod"`) {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(err.Error(), "prod-db") {
		t.Fatalf("leaked value: %v", err)
	}
	refs := ListRefs(map[string]string{"A": "${secret:prod.DB}"})
	if len(refs) != 1 || refs[0].Name != "prod.DB" {
		t.Fatalf("%+v", refs)
	}
}
