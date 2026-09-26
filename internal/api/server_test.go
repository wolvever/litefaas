package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wolvever/litefaas/internal/runner"
	"github.com/wolvever/litefaas/internal/store"
	"github.com/wolvever/litefaas/internal/types"
	"github.com/wolvever/litefaas/internal/version"
)

func TestHealthz(t *testing.T) {
	srv := New(Options{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" {
		t.Fatalf("status = %q, want ok", body.Status)
	}
	if body.Version != version.Version {
		t.Fatalf("version = %q, want %q", body.Version, version.Version)
	}
}

func TestVersion(t *testing.T) {
	srv := New(Options{})
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body version.Info
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Version != version.Version {
		t.Fatalf("version = %q, want %q", body.Version, version.Version)
	}
}

func TestResourceCRUDAndAuth(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	fake := runner.NewFake()
	srv := New(Options{Store: st, Token: "secret", Runner: fake})

	body := []byte(`{"name":"web","kind":"frontend","runtime":"static","handler":"./web/dist"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/functions", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth create status = %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/functions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/functions", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	var list []types.Resource
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Kind != types.KindFrontend {
		t.Fatalf("list = %+v", list)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/functions/web/deploy", bytes.NewReader([]byte(`{"image":"localhost:5000/web:0.1.0"}`)))
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("deploy status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(fake.Deploys) != 1 || fake.Deploys[0].Image != "localhost:5000/web:0.1.0" {
		t.Fatalf("deploys = %+v", fake.Deploys)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/functions/web", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/v1/functions/web", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", rec.Code)
	}
	if len(fake.Removed) != 1 || fake.Removed[0] != "web" {
		t.Fatalf("removed = %v", fake.Removed)
	}
}

func TestInvokeFunction(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	fn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/" {
			t.Fatalf("function got %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"hello from litefaas","function":"hello"}`))
	}))
	t.Cleanup(fn.Close)

	fake := runner.NewFake()
	fake.Endpoints["hello"] = fn.URL
	srv := New(Options{Store: st, Runner: fake})

	body := []byte(`{"name":"hello","kind":"function","runtime":"go","image":"hello:latest","timeout":"5s"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/functions", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/invoke/hello", bytes.NewReader([]byte(`{"name":"litefaas"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("invoke status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`hello from litefaas`)) {
		t.Fatalf("body = %s", rec.Body.String())
	}

	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>web</html>"))
	}))
	t.Cleanup(web.Close)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"svc":"api","path":"` + r.URL.Path + `"}`))
	}))
	t.Cleanup(api.Close)
	fake.Endpoints["site"] = web.URL
	fake.Endpoints["svc"] = api.URL
	for _, raw := range []string{
		`{"name":"site","kind":"frontend","runtime":"static","triggers":[{"type":"http","path":"/","spa":true}]}`,
		`{"name":"svc","kind":"backend","runtime":"python","triggers":[{"type":"http","path":"/api","strip_prefix":true}]}`,
	} {
		req = httptest.NewRequest(http.MethodPost, "/v1/functions", bytes.NewReader([]byte(raw)))
		rec = httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create edge resource status = %d body=%s", rec.Code, rec.Body.String())
		}
	}
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("web")) {
		t.Fatalf("edge / = %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`/orders`)) {
		t.Fatalf("edge /api/orders = %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/invoke/missing", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing invoke status = %d", rec.Code)
	}
}

func TestPutRoutesOverride(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"path":"` + r.URL.Path + `"}`))
	}))
	t.Cleanup(backend.Close)

	fake := runner.NewFake()
	fake.Endpoints["orders"] = backend.URL
	srv := New(Options{Store: st, Runner: fake})

	body := []byte(`{"name":"orders","kind":"backend","runtime":"dockerfile","triggers":[{"type":"http","path":"/unused"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/functions", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/v1/routes", bytes.NewReader([]byte(`[{"path":"/orders","name":"orders","strip_prefix":true}]`)))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("put routes status = %d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/routes", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get routes status = %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/orders/x", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"/x"`)) {
		t.Fatalf("edge /orders/x = %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/v1/routes", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("clear routes status = %d", rec.Code)
	}
}
