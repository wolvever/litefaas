package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/wolvever/litefaas/internal/client"
	"github.com/wolvever/litefaas/internal/types"
)

func TestFormatDeploySummaryFunction(t *testing.T) {
	var buf bytes.Buffer
	res := types.Resource{
		Name:   "hello",
		Kind:   types.KindFunction,
		Image:  "hello:latest",
		Health: "/healthz",
		Env: map[string]string{
			"DATABASE_URL": "${secret:db}",
			"PLAIN":        "x",
		},
	}
	dep := client.DeployResult{
		Revision:  types.Revision{ID: 3, Name: "hello", Image: "hello:latest", Status: "ready"},
		Endpoint:  "http://127.0.0.1:32768",
		Container: "litefaas-hello",
	}
	formatDeploySummary(&buf, "http://127.0.0.1:8080", res, dep)
	out := buf.String()
	for _, want := range []string{
		"name:      hello",
		"health:    ok  GET /healthz",
		"invoke:  POST http://127.0.0.1:8080/v1/invoke/hello",
		"DATABASE_URL=${secret:db}",
		"container: litefaas-hello",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "PLAIN") || strings.Contains(out, "=x") {
		t.Fatalf("leaked plaintext:\n%s", out)
	}
}

func TestFormatDeploySummaryTriggers(t *testing.T) {
	var buf bytes.Buffer
	res := types.Resource{
		Name:  "api",
		Kind:  types.KindBackend,
		Image: "api:latest",
		Triggers: []types.Trigger{
			{Path: "/api", StripPrefix: true},
		},
	}
	formatDeploySummary(&buf, "http://127.0.0.1:8080/", res, client.DeployResult{
		Revision: types.Revision{ID: 1, Status: "ready", Image: "api:latest"},
	})
	out := buf.String()
	if !strings.Contains(out, "http://127.0.0.1:8080/api") {
		t.Fatalf("missing edge url:\n%s", out)
	}
	if strings.Contains(out, "invoke:") {
		t.Fatalf("backend should not show invoke:\n%s", out)
	}
}
