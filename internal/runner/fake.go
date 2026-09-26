package runner

import (
	"context"
	"fmt"
	"sync"

	"github.com/wolvever/litefaas/internal/types"
)

// Fake is an in-memory runner for API tests (no Docker).
type Fake struct {
	mu        sync.Mutex
	Instances map[string]Instance
}

func NewFake() *Fake {
	return &Fake{Instances: map[string]Instance{}}
}

func (f *Fake) Deploy(_ context.Context, res types.Resource, image string) (Instance, error) {
	if image == "" {
		image = res.Image
	}
	inst := Instance{Name: res.Name, Image: image, Endpoint: "http://127.0.0.1:0"}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Instances == nil {
		f.Instances = map[string]Instance{}
	}
	if existing, ok := f.Instances[res.Name]; ok && existing.Endpoint != "" && existing.Endpoint != "http://127.0.0.1:0" {
		inst.Endpoint = existing.Endpoint
	}
	f.Instances[res.Name] = inst
	return inst, nil
}

func (f *Fake) Stop(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.Instances, name)
	return nil
}

func (f *Fake) Lookup(_ context.Context, name string) (Instance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	inst, ok := f.Instances[name]
	if !ok {
		return Instance{}, fmt.Errorf("function %q is not deployed", name)
	}
	return inst, nil
}

func (f *Fake) SetEndpoint(name, endpoint string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Instances == nil {
		f.Instances = map[string]Instance{}
	}
	inst := f.Instances[name]
	inst.Name = name
	inst.Endpoint = endpoint
	f.Instances[name] = inst
}
