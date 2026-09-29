package main

import (
	"flag"
	"io"
	"strings"
	"testing"
)

func TestCheckFlagParse(t *testing.T) {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	stackID := fs.String("stack", "", "")
	skipHost := fs.Bool("skip-host", false, "")
	skipSmoke := fs.Bool("skip-smoke", false, "")
	asJSON := fs.Bool("json", false, "")
	rest, err := parseMixed(fs, []string{".", "--stack", "go-gin-gorm", "--skip-host", "--skip-smoke", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rest) != 1 || rest[0] != "." {
		t.Fatalf("rest=%v", rest)
	}
	if *stackID != "go-gin-gorm" || !*skipHost || !*skipSmoke || !*asJSON {
		t.Fatalf("stack=%q skipHost=%v skipSmoke=%v json=%v", *stackID, *skipHost, *skipSmoke, *asJSON)
	}
}

func TestUpCheckIsBoolFlag(t *testing.T) {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	check := fs.Bool("check", false, "")
	addr := fs.String("addr", "127.0.0.1:8080", "")
	rest, err := parseMixed(fs, []string{"--check", "--addr", "127.0.0.1:9090"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rest) != 0 {
		t.Fatalf("rest=%v", rest)
	}
	if !*check {
		t.Fatal("expected --check")
	}
	if *addr != "127.0.0.1:9090" {
		t.Fatalf("addr=%q (bool flag stole value?)", *addr)
	}
}

func TestUsageMentionsCheck(t *testing.T) {
	var b strings.Builder
	printUsage(&b)
	s := b.String()
	if !strings.Contains(s, "lf check") {
		t.Fatal("usage missing lf check")
	}
	if !strings.Contains(s, "--check") {
		t.Fatal("usage missing up --check hint")
	}
}
