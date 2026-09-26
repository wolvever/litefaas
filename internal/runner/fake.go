package runner

import (
	"context"

	"github.com/wolvever/litefaas/internal/types"
)

// Fake is an in-memory runner for tests (no Docker).
type Fake struct {
	Endpoints map[string]string
	Deploys   []types.Resource
	Removed   []string
}

func NewFake() *Fake {
	return &Fake{Endpoints: map[string]string{}}
}

func (f *Fake) Deploy(_ context.Context, res types.Resource) (Result, error) {
	f.Deploys = append(f.Deploys, res)
	ep := f.Endpoints[res.Name]
	if ep == "" {
		ep = "http://127.0.0.1:9"
		f.Endpoints[res.Name] = ep
	}
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
