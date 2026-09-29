package main

import (
	"github.com/wolvever/litefaas/internal/proxy"
	"strings"
	"testing"

	"github.com/wolvever/litefaas/internal/types"
)

func TestPrimaryEdgeURLHTTP(t *testing.T) {
	res := types.Resource{
		Name: "api",
		Kind: types.KindBackend,
		Triggers: []types.Trigger{
			{Path: "/api", StripPrefix: true},
			{Path: "/admin"},
		},
	}
	u, kind, err := primaryEdgeURL("http://127.0.0.1:8080/", res)
	if err != nil {
		t.Fatal(err)
	}
	if kind != "http" || u != "http://127.0.0.1:8080/api" {
		t.Fatalf("got %s %s", kind, u)
	}
}

func TestPrimaryEdgeURLInvoke(t *testing.T) {
	res := types.Resource{Name: "hello", Kind: types.KindFunction}
	u, kind, err := primaryEdgeURL("http://gw", res)
	if err != nil {
		t.Fatal(err)
	}
	if kind != "invoke" || u != "http://gw/v1/invoke/hello" {
		t.Fatalf("got %s %s", kind, u)
	}
}

func TestPrimaryEdgeURLBackendNoTrigger(t *testing.T) {
	res := types.Resource{Name: "api", Kind: types.KindBackend}
	_, _, err := primaryEdgeURL("http://gw", res)
	if err == nil || !strings.Contains(err.Error(), "no http trigger") {
		t.Fatalf("got %v", err)
	}
}

func TestEdgeHTTPURLsSkipsNonHTTP(t *testing.T) {
	res := types.Resource{
		Name: "x",
		Triggers: []types.Trigger{
			{Type: "cron", Path: "/nope"},
			{Type: "http", Path: "api"},
		},
	}
	urls := edgeHTTPURLs("http://gw", res)
	if len(urls) != 1 || urls[0] != "http://gw/api" {
		t.Fatalf("%v", urls)
	}
}

func TestDraftURLKind(t *testing.T) {
	u := proxy.DraftURL("http://gw", "svc")
	if u != "http://gw/--draft/svc/" {
		t.Fatalf("%s", u)
	}
}
