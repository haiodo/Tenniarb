package scene

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/offscreen"
)

// pngFile writes a w by h PNG of one colour into dir.
func pngFile(t *testing.T, dir, name string, w, h int) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, dir, name, buf.Bytes())
}

func writeTestFile(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// placed finds the item of a built screen by name.
func placed(t *testing.T, s *Screen, name string) *Placed {
	t.Helper()
	var find func(p *Placed) *Placed
	find = func(p *Placed) *Placed {
		if p.Src != nil && p.Src.Name == name {
			return p
		}
		for _, k := range p.Kids {
			if r := find(k); r != nil {
				return r
			}
		}
		return nil
	}
	if r := find(s.Placed()); r != nil {
		return r
	}
	t.Fatalf("no item %q", name)
	return nil
}

// drawOps lists the image rectangles and whether the picture cuts with a rounded path.
func drawOps(p *gfx.Picture) (rects [][4]float64, rounded bool, images []gfx.Image) {
	if p == nil {
		return nil, false, nil
	}
	var it gfx.Iter
	it.Reset(p)
	for it.Next() {
		op := it.Op()
		switch op.Kind {
		case gfx.OpImage:
			rects = append(rects, [4]float64{op.A, op.B, op.C, op.D})
			images = append(images, op.Image)
		case gfx.OpClipPath:
			rounded = true
		}
	}
	return rects, rounded, images
}

func wantRect(t *testing.T, got [4]float64, x, y, w, h float64) {
	t.Helper()
	for i, want := range []float64{x, y, w, h} {
		if math.Abs(got[i]-want) > 0.01 {
			t.Errorf("image rect = %v, want %v", got, [4]float64{x, y, w, h})
			return
		}
	}
}

func TestImageDrawsFile(t *testing.T) {
	root := t.TempDir()
	pngFile(t, root, "assets/pic.png", 4, 2)
	writeTestFile(t, root, "a.tenn", []byte(`element "E" {
  item Pic { pos 0 0; width 40; height 20; image "assets/pic.png" }
}`))
	tree, err := Load(root, "a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	screen := buildTree(t, tree, "E")
	rects, _, _ := drawOps(placed(t, screen, "Pic").Gfx.Picture())
	if len(rects) != 1 {
		t.Fatalf("rects %v", rects)
	}
	wantRect(t, rects[0], 0, 0, 40, 20) // fill stretches into the box
	if w, h := screen.W, screen.H; w != 40 || h != 20 {
		t.Errorf("screen %v x %v", w, h)
	}
}

func buildTree(t *testing.T, tree *Tree, path string) *Screen {
	t.Helper()
	sc := gfx.NewScene()
	var screen *Screen
	var err error
	sc.Update(func(tx *gfx.Tx) { screen, err = tree.Build(tx, nil, path, Options{}) })
	if err != nil {
		t.Fatalf("build %q: %v", path, err)
	}
	sc.Commit()
	return screen
}

// imageIn builds one item with an image of 4 by 2 pixels in a 20 by 20 box.
func imageIn(t *testing.T, props string) (*Screen, *Placed) {
	t.Helper()
	root := t.TempDir()
	pngFile(t, root, "pic.png", 4, 2)
	writeTestFile(t, root, "a.tenn", []byte("element \"E\" {\n  item Pic { pos 0 0; width 20; height 20; image \"pic.png\"; corner-radius 0; "+props+" }\n}"))
	tree, err := Load(root, "a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	screen := buildTree(t, tree, "E")
	return screen, placed(t, screen, "Pic")
}

func TestImageFitCover(t *testing.T) {
	_, pl := imageIn(t, "fit cover")
	rects, rounded, _ := drawOps(pl.Gfx.Picture())
	if len(rects) != 1 {
		t.Fatalf("rects %v", rects)
	}
	// max(20/4, 20/2) = 10: 40 by 20, centered, the sides cut by the box clip.
	wantRect(t, rects[0], -10, 0, 40, 20)
	if rounded {
		t.Error("cover of a plain box takes a rect clip, not a rounded path")
	}
}

func TestImageFitContain(t *testing.T) {
	_, pl := imageIn(t, "fit contain")
	rects, rounded, _ := drawOps(pl.Gfx.Picture())
	if len(rects) != 1 {
		t.Fatalf("rects %v", rects)
	}
	// min(20/4, 20/2) = 5: 20 by 10, centered, empty bands above and below.
	wantRect(t, rects[0], 0, 5, 20, 10)
	if rounded {
		t.Error("contain needs no clip")
	}
}

func TestImageCornerRadiusClips(t *testing.T) {
	_, pl := imageIn(t, "fit cover; corner-radius 6")
	rects, rounded, _ := drawOps(pl.Gfx.Picture())
	if len(rects) != 1 || !rounded {
		t.Fatalf("rects %v rounded %v", rects, rounded)
	}
}

func TestImageNaturalSize(t *testing.T) {
	root := t.TempDir()
	pngFile(t, root, "pic.png", 7, 3)
	writeTestFile(t, root, "a.tenn", []byte("element \"E\" {\n  item Pic { pos 0 0; image \"pic.png\" }\n}"))
	tree, err := Load(root, "a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	screen := buildTree(t, tree, "E")
	if pl := placed(t, screen, "Pic"); pl.W != 7 || pl.H != 3 {
		t.Errorf("box %v x %v, want the pixels", pl.W, pl.H)
	}
}

func TestRootImage(t *testing.T) {
	root := t.TempDir()
	pngFile(t, root, "bg.png", 8, 6)
	writeTestFile(t, root, "a.tenn", []byte(`element "E" {
  width 80; height 60; image "bg.png" fit cover; corner-radius 4
}`))
	tree, err := Load(root, "a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	screen := buildTree(t, tree, "E")
	rects, rounded, _ := drawOps(screen.Root.Picture())
	if len(rects) != 1 || !rounded {
		t.Fatalf("root rects %v rounded %v", rects, rounded)
	}
	wantRect(t, rects[0], 0, 0, 80, 60)
	if screen.W != 80 || screen.H != 60 {
		t.Errorf("screen %v x %v, want the size of the element", screen.W, screen.H)
	}
}

func TestImageMissingWarns(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "a.tenn", []byte("element \"E\" {\n  item Pic { pos 0 0; width 40; height 20; image \"nope.png\" }\n}"))
	tree, err := Load(root, "a.tenn")
	if err != nil {
		t.Fatal(err) // a missing picture is not an error
	}
	screen := buildTree(t, tree, "E")
	if got := screen.Warnings.Error(); !strings.Contains(got, `image "nope.png"`) || !strings.Contains(got, "no such file") {
		t.Errorf("warnings %q", got)
	}
	pl := placed(t, screen, "Pic")
	if pl.W != 40 || pl.H != 20 {
		t.Errorf("item %v x %v, want it laid out empty", pl.W, pl.H)
	}
	if rects, _, _ := drawOps(pl.Gfx.Picture()); len(rects) != 0 {
		t.Errorf("a missing picture draws %v", rects)
	}
}

func TestImageUnreadableFormatWarns(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "photo.avif", []byte("\x00\x00\x00\x1cftypavif\x00\x00\x00\x00"))
	writeTestFile(t, root, "a.tenn", []byte("element \"E\" {\n  item Pic { pos 0 0; width 10; height 10; image \"photo.avif\" }\n}"))
	tree, err := Load(root, "a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	screen := buildTree(t, tree, "E")
	if got := screen.Warnings.Error(); !strings.Contains(got, "the AVIF format is not read") {
		t.Errorf("warnings %q", got)
	}
}

func TestImageWebP(t *testing.T) {
	// A 1 by 1 lossy WebP: the decoder is registered by the scene package, not by the caller.
	raw, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeTestFile(t, root, "pic.webp", raw)
	writeTestFile(t, root, "a.tenn", []byte("element \"E\" {\n  item Pic { pos 0 0; image \"pic.webp\" }\n}"))
	tree, err := Load(root, "a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	screen := buildTree(t, tree, "E")
	rects, _, _ := drawOps(placed(t, screen, "Pic").Gfx.Picture())
	if len(rects) != 1 {
		t.Fatalf("a WebP file draws %v; warnings %v", rects, screen.Warnings)
	}
}

func TestImageCacheReusesRaster(t *testing.T) {
	root := t.TempDir()
	pngFile(t, root, "pic.png", 2, 2)
	writeTestFile(t, root, "a.tenn", []byte("element \"E\" {\n  item Pic { pos 0 0; image \"pic.png\" }\n}"))
	tree, err := Load(root, "a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	_, _, first := drawOps(placed(t, buildTree(t, tree, "E"), "Pic").Gfx.Picture())
	_, _, second := drawOps(placed(t, buildTree(t, tree, "E"), "Pic").Gfx.Picture())
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("images %v %v", first, second)
	}
	if first[0] != second[0] {
		t.Error("a rebuild decoded the file again: a backend caches rasters by pointer")
	}
}

func TestRotateTurnsPicture(t *testing.T) {
	root := t.TempDir()
	pngFile(t, root, "pic.png", 4, 2)
	writeTestFile(t, root, "a.tenn", []byte("element \"E\" {\n  item Pic { pos 0 0; width 100; height 40; image \"pic.png\"; rotate 90 }\n}"))
	tree, err := Load(root, "a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	screen := buildTree(t, tree, "E")
	pl := placed(t, screen, "Pic")
	m := pl.Gfx.Transform()
	// 90 degrees clockwise about the center: (0,0) is turned to the left corner of the box.
	if math.Abs(m[0]) > 1e-9 || math.Abs(m[1]-1) > 1e-9 || math.Abs(m[2]+1) > 1e-9 || math.Abs(m[3]) > 1e-9 {
		t.Errorf("transform %v", m)
	}
	if b := pl.Gfx.Bounds(); math.Abs(b.W()-40) > 0.01 || math.Abs(b.H()-100) > 0.01 {
		t.Errorf("bounds %v x %v, want the turned box", b.W(), b.H())
	}
}

func TestImageLeavesWorkspace(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "a.tenn", []byte("element \"E\" {\n  item Pic { width 10; height 10; image \"../out.png\" }\n}"))
	if _, err := Load(root, "a.tenn"); err == nil || !strings.Contains(err.Error(), "leaves the workspace root") {
		t.Errorf("error %v", err)
	}
}

func TestImageOverlayPath(t *testing.T) {
	// The path of a picture is relative to the file that writes the item property, not to the entry.
	root := t.TempDir()
	pngFile(t, root, "lib/pic.png", 3, 3)
	writeTestFile(t, root, "lib/ui.tenn", []byte("item Shot { width 12; height 12; image \"pic.png\" }"))
	writeTestFile(t, root, "design.tenn", []byte("element \"ui\" --from-file \"lib/ui.tenn\"\nelement \"E\" { item Card { auto-layout row; use ui.Shot as shot } }\n"))
	tree, err := LoadIn(root, "design.tenn", "design.tenn", nil)
	if err != nil {
		t.Fatal(err)
	}
	screen := buildTree(t, tree, "E")
	if rects, _, _ := drawOps(placed(t, screen, "shot").Gfx.Picture()); len(rects) != 1 {
		t.Errorf("the picture of a mounted item draws %v; warnings %v", rects, screen.Warnings)
	}
}

func TestImageCacheEvicts(t *testing.T) {
	old := imageCache.limit
	imageCache.limit = 40 // room for two rasters of 1 by 2 and 2 by 2 pixels
	imageCache.reset()
	defer func() {
		imageCache.limit = old
		imageCache.reset()
	}()
	dir := t.TempDir()
	var keys []imageKey
	for i, name := range []string{"a.png", "b.png", "c.png"} {
		pngFile(t, dir, name, i+1, 2)
		keys = append(keys, imageKey{filepath.Join(dir, name), int64(i + 2), 0})
	}
	if img, why := decodeImage(keys[0]); img == nil {
		t.Fatalf("a: %s", why)
	}
	if img, why := decodeImage(keys[1]); img == nil {
		t.Fatalf("b: %s", why)
	}
	// A third picture that does not fit pushes the least recently used one out.
	writeTestFile(t, dir, "c.png", mustPNG(t, 3, 2))
	if img, why := decodeImage(keys[2]); img == nil {
		t.Fatalf("c: %s", why)
	}
	if imageCache.size > imageCache.limit {
		t.Errorf("cache holds %d of %d", imageCache.size, imageCache.limit)
	}
	if imageCache.get(keys[0]) != nil && imageCache.get(keys[1]) != nil && imageCache.get(keys[2]) != nil {
		t.Error("nothing was evicted")
	}
}

func mustPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.NRGBA{R: 0xff, A: 0xff})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// splitPNG writes a file of 4 by 2 pixels: red on the left half, blue on the right.
func splitPNG(t *testing.T, dir, name string) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			c := color.NRGBA{R: 0xff, A: 0xff}
			if x >= 2 {
				c = color.NRGBA{B: 0xff, A: 0xff}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, dir, name, buf.Bytes())
}

// TestImagePixels draws a split picture through the headless renderer: the fit and the turn must show
// where the geometry says they do, not only in the command stream.
func TestImagePixels(t *testing.T) {
	if _, err := offscreen.Default(); err != nil {
		t.Skip("no offscreen renderer:", err)
	}
	red, blue, white := color.RGBA{0xff, 0, 0, 0xff}, color.RGBA{0, 0, 0xff, 0xff}, color.RGBA{0xff, 0xff, 0xff, 0xff}
	cases := []struct {
		name  string
		props string
		want  [][3]any // x, y, colour
	}{
		{"cover cuts the sides", "fit cover", [][3]any{{2, 10, red}, {18, 10, blue}}},
		{"contain leaves bands", "fit contain", [][3]any{{2, 7, red}, {17, 7, blue}, {2, 1, white}}},
		{"rotate turns clockwise", "fit cover; rotate 90", [][3]any{{10, 4, red}, {10, 16, blue}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			splitPNG(t, root, "split.png")
			writeTestFile(t, root, "a.tenn", []byte("element \"E\" {\n  item Pic { pos 0 0; width 20; height 20; title \"\"; image \"split.png\" "+c.props+" }\n}"))
			tree, err := Load(root, "a.tenn")
			if err != nil {
				t.Fatal(err)
			}
			sc := gfx.NewScene()
			sc.Update(func(tx *gfx.Tx) { _, err = tree.Build(tx, nil, "E", Options{}) })
			if err != nil {
				t.Fatal(err)
			}
			// The box itself, not the bounds of the picture: a cropped picture reaches past it.
			rend, err := offscreen.Default()
			if err != nil {
				t.Fatal(err)
			}
			raster, err := gfx.RenderSubtree(rend, sc, nil, gfx.R(0, 0, 20, 20), 1, gfx.Color{R: 1, G: 1, B: 1, A: 1})
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
			for _, w := range c.want {
				x, y := w[0].(int), w[1].(int)
				if got := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA); got != w[2].(color.RGBA) {
					t.Errorf("(%d, %d) = %v, want %v", x, y, got, w[2])
				}
			}
		})
	}
}
