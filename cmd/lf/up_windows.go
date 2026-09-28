//go:build windows

package main

import "os/exec"

func setDetachedProc(cmd *exec.Cmd) {
	// Best-effort: no Setsid on Windows for v0.1.
}
