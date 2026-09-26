package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wolvever/litefaas/internal/store"
	"github.com/wolvever/litefaas/internal/types"
	"github.com/wolvever/litefaas/internal/version"
)

func TestHealthzAndVersionNoAuth(t *testing.T) {
	h := newTestServer(t, "secret")
	for _, path := range []string{"/healthz", "/version"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status %d body %s", path, rec.Code, rec.Body.String())
		}
	}
	var health types.HealthResponse
	if err := json.Unmarshal(hget(t, h, "/healthz", ""), &health); err != nil {
		t.Fatal(err)
	}
	if health.Status != "ok" || health.Version != version.Version {
		t.Fatalf("health: %+v", health)
	}
}

func TestFunctionsRequireBearer(t *testing.T) {
	h := newTestServer(t, "secret")
	req := httptest.NewRequest(http.MethodGet, "/v1/functions", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/functions", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestFunctionCRUD(t *testing.T) {
	h := newTestServer(t, "tok")
	body := `{"name":"orders","kind":"function","runtime":"go","image":"orders:1"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/functions", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/functions/orders", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d %s", rec.Code, rec.Body.String())
	}
	var got types.Resource
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "orders" || got.Runtime != "go" || len(got.Revisions) != 1 {
		t.Fatalf("got %+v", got)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/functions/orders/deploy", bytes.NewBufferString(`{"image":"orders:2"}`))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("deploy %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/v1/functions/orders", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/functions/orders", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete %d", rec.Code)
	}
}

func newTestServer(t *testing.T, token string) http.Handler {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return New(st, token).Handler()
}

func hget(t *testing.T, h http.Handler, path, token string) []byte {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s -> %d %s", path, rec.Code, rec.Body.String())
	}
	return rec.Body.Bytes()
}
