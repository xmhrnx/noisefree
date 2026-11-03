package settings

import "testing"

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := Load(dir)
	if err != nil || s != Defaults() {
		t.Fatalf("first load = %+v, %v", s, err)
	}
	s.Engine, s.Strength = "dtln", 0.5
	if err := Save(dir, s); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil || got != s {
		t.Fatalf("got %+v, %v", got, err)
	}
}
