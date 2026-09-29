package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wolvever/litefaas/internal/proxy"
	"github.com/wolvever/litefaas/internal/runner"
	"github.com/wolvever/litefaas/internal/store"
	"github.com/wolvever/litefaas/internal/types"
)

func TestEdgeRulesRedirect(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fake := runner.NewFake()
	srv := New(Options{Store: st, Token: "s", Runner: fake})

	body, _ := json.Marshal([]proxy.EdgeRule{{From: "/old", To: "/new", Status: 301}})
	req := httptest.NewRequest(http.MethodPut, "/v1/edge-rules", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer s")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("put=%d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/old", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != 301 || rec.Header().Get("Location") != "/new" {
		t.Fatalf("redirect code=%d loc=%q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestEdgeRulesRewriteThenProxy(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fake := runner.NewFake()
	srv := New(Options{Store: st, Token: "s", Runner: fake})

	res := types.Resource{
		Name:     "web",
		Kind:     types.KindFrontend,
		Runtime:  types.RuntimeStatic,
		Image:    "web:latest",
		Triggers: []types.Trigger{{Path: "/", SPA: true}},
	}
	if _, err := st.Create(res); err != nil {
		t.Fatal(err)
	}
	fake.Endpoints["web"] = "http://127.0.0.1:9"

	body, _ := json.Marshal([]proxy.EdgeRule{{From: "/app/*", To: "/", Status: 200}})
	req := httptest.NewRequest(http.MethodPut, "/v1/edge-rules", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer s")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("put=%d %s", rec.Code, rec.Body.String())
	}

	// Rewrite to / should match frontend route; fake endpoint will fail dial — still proves match.
	req = httptest.NewRequest(http.MethodGet, "/app/x", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	// ReverseProxy to dead endpoint → 502/error; important is we did not 404 via mux.
	if rec.Code == 404 {
		t.Fatalf("unexpected 404: %s", rec.Body.String())
	}
}
