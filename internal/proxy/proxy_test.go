package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
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

func TestStripPrefix(t *testing.T) {
	cases := []struct {
		path, prefix, want string
	}{
		{"/api", "/api", "/"},
		{"/api/", "/api", "/"},
		{"/api/orders", "/api", "/orders"},
		{"/api/orders/1", "/api", "/orders/1"},
		{"/healthz", "/api", "/healthz"},
		{"/", "/", "/"},
	}
	for _, tc := range cases {
		got := StripPrefix(tc.path, tc.prefix)
		if got != tc.want {
			t.Fatalf("StripPrefix(%q, %q) = %q want %q", tc.path, tc.prefix, got, tc.want)
		}
	}
}

func TestDirectorStripPrefix(t *testing.T) {
	fn := Director(Route{Path: "/api", Endpoint: "http://127.0.0.1:9", StripPrefix: true})
	req := httptest.NewRequest(http.MethodGet, "http://gateway/api/orders", nil)
	fn(req)
	if req.URL.Path != "/orders" || req.URL.Host != "127.0.0.1:9" {
		t.Fatalf("director path/host = %s %s", req.URL.Path, req.URL.Host)
	}
	req = httptest.NewRequest(http.MethodGet, "http://gateway/api", nil)
	fn(req)
	if req.URL.Path != "/" {
		t.Fatalf("director /api => %s", req.URL.Path)
	}
	keep := Director(Route{Path: "/api", Endpoint: "http://127.0.0.1:9", StripPrefix: false})
	req = httptest.NewRequest(http.MethodGet, "http://gateway/api/orders", nil)
	keep(req)
	if req.URL.Path != "/api/orders" {
		t.Fatalf("no strip => %s", req.URL.Path)
	}
}
