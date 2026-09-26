package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/wolvever/litefaas/internal/types"
)

func TestCRUDAndPersist(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	created, err := st.Create(types.Resource{
		Name:    "Hello-Fn",
		Kind:    types.KindFunction,
		Runtime: types.RuntimeGo,
		Image:   "hello:dev",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "hello-fn" {
		t.Fatalf("name normalize: %q", created.Name)
	}
	if created.Port != types.DefaultPort || created.Health != types.DefaultHealth {
		t.Fatalf("defaults: %+v", created)
	}
	if len(created.Revisions) != 1 || created.Revisions[0].Image != "hello:dev" {
		t.Fatalf("initial revision: %+v", created.Revisions)
	}

	if _, err := st.Create(types.Resource{Name: "hello-fn", Kind: types.KindFunction, Runtime: types.RuntimeGo}); !errors.Is(err, ErrExists) {
		t.Fatalf("expected exists, got %v", err)
	}

	list, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("list len=%d", len(list))
	}

	if _, err := st.Deploy("hello-fn", "hello:2"); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	st2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()

	got, err := st2.Get("hello-fn")
	if err != nil {
		t.Fatal(err)
	}
	if got.Image != "hello:2" {
		t.Fatalf("persisted image %q", got.Image)
	}
	if len(got.Revisions) != 2 {
		t.Fatalf("revisions=%d", len(got.Revisions))
	}
	if _, err := os.Stat(filepath.Join(dir, "litefaas.db")); err != nil {
		t.Fatal(err)
	}

	if err := st2.Delete("hello-fn"); err != nil {
		t.Fatal(err)
	}
	if _, err := st2.Get("hello-fn"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestRejectsInvalidName(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_, err = st.Create(types.Resource{Name: "not_valid", Kind: types.KindFunction, Runtime: types.RuntimeGo})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
