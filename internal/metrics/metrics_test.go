package metrics

import "testing"

func TestSnapshotCounts(t *testing.T) {
	m := New()
	m.IncInvokes()
	m.IncDeploys()
	m.IncEdge()
	m.IncErrors()
	s := m.Snapshot(3)
	if s.Invokes != 1 || s.Deploys != 1 || s.EdgeRequests != 1 || s.Errors != 1 || s.Resources != 3 {
		t.Fatalf("snapshot = %+v", s)
	}
}
