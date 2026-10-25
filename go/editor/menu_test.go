package editor

import (
	"testing"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
)

func labels(es []Entry) []string {
	var out []string
	for _, e := range es {
		if e.Sep {
			out = append(out, "-")
		} else {
			out = append(out, e.Label)
		}
	}
	return out
}

func find(es []Entry, label string) *Entry {
	for i := range es {
		if es[i].Label == label {
			return &es[i]
		}
	}
	return nil
}

func TestCanvasEntriesNothingSelected(t *testing.T) {
	f := newFixture(t)
	f.s.Pick(render.Point{X: 500, Y: 500})
	got := f.s.CanvasEntries(render.Point{X: 500, Y: 500}, MenuHost{Layout: func() {}})
	eq(t, "labels", labels(got), []string{"New item", "Test layout", "-", "Style", "Global Styles"})
	got = f.s.CanvasEntries(render.Point{X: 500, Y: 500}, MenuHost{})
	eq(t, "no layout host", labels(got), []string{"New item", "-", "Style", "Global Styles"})
	find(got, "New item").Run()
	eq(t, "item added", len(f.s.element.Items), 6)
	eq(t, "pivot", f.s.Selection()[0].X, 500.0)
}

func TestCanvasEntriesOneSelected(t *testing.T) {
	f := newFixture(t)
	p := f.centre(f.a)
	f.s.Pick(p)
	eq(t, "picked", f.s.Selection(), []*model.DiagramItem{f.a})
	var attached, exported int
	got := f.s.CanvasEntries(p, MenuHost{AttachImage: func() { attached++ }, ExportHTML: func() { exported++ }})
	eq(t, "labels", labels(got), []string{
		"New item", "-", "New linked item", "Linked styled item", "-", "Style", "Quick Style", "-", "Duplicate", "-", "Attach image", "Order", "-", "Export text as html", "-", "Delete",
	})
	find(got, "Attach image").Run()
	find(got, "Export text as html").Run()
	eq(t, "host calls", []int{attached, exported}, []int{1, 1})
	quick := find(got, "Quick Style").Sub
	eq(t, "first quick styles", labels(quick)[:4], []string{"Color", "Border color", "Text Color", "-"})
	find(find(quick, "Color").Sub, "🔴red").Run()
	contains(t, f.props(f.a), "color red")
	find(got, "Linked styled item").Run()
	contains(t, f.props(f.s.Selection()[0]), "color red")
	find(got, "Delete").Run()
}

func TestCanvasEntriesSeveralSelected(t *testing.T) {
	f := newFixture(t)
	f.s.Select([]*model.DiagramItem{f.a, f.b})
	got := f.s.CanvasEntries(f.centre(f.a), MenuHost{})
	align := find(got, "Align")
	if align == nil {
		t.Fatal("several items can be aligned")
	}
	find(align.Sub, "Leading Edges").Run()
	eq(t, "aligned", []float64{f.a.X, f.b.X}, []float64{0, 0})
	if find(got, "Order") != nil {
		t.Error("order is for one item")
	}
	if len(find(got, "Style").Sub) == 0 || find(got, "Quick Style") != nil {
		t.Error("quick style is for one item")
	}
}

func TestCanvasEntriesOverlappingItems(t *testing.T) {
	f := newFixture(t, fixtureOpts{src: "element \"D\" {\n item \"A\" { pos 0 0 }\n item \"B\" { pos 0 0 }\n}"})
	p := f.centre(f.s.element.Items[0])
	f.s.Pick(p)
	got := f.s.CanvasEntries(p, MenuHost{})
	sel := find(got, "Select")
	if sel == nil || len(sel.Sub) != 2 {
		t.Fatalf("Select offers both: %v", labels(got))
	}
	sel.Sub[1].Run()
	eq(t, "picked", f.s.Selection(), []*model.DiagramItem{f.s.element.Items[1]})
}

func TestStyleEntriesListNamedStyles(t *testing.T) {
	f, _, b, _ := newStyles(t, false)
	f.s.Select([]*model.DiagramItem{b})
	style := find(f.s.StyleEntries(), "Style")
	eq(t, "style submenu", labels(style.Sub), []string{"big", "-", "Define new style"})
	find(style.Sub, "Define new style").Run()
	eq(t, "defined", f.s.StyleNames(), []string{"big", "new_style_2"})
}

func TestQuickEntriesMarkersAreGrouped(t *testing.T) {
	f := newFixture(t)
	f.s.Select([]*model.DiagramItem{f.a})
	for _, q := range QuickStylesFor(model.ItemKindItem) {
		if q.Prop != "marker" {
			continue
		}
		es := f.s.QuickEntries(q)
		eq(t, "groups", len(es), len(Markers))
		es[0].Sub[0].Run()
	}
	contains(t, f.props(f.a), `marker "😀"`)
}

func TestQuickPanel(t *testing.T) {
	var item, link []string
	for _, s := range QuickPanel(model.ItemKindItem) {
		item = append(item, s.Style.Prop)
	}
	for _, s := range QuickPanel(model.ItemKindLink) {
		link = append(link, s.Style.Prop)
	}
	eq(t, "item", item, []string{"marker", "font-size", "display", "color", "line-style", "line-width"})
	eq(t, "link", link, []string{"display", "line-style", "line-width"})
}
