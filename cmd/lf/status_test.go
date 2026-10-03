package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/wolvever/litefaas/internal/api"
	"github.com/wolvever/litefaas/internal/runner"
	"github.com/wolvever/litefaas/internal/store"
	"github.com/wolvever/litefaas/internal/types"
)

func TestCurrentProdRevisionSkipsDraftAndFailed(t *testing.T) {
	id, ok := currentProdRevision([]types.Revision{
		{ID: 1, Status: "deployed", Target: "prod"},
		{ID: 2, Status: "deployed", Target: "draft"},
		{ID: 3, Status: "failed", Target: "prod"},
	})
	if !ok || id != 1 {
		t.Fatalf("id=%d ok=%v", id, ok)
	}
	if _, ok := currentProdRevision(nil); ok {
		t.Fatal("expected none")
	}
}

func TestCmdStatusJSON(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fake := runner.NewFake()
	fake.DraftEndpoints["hello"] = "http://127.0.0.1:9"
	srv := api.New(api.Options{Store: st, Token: "s", Runner: fake})
	if _, err := st.Create(types.Resource{
		Name: "hello", Kind: types.KindFunction, Runtime: types.RuntimeGo, Image: "hello:latest",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddRevisionFull(types.Revision{Name: "hello", Image: "hello:latest", Status: "deployed", Target: "prod"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddRevisionFull(types.Revision{Name: "hello", Image: "hello:draft", Status: "deployed", Target: "draft"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Create(types.Resource{
		Name: "web", Kind: types.KindFrontend, Runtime: types.RuntimeStatic, Image: "web:latest",
		Triggers: []types.Trigger{{Path: "/"}},
	}); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	err = cmdStatus([]string{"--json", "--gateway", ts.URL, "--token", "s", "--config-dir", t.TempDir()})
	_ = w.Close()
	os.Stdout = old
	raw, _ := io.ReadAll(r)
	if err != nil {
		t.Fatalf("%v\n%s", err, raw)
	}
	if len(fake.DraftDeploys) != 0 || len(fake.Deploys) != 0 {
		t.Fatalf("status started containers draft=%d prod=%d", len(fake.DraftDeploys), len(fake.Deploys))
	}
	var rows []statusRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatalf("json: %v\n%s", err, raw)
	}
	if len(rows) != 2 {
		t.Fatalf("rows=%+v", rows)
	}
	// List is ordered by name: hello, web.
	if rows[0].Name != "hello" || rows[0].Revision == nil || *rows[0].Revision != 1 || !rows[0].DraftUp {
		t.Fatalf("hello=%+v", rows[0])
	}
	if rows[0].URLKind != "invoke" || rows[0].URL != ts.URL+"/v1/invoke/hello" {
		t.Fatalf("url=%s kind=%s", rows[0].URL, rows[0].URLKind)
	}
	if rows[1].Name != "web" || rows[1].Revision != nil || rows[1].DraftUp {
		t.Fatalf("web=%+v", rows[1])
	}
	if rows[1].URL != ts.URL+"/" {
		t.Fatalf("web url=%s", rows[1].URL)
	}
	if !bytes.Contains(raw, []byte(`"draft_up": false`)) && !bytes.Contains(raw, []byte(`"draft_up":false`)) {
		t.Fatalf("missing draft_up false:\n%s", raw)
	}
}
