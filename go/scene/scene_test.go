package scene_test

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
)

func load(t *testing.T, file string) *scene.Tree {
	t.Helper()
	tree, err := scene.Load("testdata", file)
	if err != nil {
		t.Fatalf("load %s: %v", file, err)
	}
	return tree
}

func build(t *testing.T, tree *scene.Tree, path string) (*scene.Screen, *gfx.Scene) {
	t.Helper()
	sc := gfx.NewScene()
	var screen *scene.Screen
	var err error
	sc.Update(func(tx *gfx.Tx) { screen, err = tree.Build(tx, nil, path, scene.Options{}) })
	if err != nil {
		t.Fatalf("build %q: %v", path, err)
	}
	sc.Commit()
	return screen, sc
}

func boxes(t *testing.T, file, path string) map[string]scene.Box {
	t.Helper()
	screen, _ := build(t, load(t, file), path)
	m := map[string]scene.Box{}
	for _, b := range screen.Boxes() {
		m[b.Path] = b
	}
	return m
}

func paths(s *scene.Screen) []string {
	var out []string
	for _, b := range s.Boxes() {
		out = append(out, b.Path)
	}
	return out
}

func wantBox(t *testing.T, m map[string]scene.Box, path string, x, y, w, h float64) {
	t.Helper()
	b, ok := m[path]
	if !ok {
		t.Fatalf("no box %q in %v", path, m)
	}
	if b.X != x || b.Y != y || b.W != w || b.H != h {
		t.Errorf("%s = (%v %v %v %v), want (%v %v %v %v)", path, b.X, b.Y, b.W, b.H, x, y, w, h)
	}
}

// loadErrors loads a file that must fail and returns the messages.
func loadErrors(t *testing.T, file string) string {
	t.Helper()
	_, err := scene.Load("testdata", file)
	if err == nil {
		t.Fatalf("load %s: want errors", file)
	}
	return err.Error()
}

func wantIn(t *testing.T, got string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(got, p) {
			t.Errorf("missing %q in:\n%s", p, got)
		}
	}
}

func TestImportShortAndFullPath(t *testing.T) {
	m := boxes(t, "import.tenn", "Screens.Login")
	wantBox(t, m, "ok", 0, 0, 60, 24)      // kit.Button, full path through the import alias
	wantBox(t, m, "cancel", 0, 0, 33, 22)  // short path: nearest Button up the tree is Screens.Button
	wantBox(t, m, "hole", 1, 2, 60, 24)    // pos override of the root; screen y is -pos.y
	wantBox(t, m, "badge.dot", 0, 0, 6, 6) // the library resolves base.Dot with its own import
	if m["badge"].W != 6 {
		t.Errorf("badge hugs its dot: %v", m["badge"])
	}
}

func TestImportScopeSubstitution(t *testing.T) {
	m := boxes(t, "import.tenn", "Screens.Alt")
	wantBox(t, m, "ok", 0, 0, 99, 11) // kit in Alt is lib/other.tenn
}

func TestImportMountedElementPath(t *testing.T) {
	tree := load(t, "import.tenn")
	if _, err := tree.Build(&gfx.Tx{}, nil, "kit", scene.Options{}); err != nil {
		t.Fatalf("mounted file as an element: %v", err)
	}
}

func TestAmbiguousPath(t *testing.T) {
	got := loadErrors(t, "ambiguous.tenn")
	wantIn(t, got,
		"ambiguous path Item: candidates ambiguous.tenn:2:5, ambiguous.tenn:3:5",
		"ambiguous path a_b: candidates ambiguous.tenn:7:5, ambiguous.tenn:8:5")
	if strings.Contains(got, "Wrap.Inner") {
		t.Errorf("unique path reported: %s", got)
	}
}

func TestOverridesAndAt(t *testing.T) {
	m := boxes(t, "overrides.tenn", "Main")
	wantBox(t, m, "c1", 0, 0, 150, 30+2+20+2+9)
	wantBox(t, m, "c1.Head", 0, 0, 150, 30)
	wantBox(t, m, "c1.Foot", 0, 54, 9, 9)
	wantBox(t, m, "c1.Foot.Ok", 0, 0, 9, 9)
	wantBox(t, m, "Card", 0, 0, 100, 10+2+20+2+5)
	// instance of an instance: own override last, the inner one kept
	wantBox(t, m, "c2", 0, 0, 170, 30+2+20+2+9)
	wantBox(t, m, "c2.Head", 0, 0, 170, 30)
	// the instance of Primary button is found by its normalized name
	wantBox(t, m, "pb", 0, 0, 77, 31)
}

