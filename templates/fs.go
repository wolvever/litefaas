// Package templates embeds runtime, preset, frontend, meta, and stack-pack scaffolds (RFC-0001 §8).
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

// Stacks holds fingerprint packs under templates/stacks/ (Phase 7).
// Data only — litefaasd does not import this package.
//
//go:embed all:stacks
var Stacks embed.FS
