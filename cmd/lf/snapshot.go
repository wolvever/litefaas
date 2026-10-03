package main

import (
	"sort"

	"github.com/wolvever/litefaas/internal/secret"
	"github.com/wolvever/litefaas/internal/types"
)

// revisionSnapshot records pack/plan and env refs. Secret values are never included.
func revisionSnapshot(res types.Resource, packID string) types.RevisionSnapshot {
	snap := types.RevisionSnapshot{
		PackID:  packID,
		Runtime: string(res.Runtime),
		Kind:    string(res.Kind),
		Port:    res.Port,
		Health:  res.Health,
		Handler: res.Handler,
		Memory:  res.Memory,
		Image:   res.Image,
		Volumes: res.Volumes,
	}
	seen := map[string]struct{}{}
	for _, ref := range secret.ListRefs(res.Env) {
		snap.EnvRefs = append(snap.EnvRefs, types.SnapshotEnvRef{
			Key: ref.Key,
			Ref: "${secret:" + ref.Name + "}",
		})
		seen[ref.Key] = struct{}{}
	}
	keys := make([]string, 0, len(res.Env))
	for k := range res.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, ok := seen[k]; ok {
			continue
		}
		snap.EnvRefs = append(snap.EnvRefs, types.SnapshotEnvRef{Key: k})
	}
	return types.SanitizeSnapshot(snap)
}
