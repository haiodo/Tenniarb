package model

import (
	"math"
	"slices"
	"testing"
)

// Port of web/packages/core/test/layout.test.ts. Every item is a fixed 60x30 box at its position.
const (
	itemW = 60.0
	itemH = 30.0
)

type fixedScene struct{ items []*DiagramItem }

func (s fixedScene) GetItemBounds(node *DiagramItem) (Rect, bool) {
	if !slices.Contains(s.items, node) {
		return Rect{}, false
	}
	return Rect{node.X, node.Y, itemW, itemH}, true
}

func (s fixedScene) GetBounds() Rect {
	if len(s.items) == 0 {
		return Rect{}
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, i := range s.items {
		minX, minY = math.Min(minX, i.X), math.Min(minY, i.Y)
		maxX, maxY = math.Max(maxX, i.X+itemW), math.Max(maxY, i.Y+itemH)
	}
	return Rect{minX, minY, maxX - minX, maxY - minY}
}

// makeDiagram makes count items in a column, plus links chaining them when linked.
func makeDiagram(count int, linked bool) (*ElementModelStore, *Element, []*DiagramItem) {
	store, diagram := makeStore()
	var items []*DiagramItem
	for i := range count {
		item := NewDiagramItem(ItemKindItem, "Item"+string(rune('0'+i)))
		item.X, item.Y = float64(i*10), float64(i*10)
		diagram.AddItem(item)
		items = append(items, item)
	}
	if linked {
		for i := 1; i < count; i++ {
			diagram.AddSourceTarget(items[i-1], items[i])
		}
	}
	return store, diagram, items
}

var defaultView = Rect{0, 0, 800, 600}

func makeContext(store *ElementModelStore, element *Element, items []*DiagramItem, bounds Rect) *LayoutContext {
	return NewLayoutContext(element, fixedScene{items}, store, bounds)
}

func applyAll(t *testing.T, ops []ElementOperation) {
	t.Helper()
	for _, op := range ops {
		must(t, op.Apply())
	}
}

func positions(items []*DiagramItem) [][2]float64 {
	out := [][2]float64{}
	for _, i := range items {
		out = append(out, [2]float64{i.X, i.Y})
	}
	return out
}

// mustOps panics on error, which fails the test, so that it can take the two results of Apply directly.
func mustOps(ops []ElementOperation, err error) []ElementOperation {
	if err != nil {
		panic(err)
	}
	return ops
}

// LayoutContext

func TestFindNodesSkipsLinks(t *testing.T) {
	store, diagram, items := makeDiagram(3, true)
	context := makeContext(store, diagram, items, defaultView)
	eq(t, len(context.Nodes), len(items), "nodes")
	for _, n := range context.Nodes {
		eq(t, n.Kind, ItemKindItem, "kind")
	}
}

func TestFindEdgesReturnsOnlyLinks(t *testing.T) {
	store, diagram, items := makeDiagram(3, true)
	context := makeContext(store, diagram, items, defaultView)
	eq(t, len(context.Edges), 2, "edges")
	for _, e := range context.Edges {
		eq(t, e.Kind, ItemKindLink, "kind")
	}
}

func TestViewBoundsAreTheOnesPassedIn(t *testing.T) {
	store, diagram, items := makeDiagram(1, false)
	bounds := Rect{5, 6, 100, 200}
	eq(t, makeContext(store, diagram, items, bounds).GetViewBounds(), bounds, "bounds")
}

func TestBoundsOfKnownNodeIsNonEmpty(t *testing.T) {
	store, diagram, items := makeDiagram(1, false)
	b := makeContext(store, diagram, items, defaultView).GetNodeBounds(items[0])
	if b.Width <= 0 || b.Height <= 0 {
		t.Fatalf("empty bounds %v", b)
	}
}

func TestBoundsOfUnknownNodeIsZero(t *testing.T) {
	store, diagram, items := makeDiagram(1, false)
	stranger := NewDiagramItem(ItemKindItem, "NotInScene")
	eq(t, makeContext(store, diagram, items, defaultView).GetNodeBounds(stranger), Rect{}, "bounds")
}

func TestEveryNodeIsMovable(t *testing.T) {
	store, diagram, items := makeDiagram(2, false)
	context := makeContext(store, diagram, items, defaultView)
	for _, i := range items {
		if !context.IsMovable(i) {
			t.Fatal("not movable")
		}
	}
}

func TestPrePostLayoutPassesRunInOrder(t *testing.T) {
	store, diagram, items := makeDiagram(1, false)
	context := makeContext(store, diagram, items, defaultView)
	var order []string
	context.PreLayoutPass = []func(){func() { order = append(order, "pre1") }, func() { order = append(order, "pre2") }}
	context.PostLayoutPass = []func(){func() { order = append(order, "post") }}
	context.PreLayout()
	context.PostLayout()
	eqNames(t, order, "pre1", "pre2", "post")
}

func TestApplyWithoutAlgorithmReturnsNoOperations(t *testing.T) {
	store, diagram, items := makeDiagram(2, false)
	ops, err := makeContext(store, diagram, items, defaultView).Apply(true)
	must(t, err)
	eq(t, len(ops), 0, "ops")
}

func TestApplyRunsPassesAroundTheAlgorithm(t *testing.T) {
	store, diagram, items := makeDiagram(2, false)
	context := makeContext(store, diagram, items, defaultView)
	var order []string
	context.PreLayoutPass = []func(){func() { order = append(order, "pre") }}
	context.PostLayoutPass = []func(){func() { order = append(order, "post") }}
	context.Layout = NewGridLayout()
	_, err := context.Apply(true)
	must(t, err)
	eqNames(t, order, "pre", "post")
}

// Layout algorithms

func TestGridLayoutProducesOnePositionOperationPerNode(t *testing.T) {
	store, diagram, items := makeDiagram(4, false)
	ops := mustOps(NewGridLayout().Apply(makeContext(store, diagram, items, defaultView), true))
	eq(t, len(ops), len(items), "ops")
}

func TestGridLayoutIsNoOpWhenNotClean(t *testing.T) {
	store, diagram, items := makeDiagram(4, false)
	ops := mustOps(NewGridLayout().Apply(makeContext(store, diagram, items, defaultView), false))
	eq(t, len(ops), 0, "ops")
}

func TestGridLayoutMovesItemsApart(t *testing.T) {
	store, diagram, items := makeDiagram(4, false)
	applyAll(t, mustOps(NewGridLayout().Apply(makeContext(store, diagram, items, defaultView), true)))
	seen := map[[2]float64]bool{}
	for _, p := range positions(items) {
		seen[p] = true
	}
	eq(t, len(seen), len(items), "distinct positions")
}

func TestGridLayoutOnEmptyDiagramProducesNothing(t *testing.T) {
	store, diagram, items := makeDiagram(0, false)
	ops := mustOps(NewGridLayout().Apply(makeContext(store, diagram, items, defaultView), true))
	eq(t, len(ops), 0, "ops")
}

// Scene is 90x60 at the origin: 3 columns x 2 rows of 30x30 cells, 28.5 box, 0.75 offset (hand-computed from the Swift formulas).
func TestGridLayoutCellPositions(t *testing.T) {
	store, diagram, items := makeDiagram(4, false)
	layout := NewGridLayout()
	applyAll(t, mustOps(layout.Apply(makeContext(store, diagram, items, defaultView), true)))
	eq(t, layout.Cols, 3, "cols")
	eq(t, layout.Rows, 2, "rows")
	want := [][2]float64{{0.75, 30.75}, {30.75, 30.75}, {60.75, 30.75}, {0.75, 0.75}}
	if got := positions(items); !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestGridLayoutWithNon1AspectRatioUsesACeilSqrtSquare(t *testing.T) {
	layout := NewGridLayout()
	layout.AspectRatio = 2
	cols, rows, err := layout.CalculateNumberOfRowsAndCols(5, 0, 0, 100, 100)
	must(t, err)
	eq(t, [2]int{cols, rows}, [2]int{3, 3}, "cols, rows")
}

// Zero-width bounds trap in Swift (Int of infinity); TS throws RangeError instead of looping.
func TestGridLayoutOnZeroWidthBoundsFailsInsteadOfLooping(t *testing.T) {
	if _, _, err := NewGridLayout().CalculateNumberOfRowsAndCols(3, 0, 0, 0, 100); err == nil {
		t.Fatal("expected an error")
	}
}

func TestSpringLayoutProducesPositionOperations(t *testing.T) {
	store, diagram, items := makeDiagram(5, true)
	ops := mustOps(NewSpringLayout(func() float64 { return 0 }, nil).Apply(makeContext(store, diagram, items, defaultView), true))
	eq(t, len(ops), len(items), "ops")
}

func TestSpringLayoutIsNoOpWhenNotClean(t *testing.T) {
	store, diagram, items := makeDiagram(5, true)
	ops := mustOps(NewSpringLayout(func() float64 { return 0 }, nil).Apply(makeContext(store, diagram, items, defaultView), false))
	eq(t, len(ops), 0, "ops")
}

func TestSpringLayoutKeepsItemsWithinFiniteCoordinates(t *testing.T) {
	store, diagram, items := makeDiagram(6, true)
	applyAll(t, mustOps(NewSpringLayout(func() float64 { return 0 }, nil).Apply(makeContext(store, diagram, items, defaultView), true)))
	for _, i := range items {
		if math.IsNaN(i.X) || math.IsInf(i.X, 0) || math.IsNaN(i.Y) || math.IsInf(i.Y, 0) {
			t.Fatalf("not finite: %v %v", i.X, i.Y)
		}
	}
}

func TestSpringLayoutOnSingleNode(t *testing.T) {
	store, diagram, items := makeDiagram(1, false)
	ops := mustOps(NewSpringLayout(func() float64 { return 0 }, nil).Apply(makeContext(store, diagram, items, defaultView), true))
	eq(t, len(ops), 1, "ops")
}

func TestTreeLayoutIsStillAStub(t *testing.T) {
	store, diagram, items := makeDiagram(3, true)
	ops := mustOps(TreeLayout{}.Apply(makeContext(store, diagram, items, defaultView), true))
	eq(t, len(ops), 0, "ops")
}

// Beyond the Swift tests

func TestSpringLayoutIsDeterministicWithAFixedClock(t *testing.T) {
	run := func() [][2]float64 {
		store, diagram, items := makeDiagram(5, true)
		applyAll(t, mustOps(NewSpringLayout(func() float64 { return 0 }, nil).Apply(makeContext(store, diagram, items, defaultView), true)))
		return positions(items)
	}
	if !slices.Equal(run(), run()) {
		t.Fatal("runs differ")
	}
}

func TestSpringLayoutStopsIteratingOnceTheClockPassesMaxTimeMS(t *testing.T) {
	store, diagram, items := makeDiagram(3, true)
	reads := 0
	layout := NewSpringLayout(func() float64 { // the first read is the start time, then a full budget has passed
		reads++
		if reads == 1 {
			return 0
		}
		return 1
	}, nil)
	_, err := layout.Apply(makeContext(store, diagram, items, defaultView), true)
	must(t, err)
	eq(t, layout.Iteration, 1001.0, "iteration")
}

func TestSpringLayoutRandomPlacementUsesTheInjectedSource(t *testing.T) {
	store, diagram, items := makeDiagram(4, false)
	draws := []float64{0.1, 0.2, 0.3, 0.4}
	n := 0
	layout := NewSpringLayout(func() float64 { return 0 }, func() float64 { n++; return draws[n-1] })
	layout.SprRandom = true
	layout.SprIterations = 0
	layout.FitWithinBoundsEnabled = false
	applyAll(t, mustOps(layout.Apply(makeContext(store, diagram, items, Rect{0, 0, 100, 200}), true)))
	want := [][2]float64{{0, 0}, {100, 200}, {10, 40}, {30, 80}}
	if got := positions(items); !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDrand48MatchesThePOSIXDefaultSeedSequence(t *testing.T) {
	r := MakeDrand48(DefaultDrand48Seed)
	// Values printed by the C library's unseeded drand48() on macOS.
	got := [3]float64{r(), r(), r()}
	eq(t, got, [3]float64{0.39646477376027534, 0.84048536941142515, 0.35333609724524351}, "sequence")
}

func TestLayoutOperationsApplyThroughTheStoreAndUndoRestoresPositions(t *testing.T) {
	store, diagram, items := makeDiagram(4, false)
	before := positions(items)
	um := &UndoManager{}
	ops := mustOps(NewGridLayout().Apply(makeContext(store, diagram, items, defaultView), true))
	must(t, store.CompositeOperation(diagram, um, noop, ops))
	if slices.Equal(positions(items), before) {
		t.Fatal("positions unchanged")
	}
	must(t, um.Undo())
	if !slices.Equal(positions(items), before) {
		t.Fatal("undo did not restore positions")
	}
	must(t, um.Redo())
	eq(t, items[0].X, 0.75, "redo")
}
