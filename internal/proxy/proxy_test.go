package proxy

import "testing"

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
