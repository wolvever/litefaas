package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/wolvever/litefaas/internal/api"
	"github.com/wolvever/litefaas/internal/client"
	"github.com/wolvever/litefaas/internal/runner"
	"github.com/wolvever/litefaas/internal/store"
)

func TestApplyProjectEdgeRulesMerges(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	srv := api.New(api.Options{Store: st, Token: "s", Runner: runner.NewFake()})
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	c := client.New(ts.URL, "s")

	dirA := t.TempDir()
	if err := os.WriteFile(filepath.Join(dirA, "_redirects"), []byte("/from-a /to-a 301\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirB := t.TempDir()
	if err := os.WriteFile(filepath.Join(dirB, "_redirects"), []byte("/from-b /to-b 302\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyProjectEdgeRules(c, dirA, []string{"alpha"}); err != nil {
		t.Fatal(err)
	}
	if err := applyProjectEdgeRules(c, dirB, []string{"beta"}); err != nil {
		t.Fatal(err)
	}
	// Empty tree does not clear alpha.
	if err := applyProjectEdgeRules(c, t.TempDir(), []string{"alpha"}); err != nil {
		t.Fatal(err)
	}
	rules, err := c.EdgeRules()
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Fatalf("rules=%+v", rules)
	}
	if rules[0].Project != "alpha" || rules[0].From != "/from-a" || rules[1].Project != "beta" || rules[1].From != "/from-b" {
		t.Fatalf("rules=%+v", rules)
	}
}
