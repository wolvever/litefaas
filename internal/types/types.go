package types

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	KindFunction = "function"
	KindBackend  = "backend"
	KindFrontend = "frontend"
)

const (
	RuntimeGo         = "go"
	RuntimeJava       = "java"
	RuntimePython     = "python"
	RuntimeDockerfile = "dockerfile"
	RuntimeStatic     = "static"
)

const (
	RevisionRegistered = "registered"
)

const (
	DefaultPort   = 8080
	DefaultMemory = 128
	DefaultHealth = "/healthz"
)

var nameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// Resource is a registered function, backend, or frontend.
type Resource struct {
	Name      string     `json:"name"`
	Kind      string     `json:"kind"`
	Runtime   string     `json:"runtime"`
	Image     string     `json:"image,omitempty"`
	Port      int        `json:"port,omitempty"`
	Memory    int        `json:"memory,omitempty"`
	Timeout   string     `json:"timeout,omitempty"`
	Health    string     `json:"health,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Revisions []Revision `json:"revisions,omitempty"`
}

// Revision is a recorded deploy metadata snapshot (no runner in Phase 1).
type Revision struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// HealthResponse is returned by GET /healthz.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// VersionResponse is returned by GET /version.
type VersionResponse struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ErrorResponse is a JSON error body.
type ErrorResponse struct {
	Error string `json:"error"`
}

// FunctionList is GET /v1/functions.
type FunctionList struct {
	Items []Resource `json:"items"`
}

// DeployRequest is POST /v1/functions/{name}/deploy.
type DeployRequest struct {
	Image string `json:"image"`
}

// DeployResponse is the stub deploy result.
type DeployResponse struct {
	Name     string   `json:"name"`
	Revision Revision `json:"revision"`
}

// NormalizeResource applies defaults and validates a create/register payload.
func NormalizeResource(r *Resource) error {
	if r == nil {
		return fmt.Errorf("resource is required")
	}
	r.Name = strings.TrimSpace(strings.ToLower(r.Name))
	r.Kind = strings.TrimSpace(strings.ToLower(r.Kind))
	r.Runtime = strings.TrimSpace(strings.ToLower(r.Runtime))
	r.Image = strings.TrimSpace(r.Image)
	r.Timeout = strings.TrimSpace(r.Timeout)
	r.Health = strings.TrimSpace(r.Health)

	if r.Kind == "" {
		r.Kind = KindFunction
	}
	if r.Port == 0 {
		r.Port = DefaultPort
	}
	if r.Memory == 0 {
		r.Memory = DefaultMemory
	}
	if r.Health == "" {
		r.Health = DefaultHealth
	}
	return ValidateResource(*r)
}

// ValidateResource checks RFC naming and allowed enums.
func ValidateResource(r Resource) error {
	if err := ValidateName(r.Name); err != nil {
		return err
	}
	switch r.Kind {
	case KindFunction, KindBackend, KindFrontend:
	default:
		return fmt.Errorf("kind must be function, backend, or frontend")
	}
	switch r.Runtime {
	case RuntimeGo, RuntimeJava, RuntimePython, RuntimeDockerfile, RuntimeStatic:
	default:
		return fmt.Errorf("runtime must be go, java, python, dockerfile, or static")
	}
	if r.Port < 1 || r.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if r.Memory < 0 {
		return fmt.Errorf("memory must be >= 0")
	}
	return nil
}

// ValidateName enforces a DNS-label style resource name.
func ValidateName(name string) error {
	if !nameRE.MatchString(name) {
		return fmt.Errorf("name must be a lowercase DNS label (e.g. hello-fn)")
	}
	return nil
}
