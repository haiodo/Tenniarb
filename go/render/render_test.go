package render

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/offscreen"

	"github.com/haiodo/tenniarb/go/model"
)

// needRaster skips a test on an OS without an offscreen backend.
func needRaster(t *testing.T) {
	t.Helper()
	if _, err := offscreen.Default(); err != nil {
		t.Skip("no offscreen renderer:", err)
	}
}

func decodePNG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func px(img image.Image, x, y int) [3]int {
	r, g, b, _ := img.At(x, y).RGBA()
	return [3]int{int(r >> 8), int(g >> 8), int(b >> 8)}
}

func pictureOps(p *gfx.Picture) []gfx.OpKind {
	var it gfx.Iter
	var kinds []gfx.OpKind
	it.Reset(p)
	for it.Next() {
		kinds = append(kinds, it.Op().Kind)
	}
	return kinds
}

func hasOp(p *gfx.Picture, k gfx.OpKind) bool {
	for _, o := range pictureOps(p) {
		if o == k {
			return true
		}
	}
	return false
}

func TestRenderFlipsYSoLargerYIsHigher(t *testing.T) {
	needRaster(t)
	el := parseDiagram(t, `element Root {
    item Top {
        pos 0 100
        width 40
        height 20
        color red
        border-color red
        title " "
    }
    item Low {
        pos 0 0
        width 40
        height 20
        color blue
        border-color blue
        title " "
    }
}`)
	data, err := PNG(el, testOpts(), 1, gfx.White)
	if err != nil {
		t.Fatal(err)
	}
	img := decodePNG(t, data)
	size := SceneSize(el, testOpts())
	if img.Bounds().Dx() != int(size.Width+0.999) || img.Bounds().Dy() != int(size.Height+0.999) {
		t.Errorf("image %v, scene size %+v", img.Bounds(), size)
	}
	top := px(img, 35, 25)
	low := px(img, 35, img.Bounds().Dy()-25)
	if !(top[0] > 200 && top[2] < 100) {
		t.Errorf("top pixel %v", top)
	}
	if !(low[2] > 200 && low[0] < 100) {
		t.Errorf("low pixel %v", low)
	}
}

func TestTextIsUprightAndTopAligned(t *testing.T) {
	needRaster(t)
	el := parseDiagram(t, `element Root {
    item T {
        pos 0 0
        width 100
        height 100
        layout top left
        title "WWWW"
        color white
        border-color white
    }
}`)
	o := Options{} // real font
	data, err := PNG(el, o, 1, gfx.White)
	if err != nil {
		t.Fatal(err)
	}
	img := decodePNG(t, data)
	dark := func(y0, y1 int) int {
		n := 0
		for y := y0; y < y1; y++ {
			for x := 15; x < 115; x++ {
				if c := px(img, x, y); c[0] < 128 {
					n++
				}
			}
		}
		return n
	}
	if dark(15, 45) <= 50 {
		t.Error("text must sit near the top")
	}
	if n := dark(75, 115); n != 0 {
		t.Errorf("%d dark pixels near the bottom", n)
	}
}

func TestSceneSizeMatchesImage(t *testing.T) {
	el := parseDiagram(t, sceneSource)
	var buf bytes.Buffer
	if err := SVG(&buf, el, testOpts(), 1); err != nil {
		t.Fatal(err)
	}
	size := SceneSize(el, testOpts())
	want := `<svg xmlns="http://www.w3.org/2000/svg" width="`
	if !strings.HasPrefix(buf.String(), want) {
		t.Fatal(buf.String()[:60])
	}
	if !strings.Contains(buf.String(), `viewBox="0 0 `) || size.Width <= 30 {
		t.Errorf("svg header %q size %+v", buf.String()[:140], size)
	}
}

func TestPaddingOption(t *testing.T) {
	el := parseDiagram(t, "element R { item A { pos 0 0 } }")
	zero := 0.0
	o := testOpts()
	o.Padding = &zero
	a, b := SceneSize(el, o), SceneSize(el, testOpts())
	if !near(b.Width-a.Width, 30, 1e-9) || !near(b.Height-a.Height, 30, 1e-9) {
		t.Errorf("padding: %+v vs %+v", a, b)
	}
}

