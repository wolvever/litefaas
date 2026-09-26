package types

import "testing"

func TestAlwaysOnAndReplicas(t *testing.T) {
	if KindFunction.AlwaysOn() || !KindBackend.AlwaysOn() || !KindFrontend.AlwaysOn() {
		t.Fatal("always-on kinds")
	}
	if DefaultReplicas(KindFunction) != 0 || DefaultReplicas(KindBackend) != 1 {
		t.Fatal("default replicas")
	}
	got, err := NormalizeReplicas(KindBackend, 0)
	if err != nil || got != 1 {
		t.Fatalf("backend 0 => %d %v", got, err)
	}
	got, err = NormalizeReplicas(KindFunction, 0)
	if err != nil || got != 0 {
		t.Fatalf("function 0 => %d %v", got, err)
	}
	if _, err := NormalizeReplicas(KindBackend, -1); err == nil {
		t.Fatal("expected negative replicas error")
	}
}

func TestParseRuntimeDockerfile(t *testing.T) {
	rt, err := ParseRuntime("dockerfile")
	if err != nil || rt != RuntimeDockerfile {
		t.Fatalf("got %q %v", rt, err)
	}
}
