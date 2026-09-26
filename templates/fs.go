// Package templates embeds RFC-0001 copy-on-init scaffolds.
package templates

import "embed"

//go:embed all:runtimes all:presets
var FS embed.FS
