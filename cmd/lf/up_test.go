package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wolvever/litefaas/internal/config"
	"github.com/wolvever/litefaas/internal/token"
)

func TestProbeGatewayNeedStart(t *testing.T) {
	client := &http.Client{Timeout: 200 * time.Millisecond}
	pr, _, err := probeGateway(client, "http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	if pr != probeNeedStart {
		t.Fatalf("got %v", pr)
	}
}

func TestProbeGatewayReuse(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","version":"dev"}`))
	}))
	t.Cleanup(hs.Close)
	pr, hp, err := probeGateway(hs.Client(), hs.URL)
	if err != nil || pr != probeReuse || hp.Version != "dev" {
		t.Fatalf("pr=%v hp=%+v err=%v", pr, hp, err)
	}
}

func TestProbeGatewayForeign(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"hello":"world"}`))
	}))
	t.Cleanup(hs.Close)
	pr, _, err := probeGateway(hs.Client(), hs.URL)
	if pr != probeForeign || err == nil {
		t.Fatalf("pr=%v err=%v", pr, err)
	}
}

func TestResolveLitefaasdOrder(t *testing.T) {
	dir := t.TempDir()
	sib := filepath.Join(dir, "litefaasd")
	if err := os.WriteFile(sib, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	lfExe := filepath.Join(dir, "lf")

	got, err := resolveLitefaasd("/flag/litefaasd", "", lfExe, func(string) (string, error) {
		t.Fatal("should not look PATH")
		return "", nil
	})
	if err != nil || got != "/flag/litefaasd" {
		t.Fatalf("flag: %s %v", got, err)
	}

	got, err = resolveLitefaasd("", "/env/litefaasd", lfExe, func(string) (string, error) {
		t.Fatal("should not look PATH")
		return "", nil
	})
	if err != nil || got != "/env/litefaasd" {
		t.Fatalf("env: %s %v", got, err)
	}

	got, err = resolveLitefaasd("", "", lfExe, func(string) (string, error) {
		t.Fatal("should not look PATH when sibling exists")
		return "", nil
	})
	if err != nil || got != sib {
		t.Fatalf("sibling: %s %v", got, err)
	}

	got, err = resolveLitefaasd("", "", filepath.Join(t.TempDir(), "lf"), func(name string) (string, error) {
		if name != "litefaasd" {
			t.Fatalf("name=%s", name)
		}
		return "/path/litefaasd", nil
	})
	if err != nil || got != "/path/litefaasd" {
		t.Fatalf("path: %s %v", got, err)
	}
}

func TestEnsureAuthAndContext(t *testing.T) {
	data := t.TempDir()
	cfg := t.TempDir()
	opts := upOptions{
		DataDir:   data,
		ConfigDir: cfg,
		Context:   "default",
		NoAuth:    false,
	}
	if err := ensureAuthAndContext(opts, "http://127.0.0.1:8080"); err != nil {
		t.Fatal(err)
	}
	tok, err := token.ReadFile(token.Path(data))
	if err != nil || tok == "" {
		t.Fatalf("token = %q err=%v", tok, err)
	}
	cfgTok, err := token.ReadFile(token.Path(cfg))
	if err != nil || cfgTok != tok {
		t.Fatalf("cfg token = %q want %q", cfgTok, tok)
	}
	f, err := config.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if f.Current != "default" || f.Contexts["default"].Gateway != "http://127.0.0.1:8080" || f.Contexts["default"].Token != tok {
		t.Fatalf("cfg = %+v", f)
	}
}

func TestEnsureAuthNoAuth(t *testing.T) {
	data := t.TempDir()
	cfg := t.TempDir()
	opts := upOptions{DataDir: data, ConfigDir: cfg, Context: "demo", NoAuth: true}
	if err := ensureAuthAndContext(opts, "http://127.0.0.1:8080"); err != nil {
		t.Fatal(err)
	}
	f, err := config.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if f.Contexts["demo"].Token != "" {
		t.Fatalf("token should be empty: %+v", f.Contexts["demo"])
	}
	args := daemonArgs(opts)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--no-auth") {
		t.Fatalf("args = %v", args)
	}
}

func TestClearStalePidfile(t *testing.T) {
	dir := t.TempDir()
	if err := writePidfile(dir, 999999); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 200 * time.Millisecond}
	if err := clearStalePidfile(dir, client, "http://127.0.0.1:1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, pidFileName)); !os.IsNotExist(err) {
		t.Fatalf("pidfile should be removed: %v", err)
	}
}

func TestGatewayURL(t *testing.T) {
	if g := gatewayURL("127.0.0.1:8080", "", ""); g != "http://127.0.0.1:8080" {
		t.Fatal(g)
	}
	if g := gatewayURL("127.0.0.1:8443", "", "cert.pem"); g != "https://127.0.0.1:8443" {
		t.Fatal(g)
	}
	if g := gatewayURL("127.0.0.1:8080", "http://custom:9/", ""); g != "http://custom:9" {
		t.Fatal(g)
	}
}
