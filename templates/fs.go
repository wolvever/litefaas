// Package templates embeds runtime scaffolds (RFC-0001 §8).
package templates

import "embed"

// GoHTTP is templates/runtimes/go/http (HTTP on $PORT, GET /healthz).
//
//go:embed all:runtimes/go/http
var GoHTTP embed.FS

const GoHTTPDir = "runtimes/go/http"
