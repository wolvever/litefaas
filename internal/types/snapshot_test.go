package types

import "testing"

func TestSanitizeSnapshotDropsPlaintext(t *testing.T) {
	in := RevisionSnapshot{
		PackID: "python-django",
		Image:  "app:1",
		EnvRefs: []SnapshotEnvRef{
			{Key: "DATABASE_URL", Ref: "${secret:db}"},
			{Key: "PLAIN", Ref: "postgres://user:secret@db/app"},
			{Key: "EMPTY"},
			{Key: "  "},
			{Key: "MIXED", Ref: "prefix-${secret:db}"},
		},
	}
	got := SanitizeSnapshot(in)
	if got.PackID != "python-django" || got.Image != "app:1" {
		t.Fatalf("plan fields lost: %+v", got)
	}
	if len(got.EnvRefs) != 4 {
		t.Fatalf("refs = %+v", got.EnvRefs)
	}
	if got.EnvRefs[0].Ref != "${secret:db}" {
		t.Fatalf("secret ref stripped: %+v", got.EnvRefs[0])
	}
	if got.EnvRefs[1].Key != "PLAIN" || got.EnvRefs[1].Ref != "" {
		t.Fatalf("plaintext kept: %+v", got.EnvRefs[1])
	}
	if got.EnvRefs[3].Key != "MIXED" || got.EnvRefs[3].Ref != "" {
		t.Fatalf("partial ref kept: %+v", got.EnvRefs[3])
	}
}
