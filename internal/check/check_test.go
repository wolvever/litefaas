package check

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wolvever/litefaas/internal/dockercli"
	"github.com/wolvever/litefaas/internal/manifest"
	"github.com/wolvever/litefaas/internal/scaffold"
	"github.com/wolvever/litefaas/internal/stackpack"
	"github.com/wolvever/litefaas/internal/types"
)

func TestResolveHealth(t *testing.T) {
	pack := &stackpack.Pack{Verify: stackpack.Verify{Health: "ready"}}
	if got := resolveHealth(nil, pack); got != "/ready" {
		t.Fatalf("pack verify: %q", got)
	}
	m := &manifest.Manifest{Health: "/custom"}
	if got := resolveHealth(m, pack); got != "/custom" {
		t.Fatalf("manifest should win over pack: %q", got)
	}
	if got := resolveHealth(nil, nil); got != "/healthz" {
		t.Fatalf("default: %q", got)
	}
}

func TestSanitizeName(t *testing.T) {
	if sanitizeName("hello_world") != "hello_world" {
		t.Fatal(sanitizeName("hello_world"))
	}
	if sanitizeName("a/b") != "a-b" {
		t.Fatal(sanitizeName("a/b"))
	}
	if sanitizeName("") != "app" {
		t.Fatal(sanitizeName(""))
	}
}

func TestParseDockerPort(t *testing.T) {
	got, err := parseDockerPort("8080/tcp -> 127.0.0.1:32768")
	if err != nil || got != "32768" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestRunHostCreatesOutAndEnv(t *testing.T) {
	dir := t.TempDir()
	orig := HostExec
	t.Cleanup(func() { HostExec = orig })
	var sawDir string
	var sawEnv []string
	var sawArgv []string
	HostExec = func(_ context.Context, d string, env []string, _, _ io.Writer, name string, args ...string) error {
		sawDir = d
		sawEnv = append([]string{}, env...)
		sawArgv = append([]string{name}, args...)
		return nil
	}
	opts := Options{Stdout: io.Discard, Stderr: io.Discard}
	if err := runHost(context.Background(), opts, dir, []string{"go", "test", "./..."}); err != nil {
		t.Fatal(err)
	}
	if sawDir != dir {
		t.Fatalf("dir=%q", sawDir)
	}
	if !reflectEqual(sawArgv, []string{"go", "test", "./..."}) {
		t.Fatalf("argv=%v", sawArgv)
	}
	outDir := filepath.Join(dir, ".litefaas", "out")
	if st, err := os.Stat(outDir); err != nil || !st.IsDir() {
		t.Fatalf("out dir: %v", err)
	}
	found := false
	for _, e := range sawEnv {
		if strings.HasPrefix(e, "LITEFAAS_OUT=") && strings.Contains(e, outDir) {
			found = true
		}
	}
	if !found {
		t.Fatalf("env=%v", sawEnv)
	}
}

func reflectEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mockDockerOK(t *testing.T, hostPort string) {
	t.Helper()
	origOut, origExec := dockercli.Output, dockercli.Exec
	t.Cleanup(func() {
		dockercli.Output = origOut
		dockercli.Exec = origExec
	})
	dockercli.Output = func(_ context.Context, _ string, args ...string) (string, error) {
		switch args[0] {
		case "version":
			return "27.0.0", nil
		case "rm":
			return "", nil
		case "run":
			return "cid", nil
		case "port":
			return "8080/tcp -> 127.0.0.1:" + hostPort, nil
		default:
			t.Fatalf("unexpected Output %v", args)
		}
		return "", nil
	}
	dockercli.Exec = func(_ context.Context, _, _ io.Writer, _ string, args ...string) error {
		if args[0] != "build" {
			t.Fatalf("unexpected Exec %v", args)
		}
		return nil
	}
}

func TestRunSkipHostAndSmoke(t *testing.T) {
	parent := t.TempDir()
	dest, err := scaffold.Init(scaffold.Options{Name: "hello", Runtime: types.RuntimeGo, Dir: parent})
	if err != nil {
		t.Fatal(err)
	}
	mockDockerOK(t, "9") // smoke skipped; port unused
	origHost := HostExec
	t.Cleanup(func() { HostExec = origHost })
	HostExec = func(context.Context, string, []string, io.Writer, io.Writer, string, ...string) error {
		t.Fatal("host should be skipped")
		return nil
	}
	var stdout, stderr bytes.Buffer
	res, err := Run(context.Background(), Options{
		Dir:       dest,
		SkipHost:  true,
		SkipSmoke: true,
		Stdout:    &stdout,
		Stderr:    &stderr,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || len(res.Stages) != 3 {
		t.Fatalf("%+v", res)
	}
	if !res.Stages[0].Skipped || !res.Stages[2].Skipped {
		t.Fatalf("stages=%+v", res.Stages)
	}
	if !strings.Contains(stdout.String(), "check passed") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestRunHostImageSmokeJSON(t *testing.T) {
	parent := t.TempDir()
	dest, err := scaffold.Init(scaffold.Options{Name: "hello", Runtime: types.RuntimeGo, Dir: parent})
	if err != nil {
		t.Fatal(err)
	}
	// Force a pack with host.build via overlay
	overlay := filepath.Join(t.TempDir(), "packs", "go-test-pack")
	if err := os.MkdirAll(overlay, 0o755); err != nil {
		t.Fatal(err)
	}
	yml := "id: go-test-pack\ntitle: t\nlanguage: go\nruntime: go\nkind: backend\nmatch:\n  - files: [go.mod]\nhost:\n  build: [\"true\"]\nverify:\n  health: /healthz\n"
	if err := os.WriteFile(filepath.Join(overlay, "stack.yml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(overlay, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LITEFAAS_STACKS_DIR", filepath.Dir(overlay))

	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(200)
	}))
	t.Cleanup(hs.Close)
	hostPort := strings.TrimPrefix(hs.URL, "http://127.0.0.1:")

	mockDockerOK(t, hostPort)
	origHost := HostExec
	t.Cleanup(func() { HostExec = origHost })
	var hostCalled bool
	HostExec = func(_ context.Context, dir string, env []string, _, _ io.Writer, name string, args ...string) error {
		hostCalled = true
		if name != "true" {
			t.Fatalf("host cmd %s %v", name, args)
		}
		return nil
	}

	var stdout, stderr bytes.Buffer
	res, err := Run(context.Background(), Options{
		Dir:     dest,
		StackID: "go-test-pack",
		JSON:    true,
		Stdout:  &stdout,
		Stderr:  &stderr,
		HTTP:    hs.Client(),
	})
	if err != nil {
		t.Fatalf("err=%v stderr=%s", err, stderr.String())
	}
	if !hostCalled {
		t.Fatal("expected host")
	}
	if !res.OK {
		t.Fatalf("%+v", res)
	}
	var parsed Result
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout.String())
	}
	if !parsed.OK || len(parsed.Stages) != 3 {
		t.Fatalf("%+v", parsed)
	}
	for _, s := range parsed.Stages {
		if !s.OK || s.Skipped {
			t.Fatalf("stage %+v", s)
		}
	}
}

