package exec

import (
	"math"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/tenn"
)

// Port of web/packages/core/test/execution.test.ts.

func exprNode(literal string) *tenn.Node {
	return tenn.NewNode(tenn.KindExpression, tenn.NewToken(tenn.TokExpression, literal))
}

func blockNode(literal string) *tenn.Node {
	return tenn.NewNode(tenn.KindExpressionBlock, tenn.NewToken(tenn.TokExpressionBlock, literal))
}

func item(name string, props ...*tenn.Node) *model.DiagramItem {
	itm := model.NewDiagramItem(model.ItemKindItem, name)
	itm.Properties.AppendContentsOf(props)
	return itm
}

func diagram(items ...*model.DiagramItem) *model.Element {
	element := model.NewElement("Diag")
	for _, itm := range items {
		element.AddItem(itm)
	}
	return element
}

func run(element *model.Element, opts ...Options) *ExecutionContext {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	ctx := NewExecutionContext(o)
	ctx.SetElement(element)
	return ctx
}

func ev(ctx *ExecutionContext, target model.EvalTarget, n *tenn.Node) any {
	v, ok := ctx.GetEvaluated(target, nil, nil)[n.Token]
	if !ok {
		return nil
	}
	return v
}

func isErr(v any, name string) bool {
	e, ok := v.(Error)
	return ok && e.Name == name
}

