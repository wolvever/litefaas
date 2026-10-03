package types

import (
	"regexp"
	"strings"
)

// snapshotSecretRef matches a whole-value ${secret:…} expression (not a resolved secret).
var snapshotSecretRef = regexp.MustCompile(`^\$\{secret:[A-Za-z0-9][A-Za-z0-9._/-]{0,96}\}$`)

// SanitizeSnapshot drops anything that is not an env key or a ${secret:…} ref.
// Callers must not put resolved secret values in other snapshot fields.
func SanitizeSnapshot(in RevisionSnapshot) RevisionSnapshot {
	out := in
	if len(in.EnvRefs) == 0 {
		out.EnvRefs = nil
		return out
	}
	refs := make([]SnapshotEnvRef, 0, len(in.EnvRefs))
	for _, r := range in.EnvRefs {
		key := strings.TrimSpace(r.Key)
		if key == "" {
			continue
		}
		er := SnapshotEnvRef{Key: key}
		ref := strings.TrimSpace(r.Ref)
		if snapshotSecretRef.MatchString(ref) {
			er.Ref = ref
		}
		refs = append(refs, er)
	}
	if len(refs) == 0 {
		refs = nil
	}
	out.EnvRefs = refs
	return out
}
