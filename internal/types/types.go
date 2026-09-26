// Package types holds RFC-0001 resource names shared by API, store, and CLI.
package types

import (
	"fmt"
	"time"
)

const (
	DefaultMemoryMiB = 128
	MinMemoryMiB     = 16
	MaxMemoryMiB     = 8192
	DefaultTimeout   = 30 * time.Second
	MaxTimeout       = 5 * time.Minute
	DefaultIdleTTL   = 5 * time.Minute
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

// DefaultKind is the init default when --kind is omitted (RFC-0001 §8 / §15).
func DefaultKind(rt Runtime) Kind {
	switch rt {
	case RuntimeStatic:
		return KindFrontend
	case RuntimeDockerfile:
		return KindBackend
	default:
		return KindFunction
	}
}

// AlwaysOn is true for backends and frontends (no scale-to-zero).
func AlwaysOn(k Kind) bool {
	return k == KindBackend || k == KindFrontend
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