func TestReplace(t *testing.T) {
	screen, _ := build(t, load(t, "overrides.tenn"), "Main")
	var got []string
	for _, b := range screen.Boxes() {
		if strings.HasPrefix(b.Path, "c3.") && strings.Count(b.Path, ".") == 1 {
			got = append(got, b.Name)
		}
	}
	if want := "Head New1 New2 Foot"; strings.Join(got, " ") != want {
		t.Errorf("kids of c3 = %v, want %s", got, want)
	}
}

func TestOverrideErrors(t *testing.T) {
	got := loadErrors(t, "override_errors.tenn")
	wantIn(t, got,
		"override_errors.tenn:6:21: ambiguous path Same: candidates override_errors.tenn:3:9, override_errors.tenn:4:9",
		`override_errors.tenn:7:21: path Nope: nothing named "Nope" inside the instance`)
}

func TestSlots(t *testing.T) {
	tree := load(t, "slots.tenn")
	screen, _ := build(t, tree, "Slots")
	kids := func(inst string) string {
		var names []string
		for _, b := range screen.Boxes() {
			if strings.HasPrefix(b.Path, inst+".") && strings.Count(b.Path, ".") == 1 {
				names = append(names, b.Name)
			}
		}
		return strings.Join(names, " ")
	}
	cases := map[string]string{
		"w1": "Title Email Pass", // default slot filled in file order, no fallback
		"w2": "Title Empty",      // nothing passed: the fallback of the default slot
		"w3": "Title help X",     // named slot comes first in the component
		"w4": "Title Empty",      // an empty block for a slot without fallback adds nothing
	}
	for inst, want := range cases {
		if got := kids(inst); got != want {
			t.Errorf("%s kids = %q, want %q", inst, got, want)
		}
	}
}

func TestSlotErrors(t *testing.T) {
	got := loadErrors(t, "slot_errors.tenn")
	wantIn(t, got,
		`slot_errors.tenn:4:27: unknown slot "nosuch" (slots: toolbar, (default))`,
		"slot_errors.tenn:5:27: nodes passed, but the component has no default slot (slots: none)",
		`slot_errors.tenn:6:27: path Missing: nothing named "Missing" inside the instance`)
}

func TestUseCycle(t *testing.T) {
	got := loadErrors(t, "cycle_use.tenn")
	wantIn(t, got,
		`cycle: item "A" (cycle_use.tenn:2:5) -> use B as b (cycle_use.tenn:2:16) -> item "B" (cycle_use.tenn:3:5) -> use A as a (cycle_use.tenn:3:16) -> item "A" (cycle_use.tenn:2:5)`,
		`cycle: item "Self" (cycle_use.tenn:4:5) -> use Self as s (cycle_use.tenn:4:19) -> item "Self" (cycle_use.tenn:4:5)`)
}

func TestImportCycle(t *testing.T) {
	got := loadErrors(t, "cycle_a.tenn")
	wantIn(t, got, "import cycle: cycle_a.tenn:1:1 imports cycle_b.tenn, cycle_b.tenn:1:1 imports cycle_a.tenn")
}

func TestImportOutsideRoot(t *testing.T) {
	wantIn(t, loadErrors(t, "outside.tenn"), `outside.tenn:1:1: import "../outside-the-root.tenn" leaves the workspace root`)
}

func TestRowGapPaddingHug(t *testing.T) {
	m := boxes(t, "layout.tenn", "Row")
	wantBox(t, m, "Bar", 0, 0, 90, 20)
	wantBox(t, m, "Bar.A", 5, 5, 20, 10)
	wantBox(t, m, "Bar.B", 35, 5, 20, 10)
	wantBox(t, m, "Bar.C", 65, 5, 20, 10)
}

func TestColumnAlignCenter(t *testing.T) {
	m := boxes(t, "layout.tenn", "Column")
	wantBox(t, m, "Col", 0, 0, 100, 38)
	wantBox(t, m, "Col.A", 30, 2, 40, 10)
	wantBox(t, m, "Col.B", 20, 16, 60, 20)
}

