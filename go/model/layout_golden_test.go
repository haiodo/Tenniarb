package model

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// Port of web/packages/core/test/layout-golden.test.ts. Both layouts must match the Swift output bit for bit.

type goldenPos struct {
	Item int
	X, Y float64
}

type goldenLayout struct {
	Elements []struct {
		Path        []int
		Name        string
		SceneBounds Rect
		Nodes       []struct {
			Item   int
			Name   string
			X, Y   float64
			Bounds Rect
		}
		Edges []struct {
			Item   int
			Source *int
			Target *int
		}
		Grid struct {
			ViewBounds Rect
			Positions  []goldenPos
		}
		Spring struct {
			ViewBounds Rect
			Params     struct {
				Iterations  int
				MaxTimeMS   float64
				Random      bool
				Move        float64
				Strain      float64
				Length      float64
				Gravitation float64
			}
			Positions []goldenPos
		}
	}
}

type goldenScene struct {
	bounds Rect
	sizes  map[*DiagramItem]Rect
}

func (s goldenScene) GetBounds() Rect { return s.bounds }

func (s goldenScene) GetItemBounds(n *DiagramItem) (Rect, bool) {
	r, ok := s.sizes[n]
	return r, ok
}

func elementAt(model *ElementModel, path []int) *Element {
	e := &model.Element
	for _, i := range path {
		e = e.Elements[i]
	}
	return e
}

func goldenPositions(ops []ElementOperation, e *Element) []goldenPos {
	out := []goldenPos{}
	for _, op := range ops {
		up := op.(*UpdatePosition)
		out = append(out, goldenPos{slices.Index(e.Items, up.Item), up.NewValue.X, up.NewValue.Y})
	}
	return out
}

func hasItems(e *Element) bool {
	return slices.ContainsFunc(e.Items, func(i *DiagramItem) bool { return i.Kind == ItemKindItem }) ||
		slices.ContainsFunc(e.Elements, hasItems)
}

func checkPositions(t *testing.T, actual, expected []goldenPos, what string) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("%s: position count got %d, want %d", what, len(actual), len(expected))
	}
	for i, a := range actual {
		if a != expected[i] {
			t.Fatalf("%s[%d]: got %+v, want %+v", what, i, a, expected[i])
		}
	}
}

func indexOrNeg(e *Element, i *DiagramItem) *int {
	if i == nil {
		return nil
	}
	k := slices.Index(e.Items, i)
	return &k
}

func TestLayoutGolden(t *testing.T) {
	ran := 0
	for _, f := range fixtureNames(t) {
		model := ReadTenn(strings.TrimPrefix(readFile(t, f), "\xef\xbb\xbf"))
		// Swift records a layout golden for every error-free document that has an Item; a missing file fails the test.
		if model == nil || !hasItems(&model.Element) {
			continue
		}
		ran++
		t.Run(strings.TrimSuffix(f, ".tenn"), func(t *testing.T) {
			var golden goldenLayout
			if err := json.Unmarshal([]byte(readFile(t, strings.TrimSuffix(f, ".tenn")+".layout.json")), &golden); err != nil {
				t.Fatal(err)
			}
			store := NewElementModelStore(model)

			for _, ev := range golden.Elements {
				e := elementAt(model, ev.Path)
				where := fmt.Sprintf("%s %v", ev.Name, ev.Path)
				sizes := map[*DiagramItem]Rect{}
				for _, n := range ev.Nodes {
					sizes[e.Items[n.Item]] = n.Bounds
				}
				scene := goldenScene{ev.SceneBounds, sizes}

				lc := NewLayoutContext(e, scene, store, ev.Grid.ViewBounds)
				if len(lc.Nodes) != len(ev.Nodes) {
					t.Fatalf("%s: node count got %d, want %d", where, len(lc.Nodes), len(ev.Nodes))
				}
				for i, n := range lc.Nodes {
					w := ev.Nodes[i]
					if slices.Index(e.Items, n) != w.Item || n.Name != w.Name || n.X != w.X || n.Y != w.Y {
						t.Fatalf("%s: node %d differs", where, i)
					}
				}
				if len(lc.Edges) != len(ev.Edges) {
					t.Fatalf("%s: edge count got %d, want %d", where, len(lc.Edges), len(ev.Edges))
				}
				for i, l := range lc.Edges {
					w := ev.Edges[i]
					s, tg := indexOrNeg(e, l.Source), indexOrNeg(e, l.Target)
					if slices.Index(e.Items, l) != w.Item || fmt.Sprint(deref(s)) != fmt.Sprint(deref(w.Source)) || fmt.Sprint(deref(tg)) != fmt.Sprint(deref(w.Target)) {
						t.Fatalf("%s: edge %d differs", where, i)
					}
				}

				ops, err := NewGridLayout().Apply(lc, true)
				if err != nil {
					t.Fatalf("%s grid: %v", where, err)
				}
				checkPositions(t, goldenPositions(ops, e), ev.Grid.Positions, where+" grid")

				sl := NewSpringLayout(nil, nil)
				sl.MaxTimeMS = ev.Spring.Params.MaxTimeMS
				p := ev.Spring.Params
				if sl.SprIterations != p.Iterations || sl.SprRandom != p.Random || sl.SprMove != p.Move ||
					sl.SprStrain != p.Strain || sl.SprLength != p.Length || sl.SprGravitation != p.Gravitation {
					t.Fatalf("%s: spring params differ", where)
				}
				slc := NewLayoutContext(e, scene, store, ev.Spring.ViewBounds)
				ops, err = sl.Apply(slc, true)
				if err != nil {
					t.Fatalf("%s spring: %v", where, err)
				}
				checkPositions(t, goldenPositions(ops, e), ev.Spring.Positions, where+" spring")
			}
		})
	}
	if ran == 0 {
		t.Fatal("no fixture with items")
	}
}

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}