func TestRunHostFailureAborts(t *testing.T) {
	parent := t.TempDir()
	dest, err := scaffold.Init(scaffold.Options{Name: "hello", Runtime: types.RuntimeGo, Dir: parent})
	if err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(t.TempDir(), "packs", "go-fail-pack")
	if err := os.MkdirAll(overlay, 0o755); err != nil {
		t.Fatal(err)
	}
	yml := "id: go-fail-pack\ntitle: t\nlanguage: go\nruntime: go\nkind: backend\nmatch:\n  - files: [go.mod]\nhost:\n  build: [\"false\"]\n"
	if err := os.WriteFile(filepath.Join(overlay, "stack.yml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(overlay, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LITEFAAS_STACKS_DIR", filepath.Dir(overlay))

	origHost := HostExec
	t.Cleanup(func() { HostExec = origHost })
	HostExec = func(context.Context, string, []string, io.Writer, io.Writer, string, ...string) error {
		return &strErr{"host boom"}
	}
	// docker should never be called
	origOut, origExec := dockercli.Output, dockercli.Exec
	t.Cleanup(func() { dockercli.Output = origOut; dockercli.Exec = origExec })
	dockercli.Output = func(context.Context, string, ...string) (string, error) {
		t.Fatal("docker Output")
		return "", nil
	}
	dockercli.Exec = func(context.Context, io.Writer, io.Writer, string, ...string) error {
		t.Fatal("docker Exec")
		return nil
	}

	_, err = Run(context.Background(), Options{
		Dir:     dest,
		StackID: "go-fail-pack",
		Stdout:  io.Discard,
		Stderr:  io.Discard,
	})
	if err == nil || !strings.Contains(err.Error(), "host") {
		t.Fatalf("err=%v", err)
	}
}

func TestSmokeCleansUp(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	t.Cleanup(hs.Close)
	hostPort := strings.TrimPrefix(hs.URL, "http://127.0.0.1:")
	origOut := dockercli.Output
	t.Cleanup(func() { dockercli.Output = origOut })
	var rms []string
	dockercli.Output = func(_ context.Context, _ string, args ...string) (string, error) {
		switch args[0] {
		case "version":
			return "27", nil
		case "rm":
			rms = append(rms, args[len(args)-1])
			return "", nil
		case "run":
			return "id", nil
		case "port":
			return "8080/tcp -> 127.0.0.1:" + hostPort, nil
		default:
			t.Fatalf("%v", args)
		}
		return "", nil
	}
	if err := Smoke(context.Background(), SmokeOpts{
		Image:  "hello:latest",
		Name:   "hello",
		Health: "/healthz",
		Port:   8080,
		HTTP:   hs.Client(),
	}); err != nil {
		t.Fatal(err)
	}
	if len(rms) < 2 {
		t.Fatalf("expected pre+defer rm, got %v", rms)
	}
}

func TestSmokeHealthFail(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
	}))
	t.Cleanup(hs.Close)
	hostPort := strings.TrimPrefix(hs.URL, "http://127.0.0.1:")
	origOut := dockercli.Output
	t.Cleanup(func() { dockercli.Output = origOut })
	dockercli.Output = func(_ context.Context, _ string, args ...string) (string, error) {
		switch args[0] {
		case "version", "rm":
			return "", nil
		case "run":
			return "id", nil
		case "port":
			return "8080/tcp -> 127.0.0.1:" + hostPort, nil
		default:
			t.Fatalf("%v", args)
		}
		return "", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	err := Smoke(ctx, SmokeOpts{
		Image: "x", Name: "x", Health: "/healthz",
		HTTP: &http.Client{Timeout: 100 * time.Millisecond},
	})
	if err == nil {
		t.Fatal("expected fail")
	}
}

