package editor

import (
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/tenn"
)

// canonical is what go/model persistence writes for a document: the expected text of a scenario is written
// by hand, read and printed again, so the session is compared with the model's own writer.
func canonical(t *testing.T, doc string) string {
	t.Helper()
	m := model.ReadTenn(doc)
	if m == nil {
		t.Fatalf("expected document does not parse:\n%s", doc)
	}
	return model.ToTennStr(&m.Element, true, true)
}

// scenario runs act on a fresh fixture and checks that the saved text means want (hand-written; both go
// through the model's reader and writer, which keep floats as Swift Float, so 29.4 reads back as
// 29.399999618530273), that the reader and writer then agree, that the host was given the saved text, and
// that one Undo restores the original byte for byte and one Redo the edit.
func scenario(t *testing.T, want func(f *fixture) string, act func(f *fixture)) {
	t.Helper()
	f := newFixture(t)
	before := f.s.Text()
	if before != canonical(t, testSrc) {
		t.Fatalf("fixture text is not canonical:\n%s", before)
	}
	act(f)
	after := f.s.Text()
	if after == before {
		t.Fatal("the scenario changed nothing")
	}
	norm := canonical(t, after)
	if w := canonical(t, want(f)); norm != w {
		t.Errorf("saved text:\n%s\nwant:\n%s", after, w)
	}
	if again := canonical(t, norm); again != norm {
		t.Errorf("saved text is not stable under read and write:\n%s\n--\n%s", norm, again)
	}
	if n := len(f.changes); n == 0 || f.changes[n-1] != after {
		t.Errorf("OnChange got %d texts, the last is not the saved one", n)
	}
	f.s.Undo()
	if got := f.s.Text(); got != before {
		t.Errorf("undo:\n%s\nwant:\n%s", got, before)
	}
	if f.s.UndoManager().CanUndo() {
		t.Error("the scenario took more than one undo step")
	}
	f.s.Redo()
	if got := f.s.Text(); got != after {
		t.Errorf("redo:\n%s\nwant:\n%s", got, after)
	}
}

// doc is testSrc with extra lines at the end of the element, and the lines of the given items replaced.
func doc(extra string, replace ...string) string {
	s := testSrc
	for i := 0; i+1 < len(replace); i += 2 {
		s = strings.Replace(s, replace[i], replace[i+1], 1)
	}
	return strings.TrimSuffix(s, "}") + extra + "\n}"
}

func num(v float64) string { return tenn.SwiftDouble(v) }

func TestScenarioTabAddsLinkedItem(t *testing.T) {
	scenario(t, func(f *fixture) string {
		x := num(f.a.X + f.s.scene.Drawables[f.a].Bounds().Width + 10)
		return doc(`item "Untitled 1" { pos ` + x + ` 0 }
link "A" "Untitled 1"`)
	}, func(f *fixture) {
		f.click(f.centre(f.a), 0)
		f.key("Tab", 0)
		if sel := f.s.Selection(); len(sel) != 1 || sel[0].Name != "Untitled 1" {
			t.Errorf("the new item must be selected: %v", sel)
		}
	})
}

func TestScenarioTabWithoutSelectionAddsItemAtPivot(t *testing.T) {
	scenario(t, func(f *fixture) string { return doc(`item "Untitled 1" { pos 300 -300 }`) }, func(f *fixture) {
		// A click on empty space moves the pivot there.
		p := render.Point{X: 300, Y: -300}
		f.s.MouseDown(MouseEvent{P: p})
		f.s.MouseUp(MouseEvent{P: p})
		f.key("Tab", 0)
	})
}

func TestScenarioOptionTabCopiesProperties(t *testing.T) {
	f := newFixture(t)
	f.s.ApplyProps(Target{Item: f.a}, "name \"A\"\ncolor red")
	f.s.Select([]*model.DiagramItem{f.a})
	f.key("Tab", ModAlt)
	items := f.s.element.Items
	added := items[len(items)-2]
	if added.Name != "Untitled 1" || !strings.Contains(f.props(added), "color red") {
		t.Errorf("the copy must have the style: %q", f.props(added))
	}
	norm := canonical(t, f.s.Text())
	if canonical(t, norm) != norm {
		t.Error("saved text is not stable under read and write")
	}
	if strings.Count(norm, "color red") != 2 {
		t.Errorf("the item and its copy have the color:\n%s", norm)
	}
}

func TestScenarioDuplicate(t *testing.T) {
	scenario(t, func(f *fixture) string {
		// The copy of B has B's name, so the links to it are told apart by index.
		return doc(`item "B" { pos 275 0 }
link "A" "B" {
  display arrow
  target-index 1
}`)
	}, func(f *fixture) {
		f.click(f.centre(f.b), 0)
		f.key("d", ModMeta)
		if len(f.s.Selection()) != 2 {
			t.Errorf("the copy and its link must be selected: %v", f.s.Selection())
		}
	})
}