func TestFill(t *testing.T) {
	m := boxes(t, "layout.tenn", "Fill")
	wantBox(t, m, "Bar.A", 0, 0, 20, 30)
	wantBox(t, m, "Bar.F1", 30, 0, 30, 10)
	wantBox(t, m, "Bar.F2", 70, 0, 30, 10)
}

func TestWrapAndAbsolute(t *testing.T) {
	m := boxes(t, "layout.tenn", "Wrap")
	wantBox(t, m, "Bar", 0, 0, 100, 52)
	wantBox(t, m, "Bar.A", 4, 4, 40, 20)
	wantBox(t, m, "Bar.B", 50, 4, 40, 20)
	wantBox(t, m, "Bar.C", 4, 28, 30, 20)
	wantBox(t, m, "Bar.Badge", 90, -4, 8, 8)
}

func TestJustify(t *testing.T) {
	m := boxes(t, "layout.tenn", "Justify")
	wantBox(t, m, "Between.B", 80, 0, 20, 10)
	wantBox(t, m, "End.A", 80, 20, 20, 10)
}

func TestAbsAndPlainItems(t *testing.T) {
	m := boxes(t, "layout.tenn", "Abs")
	wantBox(t, m, "Free", 0, 0, 35, 45)
	wantBox(t, m, "Free.B", 30, 40, 5, 5)
	wantBox(t, m, "Plain", 7, 8, 50, 60)
}

// The measurer is gfx.EstimateText: 0.6 em a rune, ascent 0.8 em, descent 0.25 em, font-size 18.
func TestTextLeaves(t *testing.T) {
	m := boxes(t, "layout.tenn", "Text")
	wantBox(t, m, "T.L", 0, 0, 22, 19)   // ceil(2 * 10.8), ceil(18 * 1.05): no padding in a layout context
	wantBox(t, m, "T.W", 0, 19, 100, 38) // fill: wraps at 100 into two lines
	if got := strings.Join(m["T.W"].Lines, "|"); got != "aaaa bbbb|cccc" {
		t.Errorf("lines = %q", got)
	}
}

func TestLayoutValidation(t *testing.T) {
	got := loadErrors(t, "layout_errors.tenn")
	wantIn(t, got,
		"layout_errors.tenn:3:9: width fill needs auto-layout on the item or on its parent",
		"layout_errors.tenn:4:22: height hug needs auto-layout on the item or on its parent",
		`layout_errors.tenn:8:9: warning: justify space-around is not supported by the layout`,
		`layout_errors.tenn:9:9: align takes one of start, center, end, got "sideways"`,
		"layout_errors.tenn:10:9: padding takes 1, 2 or 4 numbers, got 3",
		"layout_errors.tenn:11:9: warning: width: percentages are not supported")
}

func TestOrderIsFileOrder(t *testing.T) {
	screen, sc := build(t, load(t, "order.tenn"), "Order")
	if got, want := strings.Join(paths(screen), " "), " C A B Z"; got != want {
		t.Errorf("boxes = %q, want %q", got, want)
	}
	var ids []int
	for _, c := range screen.Root.Children() {
		ids = append(ids, c.ID())
	}
	// C, A, the link, B, Z: ids are given in file order and the scene keeps the order of adding
	if fmt.Sprint(ids) != "[2 3 4 5 6]" {
		t.Errorf("children ids = %v", ids)
	}
	var svg bytes.Buffer
	if err := gfx.WriteSubtreeSVG(&svg, sc, screen.Root, gfx.Rect{}, 1); err != nil {
		t.Fatal(err)
	}
	s := svg.String()
	last := -1
	for _, c := range []string{"rgb(0,255,0)", "rgb(255,0,0)", "rgb(0,0,255)"} {
		i := strings.Index(s, c)
		if i < last {
			t.Errorf("%s drawn before an earlier item in:\n%s", c, s)
		}
		last = i
	}
}

