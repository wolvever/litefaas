package types

import "testing"

func TestEnforceMemory(t *testing.T) {
	got, err := EnforceMemory(0)
	if err != nil || got != DefaultMemoryMiB {
		t.Fatalf("default = %d %v", got, err)
	}
	if _, err := EnforceMemory(8); err == nil {
		t.Fatal("expected low memory error")
	}
	if _, err := EnforceMemory(9000); err == nil {
		t.Fatal("expected high memory error")
	}
	got, err = EnforceMemory(64)
	if err != nil || got != 64 {
		t.Fatalf("64 = %d %v", got, err)
	}
}

func TestEnforceTimeout(t *testing.T) {
	s, err := EnforceTimeout("", KindFunction)
	if err != nil || s != DefaultTimeout.String() {
		t.Fatalf("fn default = %q %v", s, err)
	}
	if _, err := EnforceTimeout("10ms", KindFunction); err == nil {
		t.Fatal("expected short function timeout error")
	}
	if _, err := EnforceTimeout("10m", KindFunction); err == nil {
		t.Fatal("expected long function timeout error")
	}
	if _, err := EnforceTimeout("45s", KindFunction); err != nil {
		t.Fatal(err)
	}
	if _, err := EnforceTimeout("10ms", KindBackend); err != nil {
		t.Fatalf("backend timeout should not be capped: %v", err)
	}
}
