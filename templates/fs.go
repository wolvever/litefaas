// Package templates embeds runtime and preset scaffolds (RFC-0001 §8).
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