type strErr struct{ s string }

func (e *strErr) Error() string { return e.s }

func TestGoGinGormHasHostBuild(t *testing.T) {
	cat, err := stackpack.OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	p, err := cat.Get("go-gin-gorm")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Host.Build) < 2 || p.Host.Build[0] != "go" {
		t.Fatalf("host.build=%v", p.Host.Build)
	}
	if p.Verify.Health != "/healthz" {
		t.Fatalf("verify.health=%q", p.Verify.Health)
	}
}

func TestMultiServiceSequentialAbort(t *testing.T) {
	dir := t.TempDir()
	web := filepath.Join(dir, "web")
	if err := os.MkdirAll(web, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	api := filepath.Join(dir, "api")
	if err := os.MkdirAll(api, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(api, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw := "services:\n  web:\n    kind: frontend\n    runtime: static\n    handler: ./web\n    image: web:latest\n    health: /\n  api:\n    kind: backend\n    runtime: go\n    handler: ./api\n    image: api:latest\n"
	if err := os.WriteFile(filepath.Join(dir, "stack.yaml"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	t.Cleanup(hs.Close)
	hostPort := strings.TrimPrefix(hs.URL, "http://127.0.0.1:")

	origOut, origExec := dockercli.Output, dockercli.Exec
	t.Cleanup(func() { dockercli.Output = origOut; dockercli.Exec = origExec })
	var builds []string
	dockercli.Output = func(_ context.Context, _ string, args ...string) (string, error) {
		switch args[0] {
		case "version", "rm":
			return "", nil
		case "run":
			return "id", nil
		case "port":
			return "8080/tcp -> 127.0.0.1:" + hostPort, nil
		default:
			t.Fatalf("%v", args)
		}
		return "", nil
	}
	dockercli.Exec = func(_ context.Context, _, _ io.Writer, _ string, args ...string) error {
		if args[0] == "build" {
			builds = append(builds, args[len(args)-1])
			// fail on second service
			if len(builds) >= 2 {
				return &strErr{"build fail"}
			}
		}
		return nil
	}

	_, err := Run(context.Background(), Options{
		Dir:       dir,
		SkipHost:  true,
		Stdout:    io.Discard,
		Stderr:    io.Discard,
		HTTP:      hs.Client(),
	})
	if err == nil {
		t.Fatal("expected image failure")
	}
	if len(builds) != 2 {
		t.Fatalf("builds=%v (expected abort after 2nd)", builds)
	}
}
