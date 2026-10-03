package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wolvever/litefaas/internal/runner"
	"github.com/wolvever/litefaas/internal/store"
	"github.com/wolvever/litefaas/internal/types"
)

func TestDraftStatusDoesNotStartContainer(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fake := runner.NewFake()
	fake.DraftEndpoints["hello"] = "http://127.0.0.1:9"
	srv := New(Options{Store: st, Token: "s", Runner: fake})
	if _, err := st.Create(types.Resource{Name: "hello", Kind: types.KindFunction, Runtime: types.RuntimeGo, Image: "hello:latest"}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/functions/hello/draft", nil)
	req.Header.Set("Authorization", "Bearer s")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("up=%d %s", rec.Code, rec.Body.String())
	}
	var up draftStatusBody
	if err := json.Unmarshal(rec.Body.Bytes(), &up); err != nil {
		t.Fatal(err)
	}
	if !up.Up || up.Container != "litefaas-hello-draft" {
		t.Fatalf("%+v", up)
	}
	if len(fake.DraftDeploys) != 0 || len(fake.Deploys) != 0 {
		t.Fatalf("status started work draft=%d prod=%d", len(fake.DraftDeploys), len(fake.Deploys))
	}

	delete(fake.DraftEndpoints, "hello")
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/functions/hello/draft", nil)
	req.Header.Set("Authorization", "Bearer s")
	srv.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("down=%d %s", rec.Code, rec.Body.String())
	}
	var down draftStatusBody
	if err := json.Unmarshal(rec.Body.Bytes(), &down); err != nil {
		t.Fatal(err)
	}
	if down.Up {
		t.Fatalf("expected down: %+v", down)
	}
}
