package client

import (
	"bytes"
	"context"
	"net/http/httptest"
	"testing"

	"github.com/wolvever/litefaas/internal/api"
	"github.com/wolvever/litefaas/internal/proxy"
	"github.com/wolvever/litefaas/internal/runner"
	"github.com/wolvever/litefaas/internal/store"
	"github.com/wolvever/litefaas/internal/types"
)

func TestClientCRUDDeploy(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fake := runner.NewFake()
	hs := httptest.NewServer(api.New(api.Options{Store: st, Token: "s", Runner: fake}))
	t.Cleanup(hs.Close)

	c := New(hs.URL, "s")
	res, err := c.Create(types.Resource{Name: "hello", Kind: types.KindFunction, Runtime: types.RuntimeGo, Image: "hello:latest"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Name != "hello" {
		t.Fatalf("create = %+v", res)
	}
	if _, err := c.Create(res); !IsConflict(err) {
		t.Fatalf("dup = %v", err)
	}
	res.Memory = 64
	if _, err := c.Update(res); err != nil {
		t.Fatal(err)
	}
	rev, err := c.Deploy("hello", "hello:latest")
	if err != nil {
		t.Fatal(err)
	}
	if rev.Status != "deployed" {
		t.Fatalf("rev = %+v", rev)
	}
	list, err := c.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v %v", list, err)
	}
	if err := c.Delete("hello"); err != nil {
		t.Fatal(err)
	}

	if _, err := c.PutRoutes([]proxy.Route{{Path: "/orders", Name: "orders", StripPrefix: true}}); err != nil {
		t.Fatal(err)
	}
	rts, err := c.Routes()
	if err != nil || len(rts) != 1 || rts[0].Path != "/orders" {
		t.Fatalf("routes = %+v err=%v", rts, err)
	}
	if err := c.ClearRoutes(); err != nil {
		t.Fatal(err)
	}

	fake.LogText["hello"] = "logline\n"
	fake.Endpoints["hello"] = "http://127.0.0.1:9"
	if _, err := c.Create(types.Resource{Name: "hello", Kind: types.KindFunction, Runtime: types.RuntimeGo, Image: "hello:latest"}); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := c.Logs(context.Background(), "hello", 10, false, &buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "logline\n" {
		t.Fatalf("logs = %q", buf.String())
	}
	snap, err := c.Metrics()
	if err != nil || snap.Resources < 1 {
		t.Fatalf("metrics = %+v err=%v", snap, err)
	}
}

func TestIsConflict(t *testing.T) {
	if !IsConflict(errString("POST /v1/functions: {\"error\":\"resource already exists\"}\n")) {
		t.Fatal("expected conflict")
	}
	if IsConflict(nil) {
		t.Fatal("nil")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
