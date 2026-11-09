//go:build darwin || linux || windows

package app

import (
	"math"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
)

// classicDriftDoc is a classic diagram the two renderers measure differently: the title has no width,
// so it takes the width of its text, and go/render wraps that text into a box of another size.
const classicDriftDoc = "element \"D\" {\n" +
	"  item A { pos 0 0; title \"a fairly long title that measures more than the renderer thinks\" }\n" +
	"  item B { pos 0 300; size 40 }\n}\n"

// The map of a classic picture is the offset the scene drew it with, so the pointer and the session
// speak the same coordinates as the picture, whatever size the old renderer gave the boxes.
func TestClassicMapFollowsTheDrawnPicture(t *testing.T) {
	v, sv, el := classicView(t, classicDriftDoc)
	if !v.syncMap(sv) {
		t.Fatal("a classic diagram drawn by the scene has no map")
	}
	if v.smap != sv.mapOf() {
		t.Fatalf("the map is %v, the picture was drawn at %v", v.smap, sv.mapOf())
	}
	if old := MapOf(v.session.Scene().Bounds()); old == v.smap {
		t.Fatalf("the map %v is the one of the old renderer: the fixture must show the drift", old)
	}
	// A press in the middle of the drawn box is a press on the item.
	for _, it := range el.Items {
		b, ok := v.drawnRect(it)
		if !ok {
			t.Fatalf("item %q is not drawn", it.Name)
		}
		at := v.smap.ToScene(gfx.Point{X: b.Min.X + b.W()/2, Y: b.Min.Y + b.H()/2})
		hits := v.session.Hit(at)
		if len(hits) == 0 || hits[0] != it {
			t.Errorf("a click at the drawn box of %q hits %v", it.Name, hits)
		}
	}
}

// The size the inspector writes for a classic diagram is the size of the drawn box: the field shows the
// item as it is on the canvas, not the box the old renderer computed for it.
func TestClassicItemSizeIsTheDrawnBox(t *testing.T) {
	v, sv, el := classicView(t, classicDriftDoc)
	v.syncMap(sv)
	it := el.Items[0]
	b, ok := v.drawnRect(it)
	if !ok {
		t.Fatal("item A is not drawn")
	}
	d := v.session.Scene().Drawables[it]
	if d == nil {
		t.Fatal("no drawable to compare with")
	}
	if old := d.Bounds(); math.Abs(old.Width-b.W()) < 1e-9 && math.Abs(old.Height-b.H()) < 1e-9 {
		t.Fatal("the drawn box and the old one are equal: the fixture must show the drift")
	}
	v.session.Select([]*model.DiagramItem{it})
	w, h := v.itemSize()
	if math.Abs(w-b.W()) > 1e-9 || math.Abs(h-b.H()) > 1e-9 {
		t.Errorf("the inspector writes %v x %v, the drawn box is %v x %v", w, h, b.W(), b.H())
	}
}

// A press in the margin the old renderer gave a text item, where the scene draws nothing, does not grab
// it: the picture has the last word on what is under the pointer.
func TestClassicPressInTheDrawnMarginMisses(t *testing.T) {
	v, sv, el := classicView(t, classicDriftDoc)
	v.syncMap(sv)
	it := el.Items[0]
	b, ok := v.drawnRect(it)
	if !ok {
		t.Fatal("item A is not drawn")
	}
	margin := gfx.Point{X: b.Max.X + 10, Y: b.Min.Y + b.H()/2}
	at := v.smap.ToScene(margin)
	if !v.session.MouseDown(editor.MouseEvent{P: at, Clicks: 1}) {
		// Nothing grabbed: the window pans, exactly what a click on empty canvas does.
		v.session.MouseUp(editor.MouseEvent{P: at})
		if sel := v.session.Selection(); len(sel) != 0 {
			t.Fatalf("the press in the margin selected %v", sel)
		}
		return
	}
	v.session.MouseUp(editor.MouseEvent{P: at})
	t.Fatalf("the press in the margin grabbed %v", v.session.Selection())
}

// A press inside the drawn box still grabs the item.
func TestClassicPressInsideTheDrawnBoxGrabs(t *testing.T) {
	v, sv, el := classicView(t, classicDriftDoc)
	v.syncMap(sv)
	it := el.Items[0]
	b, _ := v.drawnRect(it)
	at := v.smap.ToScene(gfx.Point{X: b.Min.X + b.W()/2, Y: b.Min.Y + b.H()/2})
	if !v.session.MouseDown(editor.MouseEvent{P: at, Clicks: 1}) {
		t.Fatal("the press on the drawn box grabbed nothing")
	}
	if sel := v.session.Selection(); len(sel) != 1 || sel[0] != it {
		t.Errorf("the press selected %v", sel)
	}
	v.session.MouseUp(editor.MouseEvent{P: at})
}
