package scene_test

import (
	"strings"
	"testing"
)

// TestElbowRoute: an elbow link turns at a rounded corner and keeps its dashes and arrow, where a
// straight link between the same boxes is one plain line.
func TestElbowRoute(t *testing.T) {
	_, svg := buildSrc(t, `element "E" {
    item A { pos 0 0; width 100; height 40 }
    item B { pos 140 200; width 100; height 40 }
    link A B { route elbow; display arrow; line-style dashed; line-width 2; color #E0684B }
}`)
	if !strings.Contains(svg, "Q") {
		t.Errorf("no rounded corner in the link:\n%s", svg)
	}
	if !strings.Contains(svg, `stroke-dasharray="5 5"`) {
		t.Errorf("the elbow is not dashed:\n%s", svg)
	}

	_, straight := buildSrc(t, `element "E" {
    item A { pos 0 0; width 100; height 40 }
    item B { pos 0 200; width 100; height 40 }
    link A B { }
}`)
	if strings.Contains(straight, "Q") {
		t.Errorf("a link without route turns a corner:\n%s", straight)
	}
}

// TestElbowAnchorsSpread: two links that leave the same side of a box sit beside each other, not on one
// point, each in the order of the box it leads to.
func TestElbowAnchorsSpread(t *testing.T) {
	_, svg := buildSrc(t, `element "E" {
    item A { pos 0 0; width 100; height 40 }
    item B { pos 0 200; width 40; height 40 }
    item C { pos 60 200; width 40; height 40 }
    link A B { route elbow }
    link A C { route elbow }
}`)
	// A is 100 wide at y 0: its top side holds the anchors at one and two thirds.
	for _, want := range []string{"M33.333 0", "M66.667 0"} {
		if !strings.Contains(svg, want) {
			t.Errorf("no anchor %q in:\n%s", want, svg)
		}
	}
}

// TestRouteUnknownIsWarned: a route the format does not know is reported, not drawn as something silent.
func TestRouteUnknownIsWarned(t *testing.T) {
	screen, _ := buildSrc(t, `element "E" {
    item A { pos 0 0; width 100; height 40 }
    item B { pos 0 200; width 100; height 40 }
    link A B { route curve }
}`)
	if !warned(screen, `route takes straight or elbow, got "curve"`) {
		t.Errorf("warnings: %v", screen.Warnings)
	}
}
