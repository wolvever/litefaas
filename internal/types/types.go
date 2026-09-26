// Package types holds RFC-0001 resource names shared by API, store, and CLI.
package types

import (
	"fmt"
	"time"
)

// Kind is a litefaas.yaml kind.
type Kind string

const (
	KindFunction Kind = "function"
	KindBackend  Kind = "backend"
	KindFrontend Kind = "frontend"
)

func ParseKind(s string) (Kind, error) {
	switch Kind(s) {
	case KindFunction, KindBackend, KindFrontend:
		return Kind(s), nil
	case "":
		return KindFunction, nil
	default:
		return "", fmt.Errorf("kind must be function|backend|frontend, got %q", s)
	}
}

// AlwaysOn reports whether the kind keeps at least one replica running
// (RFC-0001 §5.2). Functions are invoke-oriented and may scale to zero.
func (k Kind) AlwaysOn() bool {
	switch k {
	case KindBackend, KindFrontend:
		return true
	default:
		return false
	}
}

// DefaultReplicas is 1 for always-on kinds and 0 for functions (scale-to-zero allowed).
func DefaultReplicas(k Kind) int {
	if k.AlwaysOn() {
		return 1
	}
	return 0
}

// NormalizeReplicas applies always-on mins. Backends/frontends cannot be 0.
func NormalizeReplicas(k Kind, n int) (int, error) {
	if n < 0 {
		return 0, fmt.Errorf("replicas must be >= 0")
	}
	if k.AlwaysOn() && n < 1 {
		return 1, nil
	}
	return n, nil
}

// Runtime is a litefaas.yaml runtime.
type Runtime string

const (
	RuntimeGo         Runtime = "go"
	RuntimeJava       Runtime = "java"
	RuntimePython     Runtime = "python"
	RuntimeDockerfile Runtime = "dockerfile"
	RuntimeStatic     Runtime = "static"
)

func ParseRuntime(s string) (Runtime, error) {
	switch Runtime(s) {
	case RuntimeGo, RuntimeJava, RuntimePython, RuntimeDockerfile, RuntimeStatic:
		return Runtime(s), nil
	default:
		return "", fmt.Errorf("runtime must be go|java|python|dockerfile|static, got %q", s)
	}
}

// Trigger is an HTTP path binding from a manifest.
type Trigger struct {
	Type        string `json:"type,omitempty" yaml:"type,omitempty"`
	Path        string `json:"path,omitempty" yaml:"path,omitempty"`
	StripPrefix bool   `json:"strip_prefix,omitempty" yaml:"strip_prefix,omitempty"`
	SPA         bool   `json:"spa,omitempty" yaml:"spa,omitempty"`
}

// Build is an optional pre-image build step.
type Build struct {
	Command []string `json:"command,omitempty" yaml:"command,omitempty"`
	Output  string   `json:"output,omitempty" yaml:"output,omitempty"`
}

// Resource is control-plane metadata for a function, backend, or frontend.
type Resource struct {
	Name      string            `json:"name"`
	Kind      Kind              `json:"kind"`
	Runtime   Runtime           `json:"runtime"`
	Preset    string            `json:"preset,omitempty"`
	Handler   string            `json:"handler,omitempty"`
	Image     string            `json:"image,omitempty"`
	Port      int               `json:"port,omitempty"`
	Memory    int               `json:"memory,omitempty"`
	Timeout   string            `json:"timeout,omitempty"`
	Health    string            `json:"health,omitempty"`
	Replicas  int               `json:"replicas,omitempty"`
	Triggers  []Trigger         `json:"triggers,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Build     *Build            `json:"build,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Revision is a recorded deploy of an image (runner may still be a stub).
type Revision struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// ResourceView is a resource plus its deploy revisions.
type ResourceView struct {
	Resource
	Revisions []Revision `json:"revisions,omitempty"`
}
