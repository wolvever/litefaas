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
