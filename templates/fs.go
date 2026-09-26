// Package templates embeds runtime, preset, frontend, and meta scaffolds (RFC-0001 §8).
package templates

import "embed"

// Runtimes holds files under templates/runtimes/.
//
//go:embed all:runtimes
var Runtimes embed.FS

// Presets holds files under templates/presets/.
//
//go:embed all:presets
var Presets embed.FS

// Frontend holds files under templates/frontend/.
//
//go:embed all:frontend
var Frontend embed.FS

// Meta holds files under templates/meta/ (dockerfile escape hatch).
//
//go:embed all:meta
var Meta embed.FS
