package types

import "testing"

func TestValidateVolumeMount(t *testing.T) {
	ok := []VolumeMount{
		{Name: "data", Mount: "/app/data"},
		{Name: "a", Mount: "/x", ReadOnly: true},
		{Name: "cache_1", Mount: "/var/cache"},
	}
	for _, v := range ok {
		if err := ValidateVolumeMount(v); err != nil {
			t.Fatalf("%+v: %v", v, err)
		}
	}
	bad := []VolumeMount{
		{Name: "", Mount: "/app"},
		{Name: "Bad", Mount: "/app"},
		{Name: "-x", Mount: "/app"},
		{Name: "data", Mount: "relative"},
		{Name: "data", Mount: "/app/../etc"},
	}
	for _, v := range bad {
		if err := ValidateVolumeMount(v); err == nil {
			t.Fatalf("expected error for %+v", v)
		}
	}
	if got := DockerVolumeName("orders", "data"); got != "litefaas-orders-data" {
		t.Fatalf("DockerVolumeName = %q", got)
	}
}
