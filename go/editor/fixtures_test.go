package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
)

// Every fixture the model tests read (the files Swift saved) is opened, edited in each way a user can, and
// the edit undone: the text must come back to the byte, whatever the file holds.
func TestFixturesEditAndUndo(t *testing.T) {
	files, err := filepath.Glob("../../web/fixtures/*.tenn")
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	edited := 0
	for _, file := range files {
		if strings.HasSuffix(file, ".saved.tenn") {
			continue
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		m := model.ReadTenn(string(raw))
		if m == nil {
			continue // fixtures of parse errors
		}
		var walk func(e *model.Element)
		walk = func(e *model.Element) {
			if len(e.Items) > 0 {
				edited++
				editElement(t, filepath.Base(file)+"/"+e.Name, e)
			}
			for _, c := range e.Elements {
				walk(c)
			}
		}
		walk(&m.Element)
	}
	if edited < 20 {
		t.Errorf("only %d elements with items were edited", edited)
	}
}

func editElement(t *testing.T, name string, e *model.Element) {
	t.Helper()
	s, err := New(e, Options{
		Render:  render.Options{Measurer: gfx.EstimateText{}, NoEvaluate: true},
		OnError: func(err error) { t.Errorf("%s: store error: %v", name, err) },
	})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	before := s.Text()
	steps := map[string]func(){
		"move":      func() { s.SelectAll(""); s.MoveBy(1, 0) },
		"delete":    func() { s.SelectAll(""); s.DeleteSelection() },
		"duplicate": func() { s.SelectAll(""); s.Duplicate() },
		"tab":       func() { s.Select(s.element.Items[:1]); s.AddNewItem(true) },
		"paste":     func() { s.SelectAll(""); s.Paste(s.CopyText()) },
		"align":     func() { s.SelectAll(model.ItemKindItem); s.Align("leading") },
		"drag": func() {
			c := s.scene.Drawables[s.element.Items[0]].SelectorBounds()
			p := render.Point{X: c.X + 1, Y: c.Y + 1}
			s.MouseDown(MouseEvent{P: p})
			s.MouseMove(MouseEvent{P: render.Point{X: p.X + 20, Y: p.Y - 20}})
			s.MouseUp(MouseEvent{P: render.Point{X: p.X + 20, Y: p.Y - 20}})
		},
		"layout": func() { s.TestLayout(render.Rect{X: -400, Y: -300, Width: 800, Height: 600}) },
		"style":  func() { s.Select(s.element.Items[:1]); s.SetQuickStyle("color", "red") },
	}
	for step, run := range steps {
		s.Select(nil)
		run()
		s.EndEvent()
		for s.UndoManager().CanUndo() {
			s.Undo()
		}
		if got := s.Text(); got != before {
			t.Errorf("%s: %s then undo changed the text:\n%s\n--\n%s", name, step, got, before)
			return
		}
	}
}

func TestSessionOnFixtureWithExpressions(t *testing.T) {
	raw, err := os.ReadFile("../../web/fixtures/calc-arith.tenn")
	if err != nil {
		t.Fatal(err)
	}
	m := model.ReadTenn(string(raw))
	var el *model.Element
	for _, c := range m.Elements {
		if len(c.Items) > 0 {
			el = c
			break
		}
	}
	s, err := New(el, Options{Render: render.Options{Measurer: gfx.EstimateText{}}})
	if err != nil {
		t.Fatal(err)
	}
	s.SelectAll("")
	s.Duplicate()
	s.Undo()
	if len(s.element.Items) != len(el.Items) || s.Err() != nil {
		t.Errorf("items %d, err %v", len(s.element.Items), s.Err())
	}
}
