package config

import (
	"testing"
)

func TestSaveLoadContext(t *testing.T) {
	dir := t.TempDir()
	cfg := empty()
	cfg.Contexts["prod"] = Context{Gateway: "http://vps:8080", Token: "t"}
	cfg.Current = "prod"
	if err := cfg.Save(dir); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	name, ctx, err := got.CurrentContext()
	if err != nil {
		t.Fatal(err)
	}
	if name != "prod" || ctx.Gateway != "http://vps:8080" || ctx.Token != "t" {
		t.Fatalf("got %s %+v", name, ctx)
	}
}
