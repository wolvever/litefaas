package runner

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wolvever/litefaas/internal/types"
)

func TestDockerDeployAndLookup(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	_, hostPort, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	bin := t.TempDir()
	logf := filepath.Join(t.TempDir(), "docker.log")
	script := "#!/bin/sh\n" +
		"echo \"$0 $@\" >> " + logf + "\n" +
		"if [ \"$1\" = \"version\" ]; then echo 24.0.0; exit 0; fi\n" +
		"if [ \"$1\" = \"rm\" ]; then exit 0; fi\n" +
		"if [ \"$1\" = \"run\" ]; then echo cid123; exit 0; fi\n" +
		"if [ \"$1\" = \"inspect\" ]; then echo \"true hello:latest 8080\"; exit 0; fi\n" +
		"if [ \"$1\" = \"port\" ]; then echo \"127.0.0.1:" + hostPort + "\"; exit 0; fi\n" +
		"exit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	d := Docker{}
	inst, err := d.Deploy(context.Background(), types.Resource{
		Name:   "hello",
		Port:   8080,
		Health: "/healthz",
		Env:    map[string]string{"GREETING": "hi"},
		Memory: 128,
	}, "hello:latest")
	if err != nil {
		t.Fatal(err)
	}
	if inst.Endpoint != "http://127.0.0.1:"+hostPort {
		t.Fatalf("endpoint = %q", inst.Endpoint)
	}

	raw, err := os.ReadFile(logf)
	if err != nil {
		t.Fatal(err)
	}
	log := string(raw)
	if !strings.Contains(log, "run -d --name litefaas-hello") {
		t.Fatalf("missing docker run: %s", log)
	}
	if !strings.Contains(log, "-e GREETING=hi") || !strings.Contains(log, "--memory 128m") {
		t.Fatalf("missing env/memory: %s", log)
	}

	got, err := d.Lookup(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if got.Endpoint != inst.Endpoint {
		t.Fatalf("lookup = %+v", got)
	}
}

func TestParsePublished(t *testing.T) {
	p, err := parsePublished("127.0.0.1:32768")
	if err != nil || p != "32768" {
		t.Fatalf("got %q %v", p, err)
	}
	p, err = parsePublished("0.0.0.0:8081\n")
	if err != nil || p != "8081" {
		t.Fatalf("got %q %v", p, err)
	}
}

func TestEnvKeyOK(t *testing.T) {
	if envKeyOK("PORT") || envKeyOK("bad-key") || envKeyOK("") {
		t.Fatal("expected reject")
	}
	if !envKeyOK("DATABASE_URL") || !envKeyOK("_X") {
		t.Fatal("expected accept")
	}
}

func TestFakeLookupMissing(t *testing.T) {
	f := NewFake()
	if _, err := f.Lookup(context.Background(), "nope"); err == nil {
		t.Fatal("expected error")
	}
	_ = fmt.Sprintf("%v", f)
}
