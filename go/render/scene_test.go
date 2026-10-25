package render

import (
	"fmt"
	"math"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
)

// Port of the non-AppKit parts of SceneLayoutTests.swift (DrawableScene) and SceneRenderTests.swift.

func makeDiagram(count int, linked bool) (*model.Element, []*model.DiagramItem) {
	m := model.NewElementModel()
	diagram := model.NewElement("Diagram")
	m.AddElement(diagram)
	var items []*model.DiagramItem
	for i := 0; i < count; i++ {
		it := model.NewDiagramItem(model.ItemKindItem, fmt.Sprintf("Item%d", i))
		it.X, it.Y = float64(i*10), float64(i*10)
		diagram.AddItem(it)
		items = append(items, it)
	}
	if linked {
		for i := 1; i < count; i++ {
			diagram.AddSourceTarget(items[i-1], items[i])
		}
	}
	return diagram, items
}

func TestSceneBuildsDrawablePerItem(t *testing.T) {
	diagram, items := makeDiagram(3, false)
	s := NewDrawableScene(diagram, testOpts())
	for _, it := range items {
		if s.Drawables[it] == nil {
			t.Errorf("%s needs a drawable", it.Name)
		}
	}
	if b := s.Bounds(); !(b.Width > 0 && b.Height > 0) {
		t.Errorf("bounds %+v", b)
	}
}

func TestEmptySceneHasEmptyBounds(t *testing.T) {
	d, _ := makeDiagram(0, false)
	if w := NewDrawableScene(d, testOpts()).Bounds().Width; w != 0 {
		t.Errorf("width %v", w)
	}
}

func TestSceneTraverseVisitsSelfAndDirectChildren(t *testing.T) {
	d, _ := makeDiagram(3, true)
	s := NewDrawableScene(d, testOpts())
	n := 0
	s.Traverse(func(Drawable) bool { n++; return true })
	if n != 2 {
		t.Errorf("visited %d", n)
	}
	n = 0
	s.Traverse(func(Drawable) bool { n++; return false })
	if n != 1 {
		t.Errorf("stop: visited %d", n)
	}
}

func TestLinkDrawableFollowsEndpoints(t *testing.T) {
	d, items := makeDiagram(2, true)
	var link *model.DiagramItem
	for _, it := range d.Items {
		if it.IsLink() {
			link = it
		}
	}
	before, _ := NewDrawableScene(d, testOpts()).GetItemBounds(link)
	items[1].X += 500
	after, _ := NewDrawableScene(d, testOpts()).GetItemBounds(link)
	if before == after {
		t.Error("moving an endpoint must move the link")
	}
}

func TestSceneImplementsLayoutScene(t *testing.T) {
	d, items := makeDiagram(4, false)
	store := model.NewElementModelStore(d.Model)
	s := NewDrawableScene(d, testOpts())
	ctx := model.NewLayoutContext(d, s, store, model.Rect{Width: 800, Height: 600})

	if b := ctx.GetNodeBounds(items[0]); !(b.Width > 0) {
		t.Errorf("a drawn item has a real size: %+v", b)
	}
	if b := ctx.GetNodeBounds(model.NewDiagramItem(model.ItemKindItem, "NotInScene")); b != (model.Rect{}) {
		t.Errorf("unknown item: %+v", b)
	}
	ops, err := model.NewGridLayout().Apply(ctx, true)
	if err != nil || len(ops) != len(items) {
		t.Errorf("grid ops %d err %v", len(ops), err)
	}
}

const sceneSource = `
element Root {
    item First {
        pos 10 20
        color red
        font-size 20
    }
    item Second {
        pos 200 40
        display text
        text-color blue
    }
    item Circle {
        pos 100 200
        display circle
    }
    link First Second {
    }
}`

func TestSceneBuildsItemsAndLines(t *testing.T) {
	el := parseDiagram(t, sceneSource)
	s := NewDrawableScene(el, testOpts())
	for _, it := range el.Items {
		if s.Drawables[it] == nil {
			t.Errorf("%s needs a drawable", it.Name)
		}
	}
	for _, it := range el.Items {
		if it.Kind == model.ItemKindLink {
			if _, ok := s.Drawables[it].(*DrawableLine); !ok {
				t.Errorf("link drawable is %T", s.Drawables[it])
			}
		}
	}
	if _, ok := s.Drawables[el.Items[1]].(*EmptyBox); !ok {
		t.Errorf("display text is %T", s.Drawables[el.Items[1]])
	}
	if _, ok := s.Drawables[el.Items[2]].(*CircleBox); !ok {
		t.Errorf("display circle is %T", s.Drawables[el.Items[2]])
	}
}

func drawableOf(t *testing.T, src string) (*model.Element, *DrawableScene, Drawable) {
	el := parseDiagram(t, src)
	s := NewDrawableScene(el, testOpts())
	return el, s, s.Drawables[el.Items[0]]
}

func TestItemWithExplicitSize(t *testing.T) {
	_, _, d := drawableOf(t, "element Root {\n item Wide {\n pos 0 0\n width 300\n height 120\n }\n}")
	assertNear(t, "width", d.Bounds().Width, 300, 2)
	assertNear(t, "height", d.Bounds().Height, 120, 2)
}

func TestBackgroundLayerItemIsNotSelectable(t *testing.T) {
	_, _, d := drawableOf(t, "element Root {\n item Back {\n pos 0 0\n layer background\n }\n}")
	if d.Selectable() {
		t.Error("background item is selectable")
	}
}

