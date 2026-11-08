package scene_test

import (
	"strings"
	"testing"
)

// layer moves the picture only: a classic element draws background first and hover last, whatever the
// file order.
func TestLayerOrderClassic(t *testing.T) {
	s, _ := buildSrc(t, `element "E" {
        item Hover { pos 0 0; width 10; height 10; layer hover }
        item Back { pos 30 0; width 10; height 10; layer background }
        item Plain { pos 60 0; width 10; height 10 }
    }`)
	if got, want := strings.Join(paths(s), ","), ",Back,Plain,Hover"; got != want {
		t.Errorf("boxes %q, want %q", got, want)
	}
}

// display none keeps the box and its room but draws neither it nor its children.
func TestDisplayNoneHidesChildren(t *testing.T) {
	s, svg := buildSrc(t, `element "E" {
        item Box { width 80; height 60; display none
            item Child { pos 5 5; width 20; height 20; color #ff0000 }
        }
        item After { pos 0 80; width 20; height 20 }
    }`)
	if got, want := strings.Join(paths(s), ","), ",Box,After"; got != want {
		t.Errorf("boxes %q, want %q", got, want)
	}
	if strings.Contains(svg, "rgb(255,0,0)") {
		t.Errorf("display none still draws its children:\n%s", svg)
	}
}
