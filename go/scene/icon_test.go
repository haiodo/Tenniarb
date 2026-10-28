package scene

import "testing"

func TestParsePathCachesWithoutSharingFlags(t *testing.T) {
	a, err := parsePath("M0 0 L10 0 L10 10 Z")
	if err != nil {
		t.Fatal(err)
	}
	a.EvenOdd = true
	b, _ := parsePath("M0 0 L10 0 L10 10 Z")
	if b.EvenOdd || len(b.Pts) != len(a.Pts) || &b.Pts[0] != &a.Pts[0] {
		t.Errorf("a cached path shares its points and keeps its own flags: %+v", b)
	}
	if _, err := parsePath("not a path"); err == nil {
		t.Error("a bad path must fail")
	}
}
