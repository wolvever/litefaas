package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestStacksListsPacks(t *testing.T) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	err = run([]string{"stacks"})
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	out := buf.String()
	for _, id := range []string{"python-fastapi", "go-gin-gorm", "java-spring-mybatis"} {
		if !strings.Contains(out, id) {
			t.Fatalf("missing %s in %q", id, out)
		}
	}
}