func TestRealizeKeepsOneNodePerDrawableInDrawOrder(t *testing.T) {
	el := parseDiagram(t, `element Root {
    item Top { pos 0 0
        layer hover }
    item Mid { pos 50 0 }
    item Back { pos 100 0
        layer background }
    link Mid Back
}`)
	s := Build(el, testOpts())
	gs := gfx.NewScene()
	var r *Rendered
	gs.Update(func(tx *gfx.Tx) { r = s.Realize(tx, nil) })
	gs.Commit()
	ordered := s.elements.Ordered()
	if len(r.Nodes) != 4 || len(ordered) != 4 {
		t.Fatalf("%d nodes for %d drawables", len(r.Nodes), len(ordered))
	}
	for i, d := range ordered {
		if z := r.Nodes[d].Z(); z != i {
			t.Errorf("drawable %d has z %d", i, z)
		}
	}
	if first := ordered[0].(interface{ item() *model.DiagramItem }).item().Name; first != "Back" {
		t.Errorf("background first, got %s", first)
	}
	if last := ordered[3].(interface{ item() *model.DiagramItem }).item().Name; last != "Top" {
		t.Errorf("hover last, got %s", last)
	}
	// A node's picture covers the box it draws, in y-down scene coordinates (y is negated).
	for d, n := range r.Nodes {
		if _, isLine := d.(*DrawableLine); isLine {
			continue // a line's selector box is padded to a minimum height
		}
		pb, db := n.Picture().Bounds, d.SelectorBounds()
		if pb.Min.X > db.X+1e-3 || pb.Max.X < db.X+db.Width-1e-3 || pb.Min.Y > -(db.Y+db.Height)+1e-3 || pb.Max.Y < -db.Y-1e-3 {
			t.Errorf("picture %+v does not cover %+v", pb, db)
		}
	}
}

func TestShadowUsesShadowBox(t *testing.T) {
	el := parseDiagram(t, "element R {\n item A { pos 0 0\n shadow 3 3 4 }\n item B { pos 100 0 }\n}")
	s := Build(el, testOpts())
	if !hasOp(s.Picture(s.Drawables[el.Items[0]]), gfx.OpShadowBox) {
		t.Error("an item with a shadow records a shadow box")
	}
	if hasOp(s.Picture(s.Drawables[el.Items[1]]), gfx.OpShadowBox) {
		t.Error("an item without a shadow does not")
	}
}

func TestShadowOpsByBoxKind(t *testing.T) {
	cases := []struct {
		name, props string
		want        gfx.OpKind
	}{
		{"ellipse", "display circle\n width 120", gfx.OpFillShadow},
		{"mixed radii", "corner-radius 20 0 20 0", gfx.OpFillShadow},
		{"no-fill", "display no-fill", gfx.OpStrokeShadow},
		{"text", "display text", gfx.OpTextShadow},
	}
	for _, c := range cases {
		el := parseDiagram(t, "element R { item A { pos 0 0\n "+c.props+"\n shadow 3 3 4 } }")
		s := Build(el, testOpts())
		if !hasOp(s.Picture(s.Drawables[el.Items[0]]), c.want) {
			t.Errorf("%s: no op %v", c.name, c.want)
		}
	}
}

func TestLineShadow(t *testing.T) {
	el := parseDiagram(t, "element R {\n item A { pos 0 0 }\n item B { pos 100 0 }\n link A B { shadow 3 3 4 }\n}")
	s := Build(el, testOpts())
	for _, it := range el.Items {
		if it.IsLink() {
			if !hasOp(s.Picture(s.Drawables[it]), gfx.OpStrokeShadow) {
				t.Error("a link with a shadow records a stroke shadow")
			}
			return
		}
	}
	t.Fatal("no link")
}

func TestShadowIsYFlipped(t *testing.T) {
	// Swift shadow offsets are y-up: "shadow 0 5" puts the shadow above the box, y-down gets -5.
	el := parseDiagram(t, "element R { item A { pos 0 0\n shadow 0 5 4 } }")
	s := Build(el, testOpts())
	var it gfx.Iter
	it.Reset(s.Picture(s.Drawables[el.Items[0]]))
	for it.Next() {
		if o := it.Op(); o.Kind == gfx.OpShadowBox {
			if o.Shadow.DY != -5 || o.Shadow.DX != 0 {
				t.Errorf("shadow offset %v %v", o.Shadow.DX, o.Shadow.DY)
			}
			return
		}
	}
	t.Error("no shadow box")
}

func TestLineStyleDashes(t *testing.T) {
	for style, want := range map[string][]float64{"dashed": {5, 5}, "dotted": {1, 4}} {
		el := parseDiagram(t, "element R {\n item A { pos 0 0 }\n item B { pos 100 0 }\n link A B { line-style "+style+" }\n}")
		s := Build(el, testOpts())
		var link Drawable
		for _, it := range el.Items {
			if it.IsLink() {
				link = s.Drawables[it]
			}
		}
		var it gfx.Iter
		it.Reset(s.Picture(link))
		found := false
		for it.Next() {
			if o := it.Op(); o.Kind == gfx.OpStroke {
				found = true
				if len(o.Style.Dash) != len(want) || o.Style.Dash[0] != want[0] || o.Style.Dash[1] != want[1] {
					t.Errorf("%s dash %v", style, o.Style.Dash)
				}
			}
		}
		if !found {
			t.Errorf("%s: no stroke", style)
		}
	}
}

