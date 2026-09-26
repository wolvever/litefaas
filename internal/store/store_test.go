package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/wolvever/litefaas/internal/types"
)

func TestResourceCRUDPersists(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	got, err := s.Create(types.Resource{
		Name:    "orders-api",
		Kind:    types.KindBackend,
		Runtime: types.RuntimeJava,
		Image:   "localhost:5000/orders-api:0.1.0",
		Port:    8080,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "orders-api" || got.CreatedAt.IsZero() {
		t.Fatalf("create = %+v", got)
	}

	_, err = s.Create(types.Resource{Name: "orders-api", Kind: types.KindBackend, Runtime: types.RuntimeJava})
	if !errors.Is(err, ErrExists) {
		t.Fatalf("dup = %v, want ErrExists", err)
	}

	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if _, err := os.Stat(filepath.Join(dir, filename)); err != nil {
		t.Fatalf("db file missing: %v", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Runtime != types.RuntimeJava {
		t.Fatalf("list after reopen = %+v", list)
	}

	updated, err := s.Update(types.Resource{
		Name:    "orders-api",
		Kind:    types.KindBackend,
		Runtime: types.RuntimeJava,
		Image:   "localhost:5000/orders-api:0.2.0",
		Port:    8080,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Image != "localhost:5000/orders-api:0.2.0" || updated.CreatedAt.IsZero() {
		t.Fatalf("update = %+v", updated)
	}

	rev, err := s.AddRevision("orders-api", "localhost:5000/orders-api:0.2.0", "deployed")
	if err != nil {
		t.Fatal(err)
	}
	if rev.ID == 0 {
		t.Fatal("expected revision id")
	}

	if err := s.Delete("orders-api"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("orders-api"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after delete = %v", err)
	}
}
