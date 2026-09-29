package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestApplyEdgeRulesRedirect(t *testing.T) {
	rules := []EdgeRule{{From: "/old", To: "/new", Status: 301}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://gw/old", nil)
	handled, _ := ApplyEdgeRules(rec, req, rules)
	if !handled || rec.Code != 301 {
		t.Fatalf("code=%d handled=%v", rec.Code, handled)
	}
	if loc := rec.Header().Get("Location"); loc != "/new" {
		t.Fatalf("Location=%q", loc)
	}
}

func TestApplyEdgeRulesRewrite(t *testing.T) {
	rules := []EdgeRule{{From: "/app/*", To: "/index.html", Status: 200}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://gw/app/x", nil)
	handled, rewritten := ApplyEdgeRules(rec, req, rules)
	if handled || !rewritten {
		t.Fatalf("handled=%v rewritten=%v", handled, rewritten)
	}
	if req.URL.Path != "/index.html" {
		t.Fatalf("path=%q", req.URL.Path)
	}
}

func TestMergeResponseHeaders(t *testing.T) {
	rules := []EdgeRule{{From: "/*", Status: 0, Headers: map[string]string{"X-Frame-Options": "DENY"}}}
	h := make(http.Header)
	MergeResponseHeaders(h, "/any", rules)
	if h.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("%v", h)
	}
}

func TestParseDraftPath(t *testing.T) {
	name, ok := ParseDraftPath("/--draft/hello")
	if !ok || name != "hello" {
		t.Fatalf("%q %v", name, ok)
	}
	name, ok = ParseDraftPath("/--draft/hello/x/y")
	if !ok || name != "hello" {
		t.Fatalf("%q %v", name, ok)
	}
	if _, ok := ParseDraftPath("/hello"); ok {
		t.Fatal("expected false")
	}
	if _, ok := ParseDraftPath("/--draft/"); ok {
		t.Fatal("expected false")
	}
}

func TestDraftURL(t *testing.T) {
	u := DraftURL("http://127.0.0.1:8080/", "app")
	if u != "http://127.0.0.1:8080/--draft/app/" {
		t.Fatalf("%s", u)
	}
}

func TestDraftStripProxy(t *testing.T) {
	var gotPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(backend.Close)
	route := Route{Path: DraftPrefix("hello"), Name: "hello", Endpoint: backend.URL, StripPrefix: true}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://gw/--draft/hello/api", nil)
	Handler(route).ServeHTTP(rec, req)
	if gotPath != "/api" {
		t.Fatalf("backend path=%q", gotPath)
	}
}
