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
	Name      string            `json:"name" yaml:"name"`
	Kind      Kind              `json:"kind" yaml:"kind"`
	Runtime   Runtime           `json:"runtime" yaml:"runtime"`
	Preset    string            `json:"preset,omitempty" yaml:"preset,omitempty"`
	Handler   string            `json:"handler,omitempty" yaml:"handler,omitempty"`
	Image     string            `json:"image,omitempty" yaml:"image,omitempty"`
	Port      int               `json:"port,omitempty" yaml:"port,omitempty"`
	Memory    int               `json:"memory,omitempty" yaml:"memory,omitempty"`
	Timeout   string            `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Health    string            `json:"health,omitempty" yaml:"health,omitempty"`
	Triggers  []Trigger         `json:"triggers,omitempty" yaml:"triggers,omitempty"`
	Env       map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	Build     *Build            `json:"build,omitempty" yaml:"build,omitempty"`
	CreatedAt time.Time         `json:"created_at,omitempty" yaml:"-"`
	UpdatedAt time.Time         `json:"updated_at,omitempty" yaml:"-"`
}

func (r Resource) TimeoutDuration() time.Duration {
	if r.Timeout == "" {
		return 60 * time.Second
	}
	d, err := time.ParseDuration(r.Timeout)
	if err != nil || d <= 0 {
		return 60 * time.Second
	}
	return d
}

func (r Resource) ContainerPort() int {
	if r.Port <= 0 {
		return 8080
	}
	return r.Port
}

func (r Resource) HealthPath() string {
	if r.Health == "" {
		return "/healthz"
	}
	return r.Health
}

func (r Resource) ContainerName() string {
	return "litefaas-" + r.Name
}

func DefaultImage(name string) string {
	return "litefaas/" + name + ":latest"
}

// Instance is a running container for a resource.
type Instance struct {
	Name        string `json:"name"`
	ContainerID string `json:"container_id,omitempty"`
	Endpoint    string `json:"endpoint,omitempty"`
	Image       string `json:"image,omitempty"`
	Status      string `json:"status,omitempty"`
}

// Revision is a recorded deploy of an image.
type Revision struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// ResourceView is a resource plus its deploy revisions and live instance.
type ResourceView struct {
	Resource
	Revisions []Revision `json:"revisions,omitempty"`
	Instance  *Instance  `json:"instance,omitempty"`
}

// DeployRequest is POST /v1/functions/{name}/deploy.
type DeployRequest struct {
	Image string `json:"image,omitempty"`
}

// InvokeResult is the sync invoke response.
type InvokeResult struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    []byte            `json:"-"`
}
