package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/haiodo/gowt/gfx"
)

const sample = `element "A" {
    element "B" {
        item "x" { pos 0 0 }
    }
    element "B" {
        item "y" { pos 0 0 }
    }
}
element "C" {
    item "z" { pos 0 0 }
}
`

func TestInitialElementIsRoot(t *testing.T) {
	s := NewState(ParseText(sample))
	if s.Current == nil || s.Current != &s.Doc.Model.Element || s.Error != "" {
		t.Fatalf("current=%v err=%q", s.Current, s.Error)
	}
}

func TestPathRoundTrip(t *testing.T) {
	doc := ParseText(sample)
	a := doc.Model.Elements[0]
	second := a.Elements[1]
	p := PathOf(second)
	if len(p) != 2 || p[0] != "A" || p[1] != "B#1" {
		t.Fatalf("path %v", p)
	}
	if got := Resolve(&doc.Model.Element, p); got != second {
		t.Fatalf("resolved %v", got)
	}
	if got := Resolve(&doc.Model.Element, nil); got != &doc.Model.Element {
		t.Fatal("empty path is the root")
	}
	if Resolve(&doc.Model.Element, []string{"A", "B#2"}) != nil || Resolve(&doc.Model.Element, []string{"nope"}) != nil {
		t.Fatal("missing element resolved")
	}
}

func TestReloadKeepsSelectionAndCameras(t *testing.T) {
	s := NewState(ParseText(sample))
	sel := s.Doc.Model.Elements[0].Elements[1]
	s.Current = sel
	cam := gfx.Camera{Origin: gfx.Point{X: 5, Y: 6}, Zoom: 2}
	s.Remember(sel, cam)

	s.Reload(ParseText(sample))
	if s.Current == sel || len(PathOf(s.Current)) != 2 || PathOf(s.Current)[1] != "B#1" {
		t.Fatalf("selection lost: %v", PathOf(s.Current))
	}
	if got, ok := s.Camera(s.Current); !ok || got != cam {
		t.Fatalf("camera %v %v", got, ok)
	}
}

func TestReloadElementGoneFallsBackToRoot(t *testing.T) {
	s := NewState(ParseText(sample))
	s.Current = s.Doc.Model.Elements[1]
	s.Reload(ParseText(`element "A" { item "q" { pos 0 0 } }`))
	if s.Current != &s.Doc.Model.Element {
		t.Fatalf("current %v", PathOf(s.Current))
	}
}

func TestReloadParseErrorKeepsLastModel(t *testing.T) {
	s := NewState(ParseText(sample))
	good := s.Doc
	s.Current = good.Model.Elements[1]
	bad := ParseText("element \"A\" {")
	if bad.Model != nil || len(bad.Errors) == 0 {
		t.Fatal("expected parse errors")
	}
	s.Reload(bad)
	if s.Doc != good || s.Current != good.Model.Elements[1] || s.Error == "" {
		t.Fatalf("doc kept=%v err=%q", s.Doc == good, s.Error)
	}
	s.Reload(ParseText(sample))
	if s.Error != "" {
		t.Fatal("error not cleared by a good read")
	}
}

func TestOpenWithErrorsHasNoCurrent(t *testing.T) {
	s := NewState(ParseText("element {"))
	if s.Current != nil || s.Error == "" {
		t.Fatalf("current=%v err=%q", s.Current, s.Error)
	}
}

func TestWatcher(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.tenn")
	if err := os.WriteFile(p, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := Load(p)
	if err != nil || doc.Model == nil {
		t.Fatalf("load: %v %v", err, doc)
	}
	w := &Watcher{Path: p, Last: doc.Stamp}
	if w.Poll() {
		t.Fatal("unchanged file reported")
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(p, later, later); err != nil {
		t.Fatal(err)
	}
	if !w.Poll() || w.Poll() {
		t.Fatal("touch must report once")
	}
	os.Remove(p)
	if w.Poll() {
		t.Fatal("a missing file is not a change")
	}
}

func TestBOMAndFixture(t *testing.T) {
	if doc := ParseText("\xef\xbb\xbf" + sample); doc.Model == nil {
		t.Fatalf("BOM: %v", doc.Errors)
	}
	doc, err := Load("../../web/fixtures/Example.tenn")
	if err != nil || doc.Model == nil || len(doc.Model.Elements) == 0 {
		t.Fatalf("Example.tenn: %v %v", err, doc)
	}
}

func TestViewportMath(t *testing.T) {
	cam := FitCamera(400, 200, 800, 600)
	if cam.Zoom != 1 || cam.Origin.X != -200 || cam.Origin.Y != -200 {
		t.Fatalf("small content centered at 1: %+v", cam)
	}
	cam = FitCamera(1600, 600, 800, 600)
	if cam.Zoom != 0.5 {
		t.Fatalf("large content fits: %+v", cam)
	}
	z := ZoomCamera(gfx.Camera{Origin: gfx.Point{X: 10, Y: 20}, Zoom: 1}, 2, 100, 50)
	// the scene point under (100,50) stays put
	if sx := z.Origin.X + 100/z.Zoom; sx != 110 {
		t.Fatalf("anchor moved: %+v", z)
	}
	if ZoomCamera(gfx.Camera{Zoom: 60}, 10, 0, 0).Zoom != MaxZoom {
		t.Fatal("zoom not clamped")
	}
}
