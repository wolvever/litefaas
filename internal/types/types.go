// Package types holds RFC-0001 resource names shared by API, store, and CLI.
package types

import (
	"fmt"
	"regexp"
	"strings"
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
	RuntimeNode       Runtime = "node"
	RuntimeDockerfile Runtime = "dockerfile"
	RuntimeStatic     Runtime = "static"
)

func ParseRuntime(s string) (Runtime, error) {
	switch Runtime(s) {
	case RuntimeGo, RuntimeJava, RuntimePython, RuntimeNode, RuntimeDockerfile, RuntimeStatic:
		return Runtime(s), nil
	default:
		return "", fmt.Errorf("runtime must be go|java|python|node|dockerfile|static, got %q", s)
	}
}

// DefaultKind is the init default when --kind is omitted (RFC-0001 §8 / §15).
func DefaultKind(rt Runtime) Kind {
	switch rt {
	case RuntimeStatic:
		return KindFrontend
	case RuntimeDockerfile, RuntimeNode:
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
	Name     string            `json:"name"`
	Kind     Kind              `json:"kind"`
	Runtime  Runtime           `json:"runtime"`
	Preset   string            `json:"preset,omitempty"`
	Handler  string            `json:"handler,omitempty"`
	Image    string            `json:"image,omitempty"`
	Port     int               `json:"port,omitempty"`
	Memory   int               `json:"memory,omitempty"`
	Timeout  string            `json:"timeout,omitempty"`
	Health   string            `json:"health,omitempty"`
	Triggers []Trigger         `json:"triggers,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	Volumes  []VolumeMount     `json:"volumes,omitempty"`
	Build    *Build            `json:"build,omitempty"`
	// Release is an optional list of idempotent commands run in the candidate
	// image before cutover. Rollback clears this so they are not repeated.
	Release   []string  `json:"release,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// VolumeMount is a named Docker volume mounted into the container.
type VolumeMount struct {
	Name     string `json:"name" yaml:"name"`
	Mount    string `json:"mount" yaml:"mount"`
	ReadOnly bool   `json:"read_only,omitempty" yaml:"read_only,omitempty"`
}

// Revision is a recorded deploy of an image (runner may still be a stub).
// Snapshot holds pack/plan and env refs only — never secret values.
type Revision struct {
	ID        int64            `json:"id"`
	Name      string           `json:"name"`
	Image     string           `json:"image"`
	ImageID   string           `json:"image_id,omitempty"`
	Status    string           `json:"status"`
	Pinned    bool             `json:"pinned,omitempty"`
	Snapshot  RevisionSnapshot `json:"snapshot,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
}

// SnapshotEnvRef is an env key plus, when applicable, the unresolved ${secret:…}
// expression. Plaintext values are never stored.
type SnapshotEnvRef struct {
	Key string `json:"key"`
	Ref string `json:"ref,omitempty"`
}

// RevisionSnapshot is non-secret metadata captured at deploy time.
type RevisionSnapshot struct {
	PackID  string           `json:"pack_id,omitempty"`
	Runtime string           `json:"runtime,omitempty"`
	Kind    string           `json:"kind,omitempty"`
	Port    int              `json:"port,omitempty"`
	Health  string           `json:"health,omitempty"`
	Handler string           `json:"handler,omitempty"`
	Memory  int              `json:"memory,omitempty"`
	Image   string           `json:"image,omitempty"`
	EnvRefs []SnapshotEnvRef `json:"env_refs,omitempty"`
	Volumes []VolumeMount    `json:"volumes,omitempty"`
}

// ResourceView is a resource plus its deploy revisions.
type ResourceView struct {
	Resource
	Revisions []Revision `json:"revisions,omitempty"`
}

var volumeNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// ValidateVolumeMount checks name/mount for named Docker volumes.
func ValidateVolumeMount(v VolumeMount) error {
	if !volumeNameRE.MatchString(v.Name) {
		return fmt.Errorf("volume name %q invalid (want [a-z0-9][a-z0-9_-]{0,31})", v.Name)
	}
	if !strings.HasPrefix(v.Mount, "/") || strings.Contains(v.Mount, "..") {
		return fmt.Errorf("volume mount %q must be an absolute path without ..", v.Mount)
	}
	return nil
}

// DockerVolumeName is litefaas-<resource>-<volume>.
func DockerVolumeName(resource, volume string) string {
	return "litefaas-" + resource + "-" + volume
}