func TestSVGDeterministic(t *testing.T) {
	render := func() string {
		screen, sc := build(t, load(t, "layout.tenn"), "Text")
		var b bytes.Buffer
		if err := gfx.WriteSubtreeSVG(&b, sc, screen.Root, gfx.Rect{}, 1); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	a := render()
	if a != render() || !strings.Contains(a, "<text") {
		t.Errorf("not deterministic or no text:\n%s", a)
	}
}

func TestImportAliasTaken(t *testing.T) {
	got := loadErrors(t, "dup_alias.tenn")
	wantIn(t, got, "dup_alias.tenn:2:1: import alias kit is already taken by dup_alias.tenn:1:1")
	if strings.Contains(got, "base") {
		t.Errorf("the default alias (file name) should work:\n%s", got)
	}
}

// docs/Example.tenn is a real brain map: names are the texts, pos has Y up.
func TestExampleFile(t *testing.T) {
	tree, err := scene.Load("../../docs", "Example.tenn")
	if err != nil {
		t.Fatal(err)
	}
	screen, _ := build(t, tree, "Documentation.Basic_steps")
	src, err := os.ReadFile("../../docs/Example.tenn")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	from := strings.Index(text, `element "Basic steps"`)
	text = text[from : from+strings.Index(text[from:], `element "General info"`)]
	want := len(regexp.MustCompile(`(?m)^\s+item "`).FindAllString(text, -1))
	bs := screen.Boxes()[1:]
	if len(bs) != want || want == 0 {
		t.Fatalf("boxes = %d, items = %d", len(bs), want)
	}
	byName := map[string]scene.Box{}
	for _, b := range bs {
		if b.Name == "ColorAnnotation" { // title "" on purpose, its text is a markdown body
			continue
		}
		if len(b.Lines) == 0 || strings.TrimSpace(strings.Join(b.Lines, "")) == "" {
			t.Errorf("box %q has no text", b.Name)
		}
		byName[b.Name] = b
	}
	item, newItem, adding := byName["Item"], byName["New Item"], byName["Adding new items"]
	if !(newItem.X > item.X) || newItem.Y != item.Y { // pos -411 274 against -572 274
		t.Errorf("New Item %+v should be right of Item %+v on the same line", newItem, item)
	}
	if !(adding.Y < item.Y) { // pos y 358 against 274: higher on the screen
		t.Errorf("Adding new items %+v should be above Item %+v", adding, item)
	}
}

func wantSize(t *testing.T, m map[string]scene.Box, path string, w, h float64) {
	t.Helper()
	if b, ok := m[path]; !ok || b.W != w || b.H != h {
		t.Errorf("%s = %+v, want size %vx%v", path, b, w, h)
	}
}

func TestNestedPaths(t *testing.T) {
	m := boxes(t, "nested.tenn", "Screen")
	wantBox(t, m, "plain.list.r1.Cell", 0, 0, 10, 10)
	wantBox(t, m, "deep.list.r1.Cell", 0, 0, 20, 10) // fallback of a slot two instances down
	wantBox(t, m, "deep.list.r2.Own", 0, 0, 7, 7)    // another instance of the same component is untouched
	wantSize(t, m, "deep.Hint", 4, 3)                // fallback of the default slot: segment "default"
	wantBox(t, m, "swap.list.r1.New", 0, 0, 6, 6)    // replace inside a fallback
	wantBox(t, m, "fill.list.r1.Mine", 0, 0, 5, 5)   // slot of a nested instance filled from outside
	wantSize(t, m, "fill.Extra", 2, 2)               // default nodes go to the own slot of the component
	wantBox(t, m, "chain2.list.r2.Own", 0, 0, 9, 7)  // path through an instance of an instance
	if _, ok := m["fill.list.r1.Cell"]; ok {
		t.Errorf("filled slot keeps its fallback")
	}
}

func TestNestedPathErrors(t *testing.T) {
	got := loadErrors(t, "nested_errors.tenn")
	wantIn(t, got,
		"nested_errors.tenn:8:27: path r1.cells names a slot, not a node",
		`nested_errors.tenn:9:27: path r1.nosuch: nothing named "nosuch" inside the instance`,
		`nested_errors.tenn:10:27: path cells.Cell: nothing named "cells" inside the instance`,
		"nested_errors.tenn:11:27: nodes passed, but the component has no default slot (slots: none)",
		"nested_errors.tenn:12:26: ambiguous path cells: candidates nested_errors.tenn:4:18, nested_errors.tenn:4:36")
}

func TestNestedFallbackDropped(t *testing.T) {
	tree, err := scene.Load("testdata", "nested_warn.tenn")
	if err != nil {
		t.Fatal(err)
	}
	wantIn(t, tree.Diagnostics().Error(), "nested_warn.tenn:9:17: warning: slot cells is filled here")
}
