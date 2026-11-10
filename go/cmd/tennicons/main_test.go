package main

import (
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/tenn"
)

// A shape written in its own <path> starts at the origin of the icon. Joined into one path its commands
// continue from where the shape before left off, so a leading relative moveto has to be made absolute -
// and the pairs after that moveto stay relative, or the shape moves.
func TestAbsoluteStartKeepsTheShapeInPlace(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"M4 4h16", "M4 4h16"},
		{"m14 12", "M14 12"},
		{"m14 12 4 4 4-4", "M14 12 l 4 4 4-4"},
		{"m2 16 4.039-9.69a.5.5 0 0 1 .923 0L11 16", "M2 16 l 4.039-9.69a.5.5 0 0 1 .923 0L11 16"},
	} {
		got, err := absoluteStart(tc.in)
		if err != nil {
			t.Errorf("%q: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.in, got, tc.want)
		}
	}
	if _, err := absoluteStart("l1 1"); err == nil {
		t.Errorf("a shape that starts with a relative lineto is taken for a moveto")
	}
}

// joinParts is the guard of the table: the points of the joined path must be the points of the shapes,
// in the same order, and a shape that would land elsewhere fails the generation.
func TestJoinPartsKeepsEveryShapeInPlace(t *testing.T) {
	// The second shape is relative: read after the first one, without the fix, it would start at 6,6.
	d, err := joinParts([]string{"m1 1 2 2", "m3 3 1 1"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "M1 1 l 2 2 M3 3 l 1 1"; d != want {
		t.Fatalf("got %q, want %q", d, want)
	}
	g, err := gfx.ParseSVGPath(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Pts) != 4 || g.Pts[2].X != 3 || g.Pts[2].Y != 3 || g.Pts[3].X != 4 || g.Pts[3].Y != 4 {
		t.Errorf("points %+v", g.Pts)
	}
}

// A zero sized shape still takes part of the path: lucide has lines and rects with no width or height,
// and their coordinates become path commands too.
func TestShapeShapesBecomePaths(t *testing.T) {
	for _, tc := range []struct{ name, svg, want string }{
		{"line", `<svg><line x1="1" y1="2" x2="3" y2="4"/></svg>`, "M1 2 L3 4"},
		{"rect", `<svg><rect x="2" y="3" width="4" height="5"/></svg>`, "M2 3 L6 3 L6 8 L2 8 Z"},
		{"circle", `<svg><circle cx="12" cy="12" r="2"/></svg>`, "M10 12 A2 2 0 1 0 14 12 A2 2 0 1 0 10 12 Z"},
		{"polyline", `<svg><polyline points="1,2 3,4 5,6"/></svg>`, "M1 2 L3 4 L5 6"},
		{"polygon", `<svg><polygon points="1 2 3 4 5 6"/></svg>`, "M1 2 L3 4 L5 6 Z"},
	} {
		parts, _, err := iconParts([]byte(tc.svg))
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if got := strings.Join(parts, " "); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
	// A rounded rectangle is four arcs; the corners are inside the box.
	parts, _, err := iconParts([]byte(`<svg><rect x="0" y="0" width="10" height="10" rx="2"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	d, err := joinParts(parts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d, "A2 2 0 0 1") {
		t.Errorf("the corners are not arcs: %s", d)
	}
}

// A filled shape is marked, because one path is either filled or stroked and the converter counts the
// difference instead of drawing a solid dot as an outline.
func TestFilledShapeIsMarked(t *testing.T) {
	parts, solid, err := iconParts([]byte(`<svg><circle cx="4" cy="4" r="1" fill="currentColor"/><path d="M0 0h2"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	if !solid || len(parts) != 2 {
		t.Errorf("solid %v, parts %v", solid, parts)
	}
	_, solid, err = iconParts([]byte(`<svg><path d="M0 0h2"/></svg>`))
	if err != nil || solid {
		t.Errorf("solid %v, err %v", solid, err)
	}
}

// The view box and the stroke width are read from the icons themselves and must agree across the set.
func TestIconFrame(t *testing.T) {
	vb, sw, err := iconFrame([]byte(`<svg viewBox="0 0 24 24" stroke-width="2"><path d="M0 0h2"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	if vb != [4]float64{0, 0, 24, 24} || sw != 2 {
		t.Errorf("view box %v, stroke %g", vb, sw)
	}
	if _, _, err := iconFrame([]byte(`<svg stroke-width="2"/>`)); err == nil {
		t.Errorf("a file without a view box is accepted")
	}
}

// The library format is what the repository keeps, so it is the one that needs a test of its own: one item
// per icon, the path, the view box of the set and the stroke, and a header that names the version and the
// licence notice. What the generator writes must also read back as a .tenn file.
func TestRenderTennWritesOneItemPerIcon(t *testing.T) {
	tbl := &table{
		names:   []string{"arrow-left", "zap"},
		paths:   map[string]string{"arrow-left": "M12 19L5 12 12 5", "zap": "M13 2 3 14h9"},
		solid:   map[string]bool{},
		viewBox: [4]float64{0, 0, 24, 24},
		stroke:  2,
	}
	out, err := render("tenn", "", "1.71.0", tbl)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{
		"lucide-static 1.71.0",
		"LICENSES/lucide.txt",
		`item "arrow-left" {`,
		`item "zap" {`,
		`path "M12 19L5 12 12 5"`,
		"view-box 0 0 24 24; border-color #000000; line-width 2",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the library has no %q:\n%s", want, text)
		}
	}
	if n := strings.Count(text, "\nitem "); n != 2 {
		t.Errorf("%d items, want two:\n%s", n, text)
	}
	// A file of items is only worth keeping if the engine reads it.
	if _, p := tenn.Parse(text); p.Errors.HasErrors() {
		t.Fatalf("the library does not parse: %v", p.Errors.Errors)
	}
}
