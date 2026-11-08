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

// TestShadowBadExpression: an offset from an expression the evaluator cannot resolve drops that shadow
// with a warning; the build used to panic on the missing value.
func TestShadowBadExpression(t *testing.T) {
	screen, _ := buildEval(t, `element "E" {
    item A { pos 0 0; width 100; height 40; color #ffffff; shadow ${nope} 4 }
}`, stubEval(nil))
	var said []string
	for _, d := range screen.Warnings {
		said = append(said, d.Msg)
	}
	if len(said) == 0 || !strings.Contains(said[0], "not evaluated") {
		t.Errorf("warnings %v, want one about the shadow expression", said)
	}
}
