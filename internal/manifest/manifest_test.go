package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wolvever/litefaas/internal/types"
)

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte("name: hello\nruntime: go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, root, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if root != dir {
		t.Fatalf("root = %q", root)
	}
	if m.Kind != string(types.KindFunction) {
		t.Fatalf("kind = %q", m.Kind)
	}
	if m.Image != "hello:latest" || m.Port != 8080 || m.Memory != 128 || m.Timeout != "30s" || m.Health != "/healthz" {
		t.Fatalf("defaults = %+v", m)
	}
	res := m.Resource()
	if res.Name != "hello" || res.Runtime != types.RuntimeGo {
		t.Fatalf("resource = %+v", res)
	}
}

func TestLoadStripPrefixTrigger(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	raw := "name: orders\nkind: backend\nruntime: dockerfile\ntriggers:\n  - type: http\n    path: /orders\n    strip_prefix: true\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Triggers) != 1 || m.Triggers[0].Path != "/orders" || !m.Triggers[0].StripPrefix {
		t.Fatalf("triggers = %+v", m.Triggers)
	}
}

func TestLoadRejectsBadTimeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte("name: hello\nruntime: go\ntimeout: nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestParseTimeout(t *testing.T) {
	d, err := ParseTimeout("45s")
	if err != nil || d.Seconds() != 45 {
		t.Fatalf("got %v %v", d, err)
	}
	if _, err := ParseTimeout("0s"); err == nil {
		t.Fatal("expected error for 0s")
	}
	if _, err := ParseTimeout("10m"); err == nil {
		t.Fatal("expected error for timeout over max")
	}
}

func TestLoadRejectsLowMemory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte("name: hello\nruntime: go\nmemory: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected memory error")
	}
}

func TestLoadStackMapAndList(t *testing.T) {
	dir := t.TempDir()
	raw := "services:\n  web:\n    kind: frontend\n    runtime: static\n    handler: ./web\n  api:\n    kind: backend\n    runtime: python\n    handler: ./api\n    triggers:\n      - type: http\n        path: /api\n        strip_prefix: true\n"
	path := filepath.Join(dir, StackFileName)
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := LoadStack(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Services) != 2 || st.Services[0].Name != "web" || st.Services[1].Name != "api" {
		t.Fatalf("services = %+v", st.Services)
	}
	if st.Services[0].Kind != string(types.KindFrontend) || st.Services[1].Triggers[0].Path != "/api" {
		t.Fatalf("normalized = %+v", st.Services)
	}
	if ServiceDir(dir, &st.Services[0]) != filepath.Join(dir, "web") {
		t.Fatalf("service dir = %s", ServiceDir(dir, &st.Services[0]))
	}

	listPath := filepath.Join(dir, "list.yaml")
	if err := os.WriteFile(listPath, []byte("services:\n  - name: only\n    runtime: go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err = LoadStack(listPath)
	if err != nil || len(st.Services) != 1 || st.Services[0].Name != "only" {
		t.Fatalf("list stack = %+v err=%v", st, err)
	}
}

func TestResolvePrefersLitefaasYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("name: hello\nruntime: go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, StackFileName), []byte("services:\n  web:\n    runtime: static\n    kind: frontend\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, m, stack, err := Resolve(dir)
	if err != nil || root != dir || m == nil || m.Name != "hello" || stack != nil {
		t.Fatalf("resolve = root=%q m=%+v stack=%v err=%v", root, m, stack, err)
	}
}

func TestExamplesStackYAML(t *testing.T) {
	root, m, stack, err := Resolve("../../examples")
	if err != nil || m != nil || stack == nil {
		t.Fatalf("examples stack = root=%q m=%v stack=%v err=%v", root, m, stack, err)
	}
	if len(stack.Services) != 3 {
		t.Fatalf("services = %d", len(stack.Services))
	}
	names := map[string]bool{}
	for _, svc := range stack.Services {
		names[svc.Name] = true
	}
	for _, want := range []string{"web", "api", "orders"} {
		if !names[want] {
			t.Fatalf("missing %s in %+v", want, names)
		}
	}
}

func TestResolveStackDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, StackFileName), []byte("services:\n  web:\n    runtime: static\n    kind: frontend\n    handler: ./web\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, m, stack, err := Resolve(dir)
	if err != nil || m != nil || stack == nil || root != dir || stack.Services[0].Name != "web" {
		t.Fatalf("resolve stack = root=%q m=%v stack=%+v err=%v", root, m, stack, err)
	}
}
