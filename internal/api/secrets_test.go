package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wolvever/litefaas/internal/runner"
	"github.com/wolvever/litefaas/internal/secret"
	"github.com/wolvever/litefaas/internal/store"
	"github.com/wolvever/litefaas/internal/types"
)

func TestSecretsCRUD(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	sec, err := secret.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	srv := New(Options{Store: st, Secrets: sec, Token: "tok", Runner: runner.NewFake()})

	put := httptest.NewRequest(http.MethodPut, "/v1/secrets/db", bytes.NewReader([]byte(`{"value":"s3cr3t-value"}`)))
	put.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, put)
	if rec.Code != http.StatusOK {
		t.Fatalf("put status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "s3cr3t-value") {
		t.Fatal("put response leaked value")
	}

	get := httptest.NewRequest(http.MethodGet, "/v1/secrets/db", nil)
	get.Header.Set("Authorization", "Bearer tok")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, get)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status=%d", rec.Code)
	}
	var got secretValueResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Value != "s3cr3t-value" {
		t.Fatalf("value = %q", got.Value)
	}

	list := httptest.NewRequest(http.MethodGet, "/v1/secrets", nil)
	list.Header.Set("Authorization", "Bearer tok")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, list)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "s3cr3t") {
		t.Fatalf("list leaked or bad status: %d %s", rec.Code, rec.Body.String())
	}

	del := httptest.NewRequest(http.MethodDelete, "/v1/secrets/db", nil)
	del.Header.Set("Authorization", "Bearer tok")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, del)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d", rec.Code)
	}
}

func TestDeployResolvesSecrets(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	sec, err := secret.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = sec.Set("db", "resolved-db-url")
	fake := runner.NewFake()
	srv := New(Options{Store: st, Secrets: sec, Token: "tok", Runner: fake})

	res := types.Resource{
		Name: "api", Kind: types.KindBackend, Runtime: types.RuntimePython,
		Image: "api:latest", Port: 8080, Memory: 128, Health: "/healthz",
		Env: map[string]string{"DATABASE_URL": "${secret:db}"},
	}
	raw, _ := json.Marshal(res)
	req := httptest.NewRequest(http.MethodPost, "/v1/functions", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", rec.Code, rec.Body.String())
	}

	dep := httptest.NewRequest(http.MethodPost, "/v1/functions/api/deploy", bytes.NewReader([]byte(`{}`)))
	dep.Header.Set("Authorization", "Bearer tok")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, dep)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("deploy=%d %s", rec.Code, rec.Body.String())
	}
	if len(fake.Deploys) != 1 {
		t.Fatalf("deploys = %d", len(fake.Deploys))
	}
	got := fake.Deploys[0].Env
	if got["DATABASE_URL"] != "resolved-db-url" {
		t.Fatalf("env = %#v", got)
	}
	// store still has ref
	stored, _ := st.Get("api")
	if stored.Env["DATABASE_URL"] != "${secret:db}" {
		t.Fatalf("stored env mutated: %#v", stored.Env)
	}
}


func TestSecretsNotFoundAndBadName(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	sec, err := secret.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	srv := New(Options{Store: st, Secrets: sec, Token: "tok", Runner: runner.NewFake()})

	get := httptest.NewRequest(http.MethodGet, "/v1/secrets/missing", nil)
	get.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, get)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get missing status=%d", rec.Code)
	}

	put := httptest.NewRequest(http.MethodPut, "/v1/secrets/-bad", bytes.NewReader([]byte(`{"value":"x"}`)))
	put.Header.Set("Authorization", "Bearer tok")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, put)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad name status=%d body=%s", rec.Code, rec.Body.String())
	}

	put = httptest.NewRequest(http.MethodPut, "/v1/secrets/db", bytes.NewReader([]byte(`{"value":""}`)))
	put.Header.Set("Authorization", "Bearer tok")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, put)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty value status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDeployMissingSecret(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	sec, err := secret.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	fake := runner.NewFake()
	srv := New(Options{Store: st, Secrets: sec, Token: "tok", Runner: fake})

	res := types.Resource{
		Name: "api", Kind: types.KindBackend, Runtime: types.RuntimePython,
		Image: "api:latest", Port: 8080, Memory: 128, Health: "/healthz",
		Env: map[string]string{"DATABASE_URL": "${secret:missing}"},
	}
	raw, _ := json.Marshal(res)
	req := httptest.NewRequest(http.MethodPost, "/v1/functions", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", rec.Code, rec.Body.String())
	}

	dep := httptest.NewRequest(http.MethodPost, "/v1/functions/api/deploy", bytes.NewReader([]byte(`{}`)))
	dep.Header.Set("Authorization", "Bearer tok")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, dep)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("deploy=%d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "missing") {
		t.Fatalf("body=%s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "resolved") {
		t.Fatal("unexpected plaintext")
	}
	if len(fake.Deploys) != 0 {
		t.Fatalf("deploy should not run, got %d", len(fake.Deploys))
	}
}

func TestSecretsEnvBags(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	sec, err := secret.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := New(Options{Store: st, Secrets: sec, Token: "tok", Runner: runner.NewFake()})
	put := func(env, name, val string) {
		t.Helper()
		body := []byte(`{"value":"` + val + `"}`)
		req := httptest.NewRequest(http.MethodPut, "/v1/secrets/"+name+"?env="+env, bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer tok")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("put %s/%s = %d %s", env, name, rec.Code, rec.Body.String())
		}
	}
	put("dev", "DB", "devdb")
	put("prod", "DB", "proddb")
	req := httptest.NewRequest(http.MethodGet, "/v1/secrets/DB?env=dev", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte(`"devdb"`)) {
		t.Fatalf("get dev = %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/v1/secrets/DB?env=prod", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte(`"proddb"`)) {
		t.Fatalf("get prod = %d %s", rec.Code, rec.Body.String())
	}
}

func TestDeployInjectEnv(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	sec, err := secret.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = sec.SetEnv("prod", "DB", "injected-db")
	_ = sec.SetEnv("prod", "TOKEN", "injected-tok")
	fake := runner.NewFake()
	srv := New(Options{Store: st, Secrets: sec, Token: "tok", Runner: fake})

	res := types.Resource{
		Name: "api", Kind: types.KindBackend, Runtime: types.RuntimePython,
		Image: "api:latest", Port: 8080, Memory: 128, Health: "/healthz",
		Env: map[string]string{"TOKEN": "from-manifest"},
	}
	raw, _ := json.Marshal(res)
	req := httptest.NewRequest(http.MethodPost, "/v1/functions", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", rec.Code, rec.Body.String())
	}

	dep := httptest.NewRequest(http.MethodPost, "/v1/functions/api/deploy?env=prod&inject_env=1", bytes.NewReader([]byte(`{}`)))
	dep.Header.Set("Authorization", "Bearer tok")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, dep)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("deploy=%d %s", rec.Code, rec.Body.String())
	}
	got := fake.Deploys[0].Env
	if got["TOKEN"] != "from-manifest" {
		t.Fatalf("manifest should win: %#v", got)
	}
	if got["DB"] != "injected-db" {
		t.Fatalf("bag inject: %#v", got)
	}
	if strings.Contains(rec.Body.String(), "injected-") {
		t.Fatalf("response leaked secret: %s", rec.Body.String())
	}
}
