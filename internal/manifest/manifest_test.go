package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wolvever/litefaas/internal/types"
)

func TestLoadWrite(t *testing.T) {
	dir := t.TempDir()
	r := types.Resource{
		Name:    "hello",
		Kind:    types.KindFunction,
		Runtime: types.RuntimeGo,
		Image:   "litefaas/hello:latest",
		Port:    8080,
		Timeout: "30s",
		Triggers: []types.Trigger{{
			Type: "http",
			Path: "/fn/hello",
		}},
	}
	if err := Write(dir, r); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "hello" || got.Runtime != types.RuntimeGo || got.Timeout != "30s" {
		t.Fatalf("got %+v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); err != nil {
		t.Fatal(err)
	}
}
