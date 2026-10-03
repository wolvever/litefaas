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
