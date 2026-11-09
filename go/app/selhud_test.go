//go:build darwin || linux || windows

package app

import (
	"math"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/scene"
)

// selectionOps is the drawing of a selection picture, op by op.
// drawnOp is one op of a picture, with the extent of its path: the iterator reuses the op and the path it
// hands out, so both are copied here.
type drawnOp struct {
	kind       gfx.OpKind
	color      gfx.Color
	shadow     gfx.Shadow
	style      gfx.StrokeStyle
	minX, minY float64
	maxX, maxY float64
}

func selectionOps(pic *gfx.Picture) []drawnOp {
	var out []drawnOp
	var it gfx.Iter
	it.Reset(pic)
	for it.Next() {
		op := it.Op()
		d := drawnOp{kind: op.Kind, color: op.Color, shadow: op.Shadow}
		if op.Style != nil {
			d.style = *op.Style
			d.style.Dash = append([]float64(nil), op.Style.Dash...)
		}
		if op.Path != nil {
			d.minX, d.minY, d.maxX, d.maxY = pathBox(op.Path)
		}
		out = append(out, d)
	}
	return out
}

// pathBox is the extent of the points of a path.
func pathBox(p *gfx.Path) (minX, minY, maxX, maxY float64) {
	minX, minY, maxX, maxY = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, pt := range p.Pts {
		minX, maxX = math.Min(minX, pt.X), math.Max(maxX, pt.X)
		minY, maxY = math.Min(minY, pt.Y), math.Max(maxY, pt.Y)
	}
	return minX, minY, maxX, maxY
}

// The frame of a selected item is the selector box of the Swift app: its box grown by 5 points, a dash of
// 5 on and 5 off with a phase of 5, and a glow of its own colour. Blue, or green over the item that is
// being edited.
func TestSelectionFrameIsTheSwiftSelector(t *testing.T) {
	sv := newSceneView(nil, nil)
	sv.off = gfx.Point{X: 10, Y: 20}
	sv.byPath["a"] = &scene.Placed{X: 100, Y: 100, W: 50, H: 40}
	sv.byPath["b"] = &scene.Placed{X: 300, Y: 100, W: 50, H: 40}
	ops := selectionOps(sv.selectionPicture([]string{"a", "b"}, "b"))
	if len(ops) != 2 {
		t.Fatalf("%d ops, want the frame of each selected item", len(ops))
	}
	for i, want := range []struct {
		color          gfx.Color
		x0, y0, x1, y1 float64
	}{
		{selectorBlue, 105, 115, 165, 165}, // the box at 110,120 of 50x40 on the scene, grown by 5
		{selectorGreen, 305, 115, 365, 165},
	} {
		op := ops[i]
		if op.kind != gfx.OpStrokeShadow {
			t.Errorf("op %d: %v, want a stroked shadow", i, op.kind)
		}
		if !sameColor(op.color, want.color) || !sameColor(op.shadow.Color, want.color) {
			t.Errorf("op %d: color %v glow %v, want %v", i, op.color, op.shadow.Color, want.color)
		}
		if op.style.Width != 1 || len(op.style.Dash) != 2 || op.style.Dash[0] != 5 || op.style.Dash[1] != 5 || op.style.DashPhase != 5 {
			t.Errorf("op %d: style %+v, want a dash of 5 on 5 off from 5 and a width of 1", i, op.style)
		}
		if op.shadow.Blur != 5 || !op.shadow.Screen || op.shadow.DX != 0 || op.shadow.DY != 0 {
			t.Errorf("op %d: shadow %+v, want a glow of 5 screen points", i, op.shadow)
		}
		if math.Abs(op.minX-want.x0) > 0.6 || math.Abs(op.minY-want.y0) > 0.6 || math.Abs(op.maxX-want.x1) > 0.6 || math.Abs(op.maxY-want.y1) > 0.6 {
			t.Errorf("op %d: frame %v,%v..%v,%v, want the box grown by 5: %v,%v..%v,%v",
				i, op.minX, op.minY, op.maxX, op.maxY, want.x0, want.y0, want.x1, want.y1)
		}
	}
}

// sameColor compares two colours with a tolerance of one step of 8 bits.
func sameColor(a, b gfx.Color) bool {
	near := func(x, y float64) bool { return math.Abs(x-y) < 0.005 }
	return near(a.R, b.R) && near(a.G, b.G) && near(a.B, b.B) && near(a.A, b.A)
}

// The frame of a selection is drawn in the coordinates of the scene, so a pan moves it with the content and
// a zoom does not change where it stands.
func TestSelectionFrameFollowsTheScene(t *testing.T) {
	sv := newSceneView(nil, nil)
	sv.off = gfx.Point{X: 10, Y: 20}
	sv.byPath["a"] = &scene.Placed{X: 100, Y: 100, W: 50, H: 40}
	ops := selectionOps(sv.selectionPicture([]string{"a"}, ""))
	if len(ops) != 1 {
		t.Fatalf("%d ops", len(ops))
	}
	if ops[0].minX != 105 || ops[0].minY != 115 {
		t.Errorf("the frame starts at %v,%v, want the box at 105,115", ops[0].minX, ops[0].minY)
	}
}

// The rubber band and the line of a link being drawn are in the same scene coordinates.
func TestSelectionPicturePreviewGeometry(t *testing.T) {
	sv := newSceneView(nil, nil)
	band := gfx.R(30, 40, 20, 10)
	sv.band = &band
	line := [2]gfx.Point{{X: 1, Y: 2}, {X: 100, Y: 200}}
	sv.line = &line
	pic := sv.selectionPicture(nil, "")
	want := gfx.R(1, 2, 99, 198).Union(band)
	b := pic.Bounds
	if b.Min.X > want.Min.X+3 || b.Min.Y > want.Min.Y+3 || b.Max.X < want.Max.X-3 || b.Max.Y < want.Max.Y-3 {
		t.Fatalf("bounds %v, want the band %v and the line %v", b, band, want)
	}
}