func TestItemGeometryIsYUp(t *testing.T) {
	_, _, d := drawableOf(t, "element Root {\n item A {\n pos 10 20\n width 100\n height 40\n }\n}")
	assertRect(t, "selector bounds", d.SelectorBounds(), Rect{10, -20, 100, 40})
}

func TestSceneWithoutChildrenAndSubset(t *testing.T) {
	el := parseDiagram(t, sceneSource)
	o := testOpts()
	o.SkipChildren = true
	if w := NewDrawableScene(el, o).Bounds().Width; w != 0 {
		t.Errorf("no children: %v", w)
	}
	o = testOpts()
	o.Items = el.Items[:1]
	s := NewDrawableScene(el, o)
	if s.Drawables[el.Items[0]] == nil || len(s.Drawables) != 1 {
		t.Errorf("subset: %d drawables", len(s.Drawables))
	}
}

func TestStylesBlockAndUseStyle(t *testing.T) {
	el := parseDiagram(t, `element Root {
    styles {
        item {
            font-size 30
        }
        big {
            width 200
            height 80
        }
    }
    item A {
        pos 0 0
    }
    item B {
        pos 0 100
        use-style big
    }
}`)
	s := NewDrawableScene(el, testOpts())
	if s.Style.DefaultItemStyle.FontSize != 30 {
		t.Errorf("default font %v", s.Style.DefaultItemStyle.FontSize)
	}
	if math.Abs(s.Drawables[el.Items[0]].Bounds().Height-80) <= 1 {
		t.Error("A does not use the style")
	}
	assertNear(t, "B width", s.Drawables[el.Items[1]].Bounds().Width, 200, 2)
}

func TestInheritTakesPropertiesFromParentItem(t *testing.T) {
	root := parseDiagram(t, `element Outer {
    item Base {
        color red
    }
    element Inner {
        item Child {
            inherit "../Base"
        }
    }
}`)
	inner := root.Elements[0]
	s := NewDrawableScene(inner, testOpts())
	box := s.Drawables[inner.Items[0]].(*RoundBox)
	if box.Style.Color != (Color{1, 0, 0, 1}) {
		t.Errorf("inherited colour %+v", box.Style.Color)
	}
}

func titleOf(s *DrawableScene, it *model.DiagramItem) string {
	return s.Drawables[it].(*RoundBox).Children[0].(*TextBox).Text().String()
}

func TestExpressionValuesReachTheScene(t *testing.T) {
	el := parseDiagram(t, "element Root {\n item A {\n pos 0 0\n title ${1 + 2}\n font-size ${5 + 7}\n }\n}")
	o := testOpts()
	o.Exec = fakeExec{values: map[string]any{"1 + 2": 3.0, "5 + 7": 12.0}}
	s := NewDrawableScene(el, o)
	if got := titleOf(s, el.Items[0]); got != "3" {
		t.Errorf("title %q", got)
	}
	if fs := s.Drawables[el.Items[0]].(*RoundBox).Style.FontSize; fs != 12 {
		t.Errorf("font size %v", fs)
	}
	// Without an engine the tokens render as written.
	plain := NewDrawableScene(el, testOpts())
	if got := titleOf(plain, el.Items[0]); got != "1 + 2" {
		t.Errorf("unevaluated title %q", got)
	}
}

func TestItemSizeAndTextSize(t *testing.T) {
	el := parseDiagram(t, "element Root {\n item A {\n pos 0 0\n width 120\n height 50\n }\n}")
	sz, err := ItemSize(el.Items[0], testOpts())
	if err != nil || sz.Width != 120 || sz.Height != 50 {
		t.Errorf("item size %+v %v", sz, err)
	}
	if _, err := ItemSize(model.NewDiagramItem(model.ItemKindItem, "x"), testOpts()); err == nil {
		t.Error("item without a parent must fail")
	}
	// EstimateText: 0.6 em per rune, so 5 runes at 18 is 54; line height floor(18*2412/2048 ...) is not used here.
	ts := MeasureTextSize(testOpts(), "Hello", 18)
	if ts.Width != math.Ceil(5*18*0.6) {
		t.Errorf("text size %+v", ts)
	}
}

func TestSceneOrdersLayers(t *testing.T) {
	el := parseDiagram(t, `element Root {
    item Top { pos 0 0
        layer hover }
    item Mid { pos 0 0 }
    item Back { pos 0 0
        layer background }
}`)
	s := NewDrawableScene(el, testOpts())
	var names []string
	for _, d := range s.elements.Ordered() {
		names = append(names, d.(interface{ item() *model.DiagramItem }).item().Name)
	}
	want := []string{"Back", "Mid", "Top"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("draw order %v, want %v", names, want)
		}
	}
}

func TestDisplayNoneIsNotDrawn(t *testing.T) {
	el := parseDiagram(t, "element Root {\n item A {\n pos 0 0\n display none\n }\n item B { pos 50 0 }\n}")
	s := NewDrawableScene(el, testOpts())
	if len(s.elements.Ordered()) != 1 {
		t.Errorf("drawn %d", len(s.elements.Ordered()))
	}
	if s.Drawables[el.Items[0]].Bounds().Width <= 0 {
		t.Error("an invisible box still takes room")
	}
}
