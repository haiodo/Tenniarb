package scene_test

import (
	"strings"
	"testing"
)

// TestLineStyleDashes: a dashed or dotted border and link come out as dashes, as the renderer of a
// classic diagram draws them; solid and an unknown word leave the line unbroken.
func TestLineStyleDashes(t *testing.T) {
	for _, tc := range []struct {
		style string
		want  string
	}{
		{"dashed", `stroke-dasharray="5 5"`},
		{"dotted", `stroke-dasharray="1 4"`},
		{"solid", ""},
	} {
		_, svg := buildSrc(t, `element "E" {
    item A { pos 0 0; width 100; height 40; line-style `+tc.style+` }
    item B { pos 200 0; width 100; height 40 }
    link A B { display arrow; line-style `+tc.style+` }
}`)
		if tc.want == "" {
			if n := strings.Count(svg, "stroke-dasharray"); n != 0 {
				t.Errorf("%s: %d dashed strokes, want a solid line", tc.style, n)
			}
			continue
		}
		// The border and the link are both dashed: two strokes carry the dasharray.
		if n := strings.Count(svg, tc.want); n != 2 {
			t.Errorf("%s: %d strokes of %q, want the border and the link", tc.style, n, tc.want)
		}
	}
}

// TestLineStyleUnknownIsWarned: a word the format does not know is reported, not silently solid.
func TestLineStyleUnknownIsWarned(t *testing.T) {
	screen, _ := buildSrc(t, `element "E" {
    item A { pos 0 0; width 100; height 40; line-style wave }
}`)
	if !warned(screen, `line-style takes solid, dashed or dotted, got "wave"`) {
		t.Errorf("warnings: %v", screen.Warnings)
	}
}
