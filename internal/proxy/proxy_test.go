package proxy

import (
	"net/http"
	"testing"

	"github.com/wolvever/litefaas/internal/types"
)

func TestMatchLongestPrefix(t *testing.T) {
	routes := []Route{
		{Path: "/", Name: "web"},
		{Path: "/api", Name: "api"},
		{Path: "/fn/hello", Name: "hello"},
	}
	cases := map[string]string{
		"/":           "web",
		"/index.html": "web",
		"/api":        "api",
		"/api/":       "api",
		"/api/orders": "api",
		"/fn/hello":   "hello",
		"/fn/hello/x": "hello",
	}
	for path, want := range cases {
		got, ok := Match(path, routes)
		if !ok || got.Name != want {
			t.Fatalf("Match(%q) = %+v ok=%v, want %s", path, got, ok, want)
		}
	}
	if _, ok := Match("/nope", []Route{{Path: "/api", Name: "api"}}); ok {
		t.Fatal("expected miss")
	}
}

func TestDirectorStripPrefix(t *testing.T) {
	dir := Director(Route{Path: "/orders", Endpoint: "http://127.0.0.1:9", StripPrefix: true})
	req, err := http.NewRequest(http.MethodGet, "http://gw/orders/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	dir(req)
	if req.URL.Path != "/x" {
		t.Fatalf("path = %q", req.URL.Path)
	}
	if req.Header.Get("X-Forwarded-Prefix") != "/orders" {
		t.Fatalf("prefix = %q", req.Header.Get("X-Forwarded-Prefix"))
	}
}

func TestFromResourcesIncludesUndeployed(t *testing.T) {
	routes := FromResources([]types.Resource{{
		Name:     "orders",
		Triggers: []types.Trigger{{Type: "http", Path: "/orders", StripPrefix: true}},
	}}, nil)
	if len(routes) != 1 || routes[0].Name != "orders" || routes[0].Endpoint != "" {
		t.Fatalf("routes = %+v", routes)
	}
}
