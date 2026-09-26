// Package version holds build-time identity for lf and litefaasd.
package version

// Overridable via -ldflags, e.g.
//
//	-X github.com/wolvever/litefaas/internal/version.Version=v0.1.0-alpha
var (
	Version = "dev"
	Commit  = "unknown"
)

// Info is the JSON shape of GET /version.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

func Get() Info {
	return Info{Version: Version, Commit: Commit}
}