func TestInlineImageGrowsItemAndIsUpright(t *testing.T) {
	needRaster(t)
	// 20x10: red top half, blue bottom half.
	src := image.NewNRGBA(image.Rect(0, 0, 20, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			c := color.NRGBA{255, 0, 0, 255}
			if y >= 5 {
				c = color.NRGBA{0, 0, 255, 255}
			}
			src.Set(x, y, c)
		}
	}
	var enc bytes.Buffer
	if err := png.Encode(&enc, src); err != nil {
		t.Fatal(err)
	}
	b64 := base64.StdEncoding.EncodeToString(enc.Bytes())
	el := parseDiagram(t, `element Root {
    item Pic {
        pos 0 0
        image "pic" @(`+b64+`)
        title %{@(pic|40x20)}
        display text
    }
}`)
	with := NewDrawableScene(el, testOpts()).Drawables[el.Items[0]].Bounds()
	o := testOpts()
	o.DecodeImage = noImages
	without := NewDrawableScene(el, o).Drawables[el.Items[0]].Bounds()
	if !(with.Width >= 40 && with.Height >= 20) || !(with.Width > without.Width) {
		t.Errorf("with image %+v, without %+v", with, without)
	}
	data, err := PNG(el, testOpts(), 1, gfx.White)
	if err != nil {
		t.Fatal(err)
	}
	img := decodePNG(t, data)
	firstY := func(match func([3]int) bool) int {
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				if match(px(img, x, y)) {
					return y
				}
			}
		}
		return -1
	}
	red := firstY(func(c [3]int) bool { return c[0] > 215 && c[1] < 40 && c[2] < 40 })
	blue := firstY(func(c [3]int) bool { return c[2] > 215 && c[0] < 40 && c[1] < 40 })
	if red < 0 || blue < 0 || red >= blue {
		t.Errorf("red half at y=%d, blue half at y=%d: both must show, red above blue", red, blue)
	}
	var svg bytes.Buffer
	if err := SVG(&svg, el, testOpts(), 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(svg.String(), "data:image/png") {
		t.Error("svg has no embedded image")
	}
}

func TestDarkModeFlipsDefaults(t *testing.T) {
	el := parseDiagram(t, "element R { item A { pos 0 0 } }")
	o := testOpts()
	o.DarkMode = true
	s := NewDrawableScene(el, o)
	box := s.Drawables[el.Items[0]].(*RoundBox)
	if box.Style.Color == colorWhite || box.Style.TextColor() != colorWhite {
		t.Errorf("dark item: %+v", box.Style.Color)
	}
}

// Every fixture that parses renders; set TENN_PNG_DIR to also write PNGs for eyeballing.
func TestFixturesRender(t *testing.T) {
	files, _ := filepath.Glob("../../web/fixtures/*.tenn")
	if len(files) == 0 {
		t.Fatal("no fixtures")
	}
	pngDir := os.Getenv("TENN_PNG_DIR")
	if pngDir != "" {
		_ = os.MkdirAll(pngDir, 0o755)
	}
	_, rasterErr := offscreen.Default()
	var walk func(el *model.Element, path string)
	walk = func(el *model.Element, path string) {
		s := Build(el, Options{})
		b := s.Bounds()
		for _, v := range []float64{b.X, b.Y, b.Width, b.Height} {
			if v != v || v > 1e12 || v < -1e12 {
				t.Errorf("%s: bounds %+v", path, b)
			}
		}
		for _, it := range el.Items {
			if it.Kind == model.ItemKindItem && it.Name != "" {
				if bb, ok := s.GetItemBounds(it); !ok || !(bb.Width > 0 && bb.Height > 0) {
					t.Errorf("%s: item %q has no bounds", path, it.Name)
				}
			}
		}
		if len(el.Items) > 0 {
			var buf bytes.Buffer
			if err := SVG(&buf, el, Options{}, 1); err != nil {
				t.Errorf("%s: svg: %v", path, err)
			}
			if rasterErr == nil {
				data, err := PNG(el, Options{}, 1, gfx.White)
				if err != nil {
					t.Errorf("%s: png: %v", path, err)
				} else if pngDir != "" {
					_ = os.WriteFile(filepath.Join(pngDir, strings.NewReplacer("/", "_", " ", "_").Replace(path)+".png"), data, 0o644)
				}
			}
		}
		for _, c := range el.Elements {
			walk(c, path+"/"+c.Name)
		}
	}
	for _, f := range files {
		if strings.HasSuffix(f, ".saved.tenn") {
			continue
		}
		text, _ := os.ReadFile(f)
		if m := model.ReadTenn(string(text)); m != nil {
			walk(&m.Element, strings.TrimSuffix(filepath.Base(f), ".tenn"))
		}
	}
}

func TestFindElement(t *testing.T) {
	m := model.ReadTenn(`element A {
    element B {
        element C {
            item x
        }
    }
    element "D/E" {
        item y
    }
}`)
	if m == nil {
		t.Fatal("parse")
	}
	root := &m.Element
	if e := FindElement(root, "A/B/C"); e == nil || e.Name != "C" {
		t.Errorf("path: %v", e)
	}
	if e := FindElement(root, "C"); e == nil || e.Name != "C" {
		t.Errorf("bare name: %v", e)
	}
	if e := FindElement(root, "A/D/E"); e == nil || e.Name != "D/E" {
		t.Errorf("name with a slash: %v", e)
	}
	if FindElement(root, "Nope") != nil {
		t.Error("unknown")
	}
	if len(AllElements(root)) != 4 {
		t.Errorf("all: %d", len(AllElements(root)))
	}
}
