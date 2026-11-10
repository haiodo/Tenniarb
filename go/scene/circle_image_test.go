package scene

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/offscreen"
)

// pngDataColor is a w by h PNG of one colour in base64, the payload of a classic `image "name" @(...)`.
func pngDataColor(t *testing.T, w, h int, c color.NRGBA) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// circleImage is a classic item of 100 by 100 with one inline picture of 80 by 80 in its title: the
// picture is bigger than the ellipse of the circle and would show outside it.
func circleImage(t *testing.T, display string) string {
	t.Helper()
	data := pngDataColor(t, 80, 80, color.NRGBA{R: 0xff, A: 0xff})
	return `element "E" {
  item Pic {
    pos 0 0
    width 100
    height 100
    display ` + display + `
    title %{
@(pic|80)
}
    image "pic" @(` + data + `)
  }
}`
}

// clipsBeforeImage reports whether the picture cuts what it draws before the first picture op.
func clipsBeforeImage(p *gfx.Picture) bool {
	if p == nil {
		return false
	}
	var it gfx.Iter
	it.Reset(p)
	for it.Next() {
		switch it.Op().Kind {
		case gfx.OpClipPath:
			return true
		case gfx.OpImage:
			return false
		}
	}
	return false
}

// renderItem draws the built element on a white screen and reads the pixels.
func renderItem(t *testing.T, src string) image.Image {
	t.Helper()
	tree, err := LoadSource(t.TempDir(), "a.tenn", src)
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	sc.Update(func(tx *gfx.Tx) { _, err = tree.Build(tx, nil, "E", Options{}) })
	if err != nil {
		t.Fatal(err)
	}
	rend, err := offscreen.Default()
	if err != nil {
		t.Skip("no offscreen renderer:", err)
	}
	raster, err := gfx.RenderSubtree(rend, sc, nil, gfx.R(0, 0, 100, 100), 1, gfx.Color{R: 1, G: 1, B: 1, A: 1})
	if err != nil {
		t.Fatal(err)
	}
	data, err := raster.PNG()
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// A picture in the title of a circle is inscribed: the circle cuts it, so nothing of it shows outside
// the ellipse. A box draws its pictures in full, as it always did.
func TestCircleTitleImageIsCut(t *testing.T) {
	red, white := color.RGBA{0xff, 0, 0, 0xff}, color.RGBA{0xff, 0xff, 0xff, 0xff}
	cases := []struct {
		display string
		clip    bool
		corner  color.RGBA // a corner of the picture box, outside the ellipse
		centre  color.RGBA
	}{
		{"circle", true, white, red},
		{"rect", false, red, red},
	}
	for _, c := range cases {
		t.Run(c.display, func(t *testing.T) {
			src := circleImage(t, c.display)
			tree, err := LoadSource(t.TempDir(), "a.tenn", src)
			if err != nil {
				t.Fatal(err)
			}
			screen := buildTree(t, tree, "E")
			if got := clipsBeforeImage(placed(t, screen, "Pic").Gfx.Picture()); got != c.clip {
				t.Fatalf("clipped %v, want %v", got, c.clip)
			}
			img := renderItem(t, src)
			if got := color.RGBAModel.Convert(img.At(12, 12)).(color.RGBA); got != c.corner {
				t.Errorf("corner (12, 12) = %v, want %v", got, c.corner)
			}
			if got := color.RGBAModel.Convert(img.At(50, 50)).(color.RGBA); got != c.centre {
				t.Errorf("centre (50, 50) = %v, want %v", got, c.centre)
			}
		})
	}
}

// redPixels counts the picture of the rendered element.
func redPixels(t *testing.T, src string) int {
	t.Helper()
	img := renderItem(t, src)
	red := color.RGBA{0xff, 0, 0, 0xff}
	n := 0
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			if color.RGBAModel.Convert(img.At(x, y)).(color.RGBA) == red {
				n++
			}
		}
	}
	return n
}

// The same for a picture in the body: a circle cuts it as it cuts the one in the title, so less of a
// picture that reaches the corners of the box stays visible.
func TestCircleBodyImageIsCut(t *testing.T) {
	data := pngDataColor(t, 120, 120, color.NRGBA{R: 0xff, A: 0xff})
	src := func(display string) string {
		return fmt.Sprintf(`element "E" {
  item Pic {
    pos 0 0
    width 100
    height 100
    display %s
    body "Pic @(pic|120)"
    image "pic" @(%s)
  }
}`, display, data)
	}
	tree, err := LoadSource(t.TempDir(), "a.tenn", src("circle"))
	if err != nil {
		t.Fatal(err)
	}
	screen := buildTree(t, tree, "E")
	if !clipsBeforeImage(placed(t, screen, "Pic").Gfx.Picture()) {
		t.Fatal("the picture of the body is not cut by the circle")
	}
	box := redPixels(t, src("rect"))
	cut := redPixels(t, src("circle"))
	if cut >= box {
		t.Errorf("the circle keeps %d of %d pixels of the picture, want fewer", cut, box)
	}
}
