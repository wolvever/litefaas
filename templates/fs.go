// Package templates embeds runtime scaffolds (RFC-0001 §8).
package templates

import "embed"

// Runtimes holds files under templates/runtimes/.
//
//go:embed all:runtimes
var Runtimes embed.FS
