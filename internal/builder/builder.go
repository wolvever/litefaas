// Package builder runs `docker build` for a service directory.
package builder

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/wolvever/litefaas/internal/manifest"
)

func Build(path string, out io.Writer) (string, error) {
	res, err := manifest.Load(path)
	if err != nil {
		return "", err
	}
	dir := manifest.Dir(path)
	if res.Handler != "" && res.Handler != "." {
		if st, err := os.Stat(filepath.Join(dir, res.Handler)); err == nil && st.IsDir() {
			dir = filepath.Join(dir, res.Handler)
		}
	}
	if out == nil {
		out = os.Stderr
	}
	cmd := exec.Command("docker", "build", "-t", res.Image, ".")
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker build: %w", err)
	}
	return res.Image, nil
}
