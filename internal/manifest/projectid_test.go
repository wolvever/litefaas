package manifest

import "testing"

func TestProjectIDSortsAndDedupes(t *testing.T) {
	id, err := ProjectID([]string{"web", "api", "web"})
	if err != nil {
		t.Fatal(err)
	}
	if id != "api+web" {
		t.Fatalf("got %q", id)
	}
	one, err := ProjectID([]string{" Hello "})
	if err != nil || one != "Hello" {
		t.Fatalf("%q %v", one, err)
	}
	if _, err := ProjectID(nil); err == nil {
		t.Fatal("expected error")
	}
	if _, err := ProjectID([]string{"bad name"}); err == nil {
		t.Fatal("expected error")
	}
}
