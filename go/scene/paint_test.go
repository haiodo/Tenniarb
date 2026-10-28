package scene_test

import (
	"strings"
	"testing"
)

const paintWrap = `element "E" { item "Page" { auto-layout column
%s
} }`

func paintSrc(items string) string { return strings.Replace(paintWrap, "%s", items, 1) }

func TestShadowDrawn(t *testing.T) {
	s, svg := buildSrc(t, paintSrc(`
	item "Box" { width 40; height 20; color #112233; corner-radius 4; shadow 2 3 4 #ff000080 }
	item "Line" { width 40; height 20; border-color #00ff00; line-width 2; shadow 1 1 }`))
	if len(s.Warnings) != 0 {
		t.Errorf("warnings: %v", s.Warnings)
	}
	// Y is up as in pos: shadow dy 3 moves the shadow up the screen by 3
	for _, part := range []string{`translate(2 -3)`, `fill="rgb(255,0,0)" fill-opacity="0.5`, `<feGaussianBlur stdDeviation="2"`,
		`translate(1 -1)`, `stroke="rgb(0,255,0)"`} {
		if !strings.Contains(svg, part) {
			t.Errorf("missing %q in:\n%s", part, svg)
		}
	}
}

func TestNoShadowNoFilter(t *testing.T) {
	_, svg := buildSrc(t, paintSrc(`item "Box" { width 40; height 20; color #112233 }`))
	if strings.Contains(svg, "feGaussianBlur") {
		t.Errorf("blur without shadow:\n%s", svg)
	}
}

func TestShadowWarnings(t *testing.T) {
	s, svg := buildSrc(t, paintSrc(`
	item "A" { width 10; height 10; color #112233; shadow x 1 }
	item "B" { width 10; height 10; color #112233; shadow 1 2 -3 }
	item "C" { width 10; height 10; color #112233; shadow 1 2 3 nocolor }
	item "D" { width 10; height 10; color #112233; shadow 1 2 3 red 5 }`))
	if !warned(s, "shadow takes dx dy") || !warned(s, "the blur is not a number from 0") ||
		!warned(s, `"nocolor" is not a color`) || !warned(s, "the extra values are ignored") {
		t.Errorf("warnings: %v", s.Warnings)
	}
	if n := strings.Count(svg, "feGaussianBlur"); n != 3 { // A has no shadow; B, C, D keep one
		t.Errorf("%d blurs, want 3:\n%s", n, svg)
	}
}

func TestGradientDrawn(t *testing.T) {
	s, svg := buildSrc(t, paintSrc(`
	item "H" { width 100; height 50; color linear 90 #ff0000 0 #0000ff 1 }
	item "V" { width 100; height 50; color linear 180 #ff0000 #00ff00 #0000ff }`))
	if len(s.Warnings) != 0 {
		t.Errorf("warnings: %v", s.Warnings)
	}
	for _, part := range []string{
		`x1="0" y1="25" x2="100" y2="25"`, // 90: left to right through the center
		`x1="50" y1="0" x2="50" y2="50"`,  // 180: top to bottom
		`<stop offset="0" stop-color="rgb(255,0,0)"`, `<stop offset="1" stop-color="rgb(0,0,255)"`,
		`<stop offset="0.5" stop-color="rgb(0,255,0)"`, // no positions: even spread
		`fill="url(#`,
	} {
		if !strings.Contains(svg, part) {
			t.Errorf("missing %q in:\n%s", part, svg)
		}
	}
}

func TestGradientDiagonalReachesCorners(t *testing.T) {
	_, svg := buildSrc(t, paintSrc(`item "D" { width 100; height 50; color linear 135 #ffffff #000000 }`))
	// CSS rule: the line is |w sin a| + |h cos a| long, so the end colors sit on the corners (0,0) and (100,50)
	if !strings.Contains(svg, `x1="12.5" y1="-12.5"`) || !strings.Contains(svg, `x2="87.5" y2="62.5"`) {
		t.Errorf("gradient line:\n%s", svg)
	}
}

func TestGradientWithShadow(t *testing.T) {
	s, svg := buildSrc(t, paintSrc(`item "G" { width 40; height 20; color linear 0 #ff0000 #0000ff; shadow 0 -2 4 #00000080 }`))
	if len(s.Warnings) != 0 {
		t.Errorf("warnings: %v", s.Warnings)
	}
	if !strings.Contains(svg, "feGaussianBlur") || !strings.Contains(svg, "linearGradient") {
		t.Errorf("shadow or gradient missing:\n%s", svg)
	}
}

func TestGradientWarnings(t *testing.T) {
	s, svg := buildSrc(t, paintSrc(`
	item "A" { width 10; height 10; color linear x #ff0000 #0000ff }
	item "B" { width 10; height 10; color linear 90 #ff0000 }
	item "C" { width 10; height 10; color linear 90 #ff0000 0 #0000ff }
	item "D" { width 10; height 10; color linear 90 #ff0000 0.6 #0000ff 0.4 }
	item "E" { width 10; height 10; color linear 90 #ff0000 nope }
	item "F" { width 10; height 10; color linear 90 #ff0000 0 #0000ff 2 }`))
	for _, part := range []string{"the angle is not a number", "needs at least 2 stops", "give a position to every stop or to none",
		"positions must grow within 0..1", `"nope" is not a color`} {
		if !warned(s, part) {
			t.Errorf("no warning %q: %v", part, s.Warnings)
		}
	}
	if n := len(s.Warnings); n != 6 {
		t.Errorf("%d warnings, want 6: %v", n, s.Warnings)
	}
	if strings.Contains(svg, "linearGradient") {
		t.Errorf("a bad gradient was drawn:\n%s", svg)
	}
}

func TestGradientTextColor(t *testing.T) {
	_, svg := buildSrc(t, paintSrc(`item "T" { title "x"; color linear 90 #000000 #333333 }`))
	if !strings.Contains(svg, `rgb(255,255,255)">x<`) {
		t.Errorf("text on a dark gradient is not white:\n%s", svg)
	}
}

func TestGradientOnlyInFill(t *testing.T) {
	s, svg := buildSrc(t, paintSrc(`item "A" { width 10; height 10; color #112233; border-color linear 90 #ff0000 #0000ff }`))
	if !warned(s, `border-color: "linear" is not a color`) || strings.Contains(svg, "linearGradient") {
		t.Errorf("warnings %v, svg:\n%s", s.Warnings, svg)
	}
}
