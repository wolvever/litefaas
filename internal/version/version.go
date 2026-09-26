package version

// Name is the project / daemon identifier.
const Name = "litefaas"

// Version is the release string. Override at build time with:
//
//	-ldflags "-X github.com/wolvever/litefaas/internal/version.Version=..."
var Version = "0.1.0-dev"
