package dockercli

import (
	"context"
	"io"
	"testing"
)

func TestAvailableUsesDockerVersion(t *testing.T) {
	orig := Output
	t.Cleanup(func() { Output = orig })
	var got []string
	Output = func(_ context.Context, name string, args ...string) (string, error) {
		got = append([]string{name}, args...)
		return "27.0.0", nil
	}
	if err := Available(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(got) < 2 || got[0] != "docker" || got[1] != "version" {
		t.Fatalf("got %v", got)
	}
}

func TestExecOverride(t *testing.T) {
	orig := Exec
	t.Cleanup(func() { Exec = orig })
	called := false
	Exec = func(_ context.Context, _, _ io.Writer, name string, args ...string) error {
		called = true
		if name != "echo" || args[0] != "hi" {
			t.Fatalf("%s %v", name, args)
		}
		return nil
	}
	if err := Exec(context.Background(), io.Discard, io.Discard, "echo", "hi"); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("override not called")
	}
}
