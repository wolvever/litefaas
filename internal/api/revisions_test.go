package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/wolvever/litefaas/internal/runner"
	"github.com/wolvever/litefaas/internal/store"
	"github.com/wolvever/litefaas/internal/types"
)

func TestDeploySnapshotRollbackAndPin(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fake := runner.NewFake()
	srv := New(Options{Store: st, Token: "tok", Runner: fake})
	t.Cleanup(srv.Close)

	body := []byte(`{"name":"api","kind":"backend","runtime":"go","image":"api:1","env":{"PLAIN":"not-a-secret"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/functions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", rec.Code, rec.Body.String())
	}

	deploy := func(image, snap string) deployResponse {
		t.Helper()
		raw := `{"image":"` + image + `","snapshot":` + snap + `}`
		req := httptest.NewRequest(http.MethodPost, "/v1/functions/api/deploy", strings.NewReader(raw))
		req.Header.Set("Authorization", "Bearer tok")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("deploy %s = %d %s", image, rec.Code, rec.Body.String())
		}
		var out deployResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	first := deploy("api:1", `{"pack_id":"go-chi","env_refs":[{"key":"DATABASE_URL","ref":"${secret:db}"},{"key":"PLAIN","ref":"super-secret"}]}`)
	if first.ImageID != "sha256:fake" {
		t.Fatalf("image id = %q", first.ImageID)
	}
	if first.Snapshot.EnvRefs == nil || first.Snapshot.EnvRefs[1].Ref != "" {
		t.Fatalf("plaintext stored in revision: %+v", first.Snapshot.EnvRefs)
	}
	second := deploy("api:2", `{"pack_id":"go-chi"}`)
	if second.Image != "api:2" {
		t.Fatalf("second=%+v", second)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/functions/api/rollback", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer tok")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("rollback=%d %s", rec.Code, rec.Body.String())
	}
	var rolled deployResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &rolled); err != nil {
		t.Fatal(err)
	}
	if rolled.Status != "rolled_back" || rolled.Image != "api:1" {
		t.Fatalf("rolled=%+v", rolled)
	}
	if fake.Deploys[len(fake.Deploys)-1].Image != "api:1" {
		t.Fatalf("runner image=%s", fake.Deploys[len(fake.Deploys)-1].Image)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/functions/api/revisions/"+strconv.FormatInt(first.ID, 10)+"/pin", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("pin=%d %s", rec.Code, rec.Body.String())
	}
	var pinned types.Revision
	if err := json.Unmarshal(rec.Body.Bytes(), &pinned); err != nil {
		t.Fatal(err)
	}
	if !pinned.Pinned || pinned.Snapshot.PackID != "go-chi" {
		t.Fatalf("pinned=%+v", pinned)
	}
}

func TestRollbackSkipsReleaseCommands(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fake := runner.NewFake()
	srv := New(Options{Store: st, Token: "tok", Runner: fake})
	t.Cleanup(srv.Close)

	body := []byte(`{"name":"api","kind":"backend","runtime":"go","image":"api:1","release":["echo migrate"]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/functions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", rec.Code, rec.Body.String())
	}
	for _, image := range []string{"api:1", "api:2"} {
		req = httptest.NewRequest(http.MethodPost, "/v1/functions/api/deploy", strings.NewReader(`{"image":"`+image+`"}`))
		req.Header.Set("Authorization", "Bearer tok")
		rec = httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("deploy %s = %d %s", image, rec.Code, rec.Body.String())
		}
	}
	if len(fake.Deploys) < 2 || len(fake.Deploys[1].Release) != 1 || fake.Deploys[1].Release[0] != "echo migrate" {
		t.Fatalf("deploy release = %+v", fake.Deploys)
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/functions/api/rollback", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer tok")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("rollback=%d %s", rec.Code, rec.Body.String())
	}
	last := fake.Deploys[len(fake.Deploys)-1]
	if last.Image != "api:1" || len(last.Release) != 0 {
		t.Fatalf("rollback ran release or wrong image: %+v", last)
	}
}

func TestDraftDeployDoesNotReplaceProd(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fake := runner.NewFake()
	srv := New(Options{Store: st, Token: "tok", Runner: fake})
	t.Cleanup(srv.Close)

	body := []byte(`{"name":"api","kind":"backend","runtime":"go","image":"api:prod","release":["echo migrate"]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/functions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", rec.Code, rec.Body.String())
	}
	raw := `{"image":"api:draft","env":{"MODE":"draft"},"release":["echo migrate"]}`
	req = httptest.NewRequest(http.MethodPost, "/v1/functions/api/deploy?draft=1", strings.NewReader(raw))
	req.Header.Set("Authorization", "Bearer tok")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("draft=%d %s", rec.Code, rec.Body.String())
	}
	if len(fake.Deploys) != 0 {
		t.Fatalf("prod deploy called: %+v", fake.Deploys)
	}
	if len(fake.DraftDeploys) != 1 || fake.DraftDeploys[0].Image != "api:draft" || len(fake.DraftDeploys[0].Release) != 0 {
		t.Fatalf("draft deploy=%+v", fake.DraftDeploys)
	}
	if fake.DraftDeploys[0].Env["MODE"] != "draft" {
		t.Fatalf("env=%v", fake.DraftDeploys[0].Env)
	}
	got, err := st.Get("api")
	if err != nil {
		t.Fatal(err)
	}
	if got.Image != "api:prod" {
		t.Fatalf("prod image overwritten: %s", got.Image)
	}
	revs, err := st.ListRevisions("api")
	if err != nil {
		t.Fatal(err)
	}
	if len(revs) != 1 || revs[0].Target != "draft" {
		t.Fatalf("revs=%+v", revs)
	}
}

func TestDraftEdgeMissesWhenUndeployed(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fake := runner.NewFake()
	fake.Endpoints["hello"] = "http://127.0.0.1:9"
	srv := New(Options{Store: st, Runner: fake})
	t.Cleanup(srv.Close)
	if _, err := st.Create(types.Resource{Name: "hello", Kind: types.KindFunction, Runtime: types.RuntimeGo, Image: "hello:latest"}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/--draft/hello/ping", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "draft not deployed") {
		t.Fatalf("status=%d %s", rec.Code, rec.Body.String())
	}
}
