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

func TestRouteOverridePersists(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if _, ok, err := s.GetRouteOverride(); err != nil || ok {
		t.Fatalf("empty override ok=%v err=%v", ok, err)
	}
	want := []RouteSpec{{Path: "/orders", Name: "orders", StripPrefix: true}}
	if err := s.SetRouteOverride(want); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, ok, err := s.GetRouteOverride()
	if err != nil || !ok || len(got) != 1 || got[0].Name != "orders" || !got[0].StripPrefix {
		t.Fatalf("override = %+v ok=%v err=%v", got, ok, err)
	}
	if err := s.ClearRouteOverride(); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.GetRouteOverride(); err != nil || ok {
		t.Fatalf("cleared ok=%v err=%v", ok, err)
	}
}

func TestEdgeRulesPersist(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if _, ok, err := s.GetEdgeRules(); err != nil || ok {
		t.Fatalf("empty ok=%v err=%v", ok, err)
	}
	want := []EdgeRuleSpec{{From: "/old", To: "/new", Status: 301, Source: "_redirects"}}
	if err := s.SetEdgeRules(want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.GetEdgeRules()
	if err != nil || !ok || len(got) != 1 || got[0].From != "/old" {
		t.Fatalf("got=%+v ok=%v err=%v", got, ok, err)
	}
	if err := s.ClearEdgeRules(); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.GetEdgeRules(); err != nil || ok {
		t.Fatalf("cleared ok=%v err=%v", ok, err)
	}
}

func TestRevisionSnapshotPinAndPrune(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if _, err := s.Create(types.Resource{Name: "api", Kind: types.KindBackend, Runtime: types.RuntimeGo}); err != nil {
		t.Fatal(err)
	}
	first, err := s.AddRevisionFull(types.Revision{
		Name: "api", Image: "api:1", ImageID: "sha256:one", Status: "deployed",
		Snapshot: types.RevisionSnapshot{
			PackID: "go-chi",
			EnvRefs: []types.SnapshotEnvRef{
				{Key: "DATABASE_URL", Ref: "${secret:db}"},
				{Key: "TOKEN", Ref: "super-secret-value"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Snapshot.EnvRefs[1].Ref != "" {
		t.Fatalf("plaintext stored: %+v", first.Snapshot.EnvRefs)
	}
	if _, err := s.SetRevisionPinned("api", first.ID, true); err != nil {
		t.Fatal(err)
	}
	for i := 2; i <= 7; i++ {
		if _, err := s.AddRevision("api", "api:"+string(rune('0'+i)), "deployed"); err != nil {
			t.Fatal(err)
		}
	}
	n, err := s.PruneRevisions("api", DefaultRevisionKeep)
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("expected prune, n=%d", n)
	}
	list, err := s.ListRevisions("api")
	if err != nil {
		t.Fatal(err)
	}
	// 7 rows, keep 5, but pinned oldest is exempt so we delete unpinned until
	// remaining would be 5. Newest is kept. Excess starts at 2.
	if len(list) != 5 {
		t.Fatalf("len=%d %+v", len(list), list)
	}
	if list[0].ID != first.ID || !list[0].Pinned {
		t.Fatalf("pinned oldest dropped: %+v", list[0])
	}
	if list[0].ImageID != "sha256:one" || list[0].Snapshot.PackID != "go-chi" {
		t.Fatalf("snapshot lost: %+v", list[0])
	}
	if list[0].Snapshot.EnvRefs[0].Ref != "${secret:db}" {
		t.Fatalf("ref lost: %+v", list[0].Snapshot)
	}
	reopen := list
	_ = reopen
}

func TestEdgeRulesProjectScope(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// Legacy array row (pre-project document).
	now := "2026-01-01T00:00:00Z"
	legacy := `[{"from":"/old","to":"/legacy","status":301}]`
	if _, err := s.db.Exec(`INSERT INTO edge_rules (id, spec_json, updated_at) VALUES (1, ?, ?)`, legacy, now); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.GetEdgeRules()
	if err != nil || !ok || len(got) != 1 || got[0].To != "/legacy" || got[0].Project != "" {
		t.Fatalf("legacy got=%+v ok=%v err=%v", got, ok, err)
	}

	// First scoped write replaces the sole legacy bucket.
	if err := s.SetProjectEdgeRules("web", []EdgeRuleSpec{{From: "/a", To: "/a2", Status: 301}}); err != nil {
		t.Fatal(err)
	}
	got, _, err = s.GetEdgeRules()
	if err != nil || len(got) != 1 || got[0].Project != "web" || got[0].From != "/a" {
		t.Fatalf("upgraded %+v %v", got, err)
	}

	if err := s.SetProjectEdgeRules("api", []EdgeRuleSpec{{From: "/b", To: "/b2", Status: 302}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetProjectEdgeRules("web", []EdgeRuleSpec{{From: "/a", To: "/a3", Status: 301}}); err != nil {
		t.Fatal(err)
	}
	got, _, err = s.GetEdgeRules()
	if err != nil || len(got) != 2 {
		t.Fatalf("merged %+v %v", got, err)
	}
	if got[0].Project != "web" || got[0].To != "/a3" || got[1].Project != "api" || got[1].From != "/b" {
		t.Fatalf("order/update %+v", got)
	}

	// Bare replace wipes named projects.
	if err := s.SetEdgeRules([]EdgeRuleSpec{{From: "/z", To: "/z", Status: 301}}); err != nil {
		t.Fatal(err)
	}
	got, _, err = s.GetEdgeRules()
	if err != nil || len(got) != 1 || got[0].Project != "" || got[0].From != "/z" {
		t.Fatalf("wiped %+v %v", got, err)
	}
}
