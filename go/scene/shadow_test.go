package scene_test

import (
	"strings"
	"testing"
)

// TestTwoShadows: `shadow` may be written twice to soft a box - a wide faint pass and a tight one - and
// both are drawn, where the format used to keep only the last command.
func TestTwoShadows(t *testing.T) {
	_, one := buildSrc(t, `element "E" {
    item A { pos 0 0; width 100; height 40; color #ffffff; corner-radius 8; shadow 0 2 4 #00000020 }
}`)
	_, two := buildSrc(t, `element "E" {
    item A { pos 0 0; width 100; height 40; color #ffffff; corner-radius 8
        shadow 0 2 4 #00000020
        shadow 0 14 36 #00000008 }
}`)
	single := strings.Count(one, "filter")
	pair := strings.Count(two, "filter")
	if single == 0 {
		t.Fatalf("one shadow draws no filter:\n%s", one)
	}
	if pair != 2*single {
		t.Errorf("%d filters for two shadows, %d for one:\n%s", pair, single, two)
	}
}