func eq(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

// ExecutionContextTests.swift

func TestSimpleIntAssignment(t *testing.T) {
	tok := exprNode("5")
	a := item("A", tenn.NewCommand("x", tok))
	eq(t, ev(run(diagram(a)), a, tok), 5.0)
}

func TestChainedExpression(t *testing.T) {
	x, y := blockNode("var x = 5; x"), exprNode("x + 3")
	a := item("A", tenn.NewCommand("x", x), tenn.NewCommand("y", y))
	ctx := run(diagram(a))
	eq(t, ev(ctx, a, x), 5.0)
	eq(t, ev(ctx, a, y), 8.0)
}

func TestExpressionWithParens(t *testing.T) {
	x, z := blockNode("var x = 5; x"), exprNode("(x * 2)")
	a := item("A", tenn.NewCommand("x", x), tenn.NewCommand("z", z))
	eq(t, ev(run(diagram(a)), a, z), 10.0)
}

func TestSiblingItemReference(t *testing.T) {
	b := item("B", tenn.NewCommand("val", tenn.NewIntNode(42)))
	ref := exprNode("items.B.val")
	a := item("A", tenn.NewCommand("bval", ref))
	eq(t, ev(run(diagram(b, a)), a, ref), 42.0)
}

func TestUpdateOnPositionChange(t *testing.T) {
	tok := exprNode("pos.x + 1")
	a := item("A", tenn.NewCommand("derived", tok))
	a.X = 10
	element := diagram(a)
	ctx := run(element)
	eq(t, ev(ctx, a, tok), 11.0)

	a.X = 20
	event := model.NewModelEvent(model.EventLayout, element)
	event.Items.Set(a, model.EventUpdate)
	ctx.NotifyChanges(event)
	eq(t, ev(ctx, a, tok), 21.0)
}

func TestCircularReferenceDoesNotCrash(t *testing.T) {
	aTok := exprNode("items.B ? (items.B.bval || 0) + 1 : 1")
	bTok := exprNode("items.A ? (items.A.aval || 0) + 1 : 1")
	a := item("A", tenn.NewCommand("aval", aTok))
	b := item("B", tenn.NewCommand("bval", bTok))
	ctx := run(diagram(a, b))
	if ev(ctx, a, aTok) == nil || ev(ctx, b, bTok) == nil {
		t.Fatal("no value")
	}
}

func TestInvalidExpressionReturnsGracefully(t *testing.T) {
	tok := exprNode("this is not js !!!")
	a := item("A", tenn.NewCommand("broken", tok))
	if v := ev(run(diagram(a)), a, tok); !isErr(v, "SyntaxError") {
		t.Fatalf("got %#v", v)
	}
}

func TestInvalidExpressionBlockReturnsGracefully(t *testing.T) {
	tok := blockNode("function( { broken }")
	a := item("A", tenn.NewCommand("x", tok))
	if v := ev(run(diagram(a)), a, tok); !isErr(v, "SyntaxError") {
		t.Fatalf("got %#v", v)
	}
}

func TestGetEvaluatedWithNodeOverride(t *testing.T) {
	a := item("A")
	ctx := run(diagram(a))
	tok := exprNode("2 + 3")
	result := ctx.GetEvaluated(a, tenn.NewBlockExpr(tenn.NewCommand("v", tok)), nil)
	eq(t, result[tok.Token], 5.0)
}

func TestUpdateAllCallsNotifier(t *testing.T) {
	a := item("A", tenn.NewCommand("v", exprNode("1 + 1")))
	ctx := run(diagram(a))
	called := 0
	ctx.UpdateAll(func() { called++ })
	eq(t, called, 1)
}

// Engine behaviour beyond the Swift unit tests (calc-*.tenn goldens cover most of it too)

func TestThrownValueIsStoredAndEvaluationContinues(t *testing.T) {
	t1, t2 := blockNode(`throw "s"`), exprNode("1 + 1")
	a := item("A", tenn.NewCommand("t", t1), tenn.NewCommand("ok", t2))
	ctx := run(diagram(a))
	eq(t, ev(ctx, a, t1), "s")
	eq(t, ev(ctx, a, t2), 2.0)
}

func TestNameIsDeletedBeforeItemIsReEvaluated(t *testing.T) {
	tok := exprNode("sv + 1")
	a := item("S", tenn.NewCommand("sv", tok))
	if v := ev(run(diagram(a)), a, tok); !isErr(v, "ReferenceError") {
		t.Fatalf("got %#v", v)
	}
}

func TestCycleStopsAfter100Iterations(t *testing.T) {
	cTok, dTok := exprNode("(items.D.dval || 0) + 1"), exprNode("(items.C.cval || 0) + 1")
	c := item("C", tenn.NewCommand("cval", cTok))
	d := item("D", tenn.NewCommand("dval", dTok))
	ctx := run(diagram(c, d))
	eq(t, ev(ctx, c, cTok), 200.0)
	eq(t, ev(ctx, d, dTok), 201.0)
}

func TestItemsProxyResolvesConvertedNamesOnly(t *testing.T) {
	spaced, raw := exprNode("items.my_item.val"), exprNode(`items["my item"]`)
	my := item("my item", tenn.NewCommand("val", tenn.NewIntNode(7)))
	a := item("A", tenn.NewCommand("s", spaced), tenn.NewCommand("r", raw))
	ctx := run(diagram(my, a))
	eq(t, ev(ctx, a, spaced), 7.0)
	eq(t, ev(ctx, a, raw), Undefined)
}

func TestTemplateStringsAreEvaluated(t *testing.T) {
	str := tenn.NewStrNode("Hello ${name} ${n + 1}")
	a := item("A", tenn.NewCommand("n", tenn.NewIntNode(3)), tenn.NewCommand("title", str))
	eq(t, ev(run(diagram(a)), a, str), "Hello A 4")
}

func TestFunctionsDeclaredInElementScopeStayVisibleToItems(t *testing.T) {
	define := blockNode("function twice(v) { return v * 2 + k }")
	tok := exprNode("twice(k)")
	element := diagram(item("A", tenn.NewCommand("k", tenn.NewIntNode(5)), tenn.NewCommand("r", tok)))
	element.Properties.Append(tenn.NewCommand("define", define))
	eq(t, ev(run(element), element.Items[0], tok), 15.0)
}

func TestSumByTagInputsOutputsEdges(t *testing.T) {
	sumTok := exprNode(`sum("cost")`)
	sumW := exprNode(`sum("cost", "w")`)
	tagTok := exprNode(`byTag("cost").map(function(i) { return i.name })`)
	outTok := exprNode("outputs().map(function(i) { return i.name })")
	inTok := exprNode("inputs().length")
	edgTok := exprNode("edges().length")
	a := item("A", tenn.NewCommand("cost", tenn.NewIntNode(10)), tenn.NewCommand("s", sumTok), tenn.NewCommand("t", tagTok))
	a.Properties.Append(tenn.NewCommand("o", outTok))
	b := item("B", tenn.NewCommand("cost", tenn.NewIntNode(5)), tenn.NewCommand("w", tenn.NewIntNode(2)))
	b.Properties.Append(tenn.NewCommand("sw", sumW))
	b.Properties.Append(tenn.NewCommand("i", inTok))
	b.Properties.Append(tenn.NewCommand("e", edgTok))
	element := diagram(a, b)
	element.AddItem(model.NewLinkItem(model.ItemKindLink, "", a, b))
	ctx := run(element)
	eq(t, ev(ctx, a, sumTok), 15.0)
	eq(t, ev(ctx, a, tagTok), []any{"A", "B"})
	eq(t, ev(ctx, a, outTok), []any{"B"})
	eq(t, ev(ctx, b, sumW), 2.0)
	eq(t, ev(ctx, b, inTok), 1.0)
	eq(t, ev(ctx, b, edgTok), 1.0)
}

func TestNotifyChangesAddsChangedDependantsToTheEvent(t *testing.T) {
	tok := exprNode("items.B.val + 1")
	a := item("A", tenn.NewCommand("r", tok))
	b := item("B", tenn.NewCommand("val", tenn.NewIntNode(1)))
	element := diagram(a, b)
	ctx := run(element)
	eq(t, ev(ctx, a, tok), 2.0)

	b.Properties.AsNode().Children[0].Children[1] = tenn.NewIntNode(5)
	event := model.NewModelEvent(model.EventStructure, element)
	event.Items.Set(b, model.EventUpdate)
	ctx.NotifyChanges(event)
	eq(t, ev(ctx, a, tok), 6.0)
	op, ok := event.Items.Get(a)
	eq(t, []any{op, ok}, []any{model.EventUpdate, true})
}

func TestRemoveAndAppendEventsUpdateTheItemsProxy(t *testing.T) {
	tok := exprNode("items.B ? items.B.val : -1")
	a := item("A", tenn.NewCommand("r", tok))
	b := item("B", tenn.NewCommand("val", tenn.NewIntNode(1)))
	element := diagram(a, b)
	ctx := run(element)
	eq(t, ev(ctx, a, tok), 1.0)

	element.RemoveItem(b)
	removed := model.NewModelEvent(model.EventStructure, element)
	removed.Items.Set(b, model.EventRemove)
	ctx.NotifyChanges(removed)
	eq(t, ev(ctx, a, tok), -1.0)

	element.AddItem(b)
	added := model.NewModelEvent(model.EventStructure, element)
	added.Items.Set(b, model.EventAppend)
	ctx.NotifyChanges(added)
	eq(t, ev(ctx, a, tok), 1.0)
}

func TestStoreDrivesTheContext(t *testing.T) {
	tok := exprNode("pos.x * 2")
	a := item("A", tenn.NewCommand("d", tok))
	element := diagram(a)
	m := model.NewElementModel()
	m.AddElement(element)
	store := model.NewElementModelStore(m)
	ctx := run(element)
	store.ExecutionContext = ctx
	if err := store.UpdatePosition(a, model.Point{X: 4}, nil, nil); err != nil {
		t.Fatal(err)
	}
	eq(t, ev(ctx, a, tok), 8.0)
}

func TestItemSizeHookDefinesWidthHeightAndDefaults(t *testing.T) {
	w := exprNode("width + defaults.height")
	a := item("A", tenn.NewCommand("r", w))
	ctx := run(diagram(a), Options{ItemSize: func(*model.DiagramItem, model.IExecutionContext) *model.DrawableSize {
		return &model.DrawableSize{Width: 10.5, Height: 4.4}
	}})
	eq(t, ev(ctx, a, w), 15.0)
	undef := exprNode("width")
	b := item("B", tenn.NewCommand("r", undef))
	if v := ev(run(diagram(b)), b, undef); !isErr(v, "ReferenceError") {
		t.Fatalf("got %#v", v)
	}
}

func TestNoEvaluateRunsNothingAndReportsNothing(t *testing.T) {
	tok := exprNode("globalThis.__ran = true")
	str := tenn.NewStrNode("${name}")
	a := item("A", tenn.NewCommand("x", tok), tenn.NewCommand("t", str))
	element := diagram(a)
	ctx := run(element, Options{NoEvaluate: true})
	eq(t, len(ctx.GetEvaluated(a, nil, nil)), 0)
	eq(t, len(ctx.GetEvaluated(element, nil, nil)), 0)
	if ctx.root != nil {
		t.Fatal("a runtime was created")
	}
	called := 0
	ctx.UpdateAll(func() { called++ })
	eq(t, called, 0)
	ctx.NotifyChanges(model.NewModelEvent(model.EventLayout, element))
}

func TestCommandsWithNonIdentifierNamesDoNotBreakTheContext(t *testing.T) {
	tok := exprNode("font_size + 1")
	a := item("A", tenn.NewCommand("font-size", tenn.NewIntNode(10)), tenn.NewCommand("r", tok))
	eq(t, ev(run(diagram(a)), a, tok), 11.0)
}

func TestImplicitGlobalsStayInTheContext(t *testing.T) {
	set, read := blockNode("leaked_y = 3"), exprNode("leaked_y")
	a := item("A", tenn.NewCommand("s", set), tenn.NewCommand("r", read))
	eq(t, ev(run(diagram(a)), a, read), 3.0)
	b := item("B", tenn.NewCommand("r", read))
	if v := ev(run(diagram(b)), b, read); !isErr(v, "ReferenceError") {
		t.Fatalf("got %#v", v)
	}
}

func TestDocumentNamesWinOverHostGlobals(t *testing.T) {
	toks := []*tenn.Node{exprNode("name"), exprNode("status"), exprNode("top")}
	props := []*tenn.Node{tenn.NewCommand("status", tenn.NewIntNode(7)), tenn.NewCommand("top", tenn.NewStrNode("t"))}
	for i, tok := range toks {
		props = append(props, tenn.NewCommand("r"+string(rune('0'+i)), tok))
	}
	a := item("A", props...)
	ctx := run(diagram(a))
	eq(t, []any{ev(ctx, a, toks[0]), ev(ctx, a, toks[1]), ev(ctx, a, toks[2])}, []any{"A", 7.0, "t"})
}

func TestVarPersistsAcrossExpressions(t *testing.T) {
	t1, t2 := blockNode("var shared = 4; 0"), exprNode("shared + 1")
	a := item("A", tenn.NewCommand("x", t1), tenn.NewCommand("y", t2))
	eq(t, ev(run(diagram(a)), a, t2), 5.0)
}

// goja-specific: what the indirect eval and the call stack limit are for.

func TestLetDoesNotCollideBetweenExpressions(t *testing.T) {
	t1, t2 := blockNode("let z = 1; z"), blockNode("let z = 2; z")
	a := item("A", tenn.NewCommand("p", t1), tenn.NewCommand("q", t2))
	ctx := run(diagram(a))
	eq(t, []any{ev(ctx, a, t1), ev(ctx, a, t2)}, []any{1.0, 2.0})
}

func TestRunawayRecursionIsAnError(t *testing.T) {
	tok := blockNode("function f() { return f() + 1 } f()")
	a := item("A", tenn.NewCommand("r", tok))
	if v := ev(run(diagram(a)), a, tok); !isErr(v, "RangeError") {
		t.Fatalf("got %#v", v)
	}
}

func TestConcurrentUseIsSafe(t *testing.T) {
	tok := exprNode("pos.x + 1")
	a := item("A", tenn.NewCommand("d", tok))
	element := diagram(a)
	ctx := run(element)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				ctx.UpdateAll(func() {})
				ctx.GetEvaluated(a, nil, nil)
				ctx.GetEvaluated(a, tenn.NewBlockExpr(tenn.NewCommand("v", exprNode("1 + 1"))), nil)
			}
		}()
	}
	wg.Wait()
	eq(t, ev(ctx, a, tok), 1.0)
}