func TestScenarioDelete(t *testing.T) {
	scenario(t, func(f *fixture) string {
		return `element "D" {
  item "A" { pos 0 0 }
  item "C" { pos 0 -150 }
}`
	}, func(f *fixture) {
		f.click(f.centre(f.b), 0)
		f.key("Backspace", 0)
		if len(f.s.Selection()) != 0 {
			t.Errorf("selection after delete: %v", f.s.Selection())
		}
	})
}

func TestScenarioDragMove(t *testing.T) {
	scenario(t, func(f *fixture) string {
		return doc("", `item "A" { pos 0 0 }`, `item "A" { pos 40 -25 }`)
	}, func(f *fixture) {
		p := f.centre(f.a)
		f.drag(p, render.Point{X: p.X + 40, Y: p.Y - 25}, 0)
	})
}

func TestScenarioCtrlDragLink(t *testing.T) {
	scenario(t, func(f *fixture) string { return doc(`link "A" "C"`) }, func(f *fixture) {
		f.drag(f.centre(f.a), f.centre(f.c), ModCtrl)
	})
}

func TestScenarioOptionDragMovesSubtree(t *testing.T) {
	scenario(t, func(f *fixture) string {
		return doc("", `item "B" { pos 200 0 }`, `item "B" { pos 230 10 }`, `item "C" { pos 0 -150 }`, `item "C" { pos 30 -140 }`)
	}, func(f *fixture) {
		p := f.centre(f.b)
		f.drag(p, render.Point{X: p.X + 30, Y: p.Y + 10}, ModAlt)
	})
}

func TestScenarioRename(t *testing.T) {
	scenario(t, func(f *fixture) string {
		return strings.ReplaceAll(strings.ReplaceAll(testSrc, `item "A"`, `item "Alpha"`), `link "A"`, `link "Alpha"`)
	}, func(f *fixture) {
		f.s.CommitEdit(f.a, EditName, "Alpha")
	})
}

func TestScenarioEditBody(t *testing.T) {
	scenario(t, func(f *fixture) string {
		return doc("", `item "A" { pos 0 0 }`, `item "A" { body "hello" }`)
	}, func(f *fixture) {
		tg, ok := f.s.BeginEdit(f.a, EditBody)
		if !ok || tg.Text != "" {
			t.Fatalf("target %v %v", tg, ok)
		}
		f.s.FinishEdit("hello", true)
	})
}

func TestScenarioEditMultilineBody(t *testing.T) {
	scenario(t, func(f *fixture) string {
		return doc("", `item "A" { pos 0 0 }`, "item \"A\" { body %{one\ntwo} }")
	}, func(f *fixture) {
		f.s.CommitEdit(f.a, EditBody, "one\ntwo")
	})
}

func TestScenarioEditValue(t *testing.T) {
	scenario(t, func(f *fixture) string {
		return doc("", `item "A" { pos 0 0 }`, `item "A" { value 42 }`)
	}, func(f *fixture) {
		f.s.CommitEdit(f.a, EditValue, "42")
	})
}

func TestScenarioStyleChange(t *testing.T) {
	scenario(t, func(f *fixture) string {
		return doc("", `item "B" { pos 200 0 }`, `item "B" { pos 200 0
 color red }`)
	}, func(f *fixture) {
		f.click(f.centre(f.b), 0)
		// The way the menu does it.
		for _, q := range QuickStylesFor(model.ItemKindItem) {
			if q.Prop == "color" {
				for _, e := range f.s.QuickEntries(q) {
					if e.Label == "🔴red" {
						e.Run()
					}
				}
			}
		}
	})
}

func TestScenarioNamedStyle(t *testing.T) {
	scenario(t, func(f *fixture) string {
		return strings.Replace(doc("", `item "C" { pos 0 -150 }`, `item "C" { pos 0 -150
 use-style new_style_1 }`), `item "A"`, `styles { new_style_1 { color "white" } }
  item "A"`, 1)
	}, func(f *fixture) {
		f.s.Batch(func() {
			name := f.s.DefineStyle()
			f.s.Select([]*model.DiagramItem{f.c})
			f.s.ApplyStyle(name)
		})
	})
}

func TestScenarioMoveByArrows(t *testing.T) {
	scenario(t, func(f *fixture) string {
		return doc("", `item "A" { pos 0 0 }`, `item "A" { pos 5 0 }`)
	}, func(f *fixture) {
		f.s.Select([]*model.DiagramItem{f.a})
		f.key("ArrowRight", 0)
	})
}

func TestScenarioPasteKeepsSavedFormat(t *testing.T) {
	scenario(t, func(f *fixture) string {
		return doc(`item "X" { pos 10 10 }`)
	}, func(f *fixture) {
		if !f.s.Paste("item \"X\" {\n  pos 10 10\n}") {
			t.Fatal("paste")
		}
	})
}

func TestScenarioPropsPanelEdit(t *testing.T) {
	scenario(t, func(f *fixture) string {
		return doc("", `item "A" { pos 0 0 }`, `item "A" { pos 0 0
 font-size 20 }`)
	}, func(f *fixture) {
		if !f.s.ApplyProps(Target{Item: f.a}, "name \"A\"\npos 0 0\nfont-size 20") {
			t.Fatal("apply")
		}
	})
}
