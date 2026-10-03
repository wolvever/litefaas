// Package release runs data-driven commands in the candidate image before cutover.
// Commands come from litefaas.yaml or a stack pack. Nothing here is framework-specific.
package release

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/wolvever/litefaas/internal/dockercli"
	"github.com/wolvever/litefaas/internal/types"
)

// Timeout is the per-command limit. Cancel or a non-zero exit leaves the live version up.
const Timeout = 120 * time.Second

// Args is `docker run --rm` of the candidate image: same env and mounts, no published port.
func Args(res types.Resource, command string) []string {
	port := res.Port
	if port == 0 {
		port = 8080
	}
	mem := res.Memory
	if mem <= 0 {
		mem = 128
	}
	memFlag := fmt.Sprintf("%dm", mem)
	args := []string{
		"run", "--rm",
		"--name", "litefaas-" + res.Name + "-release",
		"--label", "litefaas.managed=1",
		"--label", "litefaas.name=" + res.Name,
		"--label", "litefaas.phase=release",
		"--add-host", "host.docker.internal:host-gateway",
		"--memory", memFlag,
		"--memory-swap", memFlag,
	}
	for _, v := range res.Volumes {
		vol := types.DockerVolumeName(res.Name, v.Name)
		spec := vol + ":" + v.Mount
		if v.ReadOnly {
			spec += ":ro"
		}
		args = append(args, "-v", spec)
	}
	keys := make([]string, 0, len(res.Env))
	for k := range res.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "-e", k+"="+res.Env[k])
	}
	args = append(args, "-e", fmt.Sprintf("PORT=%d", port), res.Image, "sh", "-c", command)
	return args
}

// Run executes each release command. The caller must not have renamed the live container yet.
func Run(ctx context.Context, res types.Resource) error {
	for i, cmd := range res.Release {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}
		name := "litefaas-" + res.Name + "-release"
		_, _ = dockercli.Output(ctx, "docker", "rm", "-f", name)
		cctx, cancel := context.WithTimeout(ctx, Timeout)
		_, err := dockercli.Output(cctx, "docker", Args(res, cmd)...)
		cancel()
		if err != nil {
			if cctx.Err() != nil && ctx.Err() == nil {
				return fmt.Errorf("release command %d timed out after %s (live version unchanged)", i+1, Timeout)
			}
			if ctx.Err() != nil {
				return fmt.Errorf("release command %d cancelled (live version unchanged): %w", i+1, ctx.Err())
			}
			return fmt.Errorf("release command %d failed (live version unchanged): %w", i+1, err)
		}
	}
	return nil
}

// Commands resolves manifest release over pack release. Empty means run nothing.
func Commands(manifestRelease, packRelease []string) []string {
	if len(manifestRelease) > 0 {
		return append([]string(nil), manifestRelease...)
	}
	if len(packRelease) > 0 {
		return append([]string(nil), packRelease...)
	}
	return nil
}
