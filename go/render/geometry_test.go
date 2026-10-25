package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
)

// The golden is the TS renderer's layout of every web/fixtures element with a 0.6 em text measurer
// (testdata/geometry.ts). With the same measurer the two renderers must agree to rounding error.
const geometryTol = 1e-9

type jsonRect struct{ X, Y, Width, Height float64 }

type jsonPoint struct{ X, Y float64 }

type jsonItem struct {
	I        int
	None     bool
	Kind     string
	Bounds   jsonRect
	Selector jsonRect
	Source   *jsonPoint
	Target   *jsonPoint
	Extra    []jsonPoint
}

type jsonScene struct {
	Bounds jsonRect
	Items  []jsonItem
}

func noImages(string) *DecodedImage { return nil }

func rectOf(r Rect) jsonRect { return jsonRect(r) }

func TestGeometryMatchesTS(t *testing.T) {
	raw, err := os.ReadFile("testdata/geometry.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]jsonScene
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	seen := 0
	var walk func(el *model.Element, path string)
	walk = func(el *model.Element, path string) {
		if want, ok := golden[path]; ok {
			seen++
			o := testOpts()
			o.DecodeImage = noImages
			o.NoEvaluate = true // the golden is TS layoutScene(el, null)
			s := Build(el, o)
			compareScene(t, path, s, el, want)
		}
		for _, c := range el.Elements {
			walk(c, path+"/"+c.Name)
		}
	}
	files, _ := filepath.Glob("../../web/fixtures/*.tenn")
	for _, f := range files {
		if strings.HasSuffix(f, ".saved.tenn") {
			continue
		}
		text, _ := os.ReadFile(f)
		m := model.ReadTenn(string(text))
		if m == nil {
			continue
		}
		walk(&m.Element, strings.TrimSuffix(filepath.Base(f), ".tenn"))
	}
	if seen != len(golden) {
		t.Errorf("compared %d of %d golden scenes", seen, len(golden))
	}
}

func compareScene(t *testing.T, path string, s *DrawableScene, el *model.Element, want jsonScene) {
	t.Helper()
	cmpRect := func(what string, got Rect, w jsonRect) {
		t.Helper()
		g := rectOf(got)
		if !near(g.X, w.X, geometryTol) || !near(g.Y, w.Y, geometryTol) || !near(g.Width, w.Width, geometryTol) || !near(g.Height, w.Height, geometryTol) {
			t.Errorf("%s %s: got %+v, TS %+v", path, what, g, w)
		}
	}
	cmpPoint := func(what string, got Point, w jsonPoint) {
		t.Helper()
		if !near(got.X, w.X, geometryTol) || !near(got.Y, w.Y, geometryTol) {
			t.Errorf("%s %s: got %+v, TS %+v", path, what, got, w)
		}
	}
	cmpRect("scene", s.Bounds(), want.Bounds)
	for _, w := range want.Items {
		it := el.Items[w.I]
		d := s.Drawables[it]
		if w.None {
			if d != nil {
				t.Errorf("%s item %d: TS has no drawable", path, w.I)
			}
			continue
		}
		if d == nil {
			t.Errorf("%s item %d (%s): no drawable", path, w.I, it.Name)
			continue
		}
		cmpRect("item "+it.Name+" bounds", d.Bounds(), w.Bounds)
		cmpRect("item "+it.Name+" selector", d.SelectorBounds(), w.Selector)
		if line, ok := d.(*DrawableLine); ok && w.Source != nil {
			cmpPoint("link source", line.Source, *w.Source)
			cmpPoint("link target", line.Target, *w.Target)
			if len(line.ExtraPoints) != len(w.Extra) {
				t.Errorf("%s link %d: %d waypoints, TS %d", path, w.I, len(line.ExtraPoints), len(w.Extra))
				continue
			}
			for i, p := range line.ExtraPoints {
				cmpPoint("waypoint", p, w.Extra[i])
			}
		}
	}
}
