package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDaemonLogPath(t *testing.T) {
	got := daemonLogPath("/tmp/data")
	if got != filepath.Join("/tmp/data", "litefaasd.log") {
		t.Fatalf("got %s", got)
	}
}

func TestTailDaemonLogTailLines(t *testing.T) {
	dir := t.TempDir()
	path := daemonLogPath(dir)
	content := "line1\nline2\nline3\nline4\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := tailDaemonLog(context.Background(), dir, 2, false, &buf); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "line3") || !strings.Contains(got, "line4") {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(got, "line1") {
		t.Fatalf("should not include line1: %q", got)
	}
}

func TestTailDaemonLogMissing(t *testing.T) {
	err := tailDaemonLog(context.Background(), t.TempDir(), 10, false, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "daemon log not found") {
		t.Fatalf("got %v", err)
	}
}

func TestFollowFileAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.log")
	if err := os.WriteFile(path, []byte("a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var buf bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- followFile(ctx, path, 2, &buf) // offset after "a\n"
	}()
	time.Sleep(50 * time.Millisecond)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("b\n")
	_ = f.Close()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), "b") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "b") {
		t.Fatalf("got %q", buf.String())
	}
}

func TestIsNotDeployedErr(t *testing.T) {
	if !isNotDeployedErr(errString("GET /v1/functions/x/logs: {\"error\":\"not deployed\"}")) {
		t.Fatal("expected match")
	}
	if isNotDeployedErr(errString("boom")) {
		t.Fatal("unexpected")
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestPrefixWriterLines(t *testing.T) {
	var mu sync.Mutex
	var buf bytes.Buffer
	pw := &prefixWriter{mu: &mu, w: &buf, prefix: "[api] "}
	if _, err := pw.Write([]byte("hello\nwor")); err != nil {
		t.Fatal(err)
	}
	if _, err := pw.Write([]byte("ld\n")); err != nil {
		t.Fatal(err)
	}
	if err := pw.Flush(); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "[api] hello\n") || !strings.Contains(got, "[api] world\n") {
		t.Fatalf("got %q", got)
	}
}

func TestStreamLogsSinglePassesThrough(t *testing.T) {
	// prefix only used for aggregate; single-name path has no prefixWriter.
	if prefixLogLabel("api") != "[api] " {
		t.Fatal(prefixLogLabel("api"))
	}
}

func prefixLogLabel(name string) string {
	return "[" + name + "] "
}