func TestToStringAndToFloat(t *testing.T) {
	for _, c := range []struct {
		v any
		s string
		f float64
	}{
		{1.5, "1.5", 1.5},
		{1e21, "1e+21", 1e21},
		{1e-7, "1e-7", 1e-7},
		{123456789012345680000.0, "123456789012345680000", 123456789012345680000.0},
		{0.000001, "0.000001", 0.000001},
		{-0.0, "0", 0},
		{[]any{1.0, nil, "x"}, "1,,x", math.NaN()},
		{[]any{}, "", 0},
		{[]any{7.0}, "7", 7},
		{" 0x10 ", " 0x10 ", 16},
		{"", "", 0},
		{"1e3", "1e3", 1000},
		{"12px", "12px", math.NaN()},
		{true, "true", 1},
		{nil, "null", 0},
		{Undefined, "undefined", math.NaN()},
		{map[string]any{}, "[object Object]", math.NaN()},
		{Error{Name: "TypeError", Message: "m"}, "TypeError: m", math.NaN()},
	} {
		if got := ToString(c.v); got != c.s {
			t.Errorf("ToString(%#v) = %q, want %q", c.v, got, c.s)
		}
		if got := ToFloat(c.v); got != c.f && !(math.IsNaN(got) && math.IsNaN(c.f)) {
			t.Errorf("ToFloat(%#v) = %v, want %v", c.v, got, c.f)
		}
	}
}

