package runner

import (
	"context"
	"io"

	"github.com/wolvever/litefaas/internal/types"
)

// Fake is an in-memory runner for tests (no Docker).
type Fake struct {
	Endpoints map[string]string
	Prefer    map[string]string
	LogsText  map[string]string
	Deploys   []types.Resource
	Removed   []string
	LogCalls  []string
}

func NewFake() *Fake {
	return &Fake{Endpoints: map[string]string{}, Prefer: map[string]string{}, LogsText: map[string]string{}}
}

func (f *Fake) Deploy(_ context.Context, res types.Resource) (Result, error) {
	f.Deploys = append(f.Deploys, res)
	ep := f.Endpoints[res.Name]
	if ep == "" {
		ep = f.Prefer[res.Name]
	}
	if ep == "" {
		ep = "http://127.0.0.1:9"
	}
	f.Endpoints[res.Name] = ep
	return Result{Container: ContainerName(res.Name), Endpoint: ep}, nil
}

func (f *Fake) Remove(_ context.Context, name string) error {
	f.Removed = append(f.Removed, name)
	delete(f.Endpoints, name)
	return nil
}

func (f *Fake) Endpoint(_ context.Context, name string) (string, error) {
	ep, ok := f.Endpoints[name]
	if !ok || ep == "" {
		return "", ErrNotDeployed
	}
	return ep, nil
}

func (f *Fake) Logs(_ context.Context, name string, _ LogsOptions, w io.Writer) error {
	f.LogCalls = append(f.LogCalls, name)
	text, ok := f.LogsText[name]
	if !ok {
		if _, deployed := f.Endpoints[name]; !deployed {
			return ErrNotDeployed
		}
		return nil
	}
	if w != nil {
		_, _ = io.WriteString(w, text)
	}
	return nil
}