// ItemSize runs under the lock; it must read the previous round through prev and not deadlock.
func TestItemSizeReadsPreviousRoundThroughView(t *testing.T) {
	expr := exprNode("pos.x + 1")
	a := item("A", tenn.NewCommand("v", expr))
	var seen []any
	done := make(chan *ExecutionContext)
	go func() {
		done <- run(diagram(a), Options{ItemSize: func(it *model.DiagramItem, prev model.IExecutionContext) *model.DrawableSize {
			seen = append(seen, prev.GetEvaluated(it, nil, nil)[expr.Token])
			return nil
		}})
	}()
	select {
	case ctx := <-done:
		eq(t, ev(ctx, a, expr), 1.0)
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock: ItemSize could not read evaluated values")
	}
	// Round 1 sees nothing; the following rounds see the value of the round before.
	if len(seen) < 2 || seen[0] != nil || seen[1] != 1.0 {
		t.Fatalf("seen %#v", seen)
	}
}

func TestRoundLimitInterruptsRunawayLoop(t *testing.T) {
	loop := exprNode("while(true){}")
	ok := exprNode("1 + 1")
	a := item("A", tenn.NewCommand("l", loop))
	b := item("B", tenn.NewCommand("o", ok))
	done := make(chan *ExecutionContext)
	go func() { done <- run(diagram(a, b), Options{RoundLimit: 50 * time.Millisecond}) }()
	select {
	case ctx := <-done:
		v := ev(ctx, a, loop)
		if !isErr(v, "Error") || v.(Error).Message != errRoundLimit {
			t.Fatalf("got %#v", v)
		}
		// The context stays usable: the next round starts with a clean interrupt flag.
		ctx.UpdateAll(nil)
		if v := ev(ctx, a, loop); !isErr(v, "Error") {
			t.Fatalf("after UpdateAll: %#v", v)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("round was not interrupted")
	}
}

func TestRoundLimitDoesNotBreakFastRounds(t *testing.T) {
	tok := exprNode("1 + 1")
	a := item("A", tenn.NewCommand("r", tok))
	ctx := run(diagram(a), Options{RoundLimit: 50 * time.Millisecond})
	time.Sleep(120 * time.Millisecond) // the timer of the first round must be gone
	ctx.UpdateAll(nil)
	eq(t, ev(ctx, a, tok), 2.0)
}
