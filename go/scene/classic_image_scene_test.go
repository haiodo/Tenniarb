package scene

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"testing"
)

// pngData is a w by h PNG in base64, the payload of a classic `image "name" @(...)`.
func pngData(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// An inline image the text does not name is the content of the box, and a box without a written size
// takes the pixels of the picture, as a file image does.
func TestClassicInlineImageSizesItem(t *testing.T) {
	src := fmt.Sprintf(`element "E" {
  item Pic { pos 0 0; title ""; image "logo" @(%s) }
}`, pngData(t, 7, 3))
	tree, err := LoadSource(t.TempDir(), "a.tenn", src)
	if err != nil {
		t.Fatal(err)
	}
	screen := buildTree(t, tree, "E")
	pl := placed(t, screen, "Pic")
	if pl.W != 7 || pl.H != 3 {
		t.Errorf("box %v x %v, want the pixels 7 x 3", pl.W, pl.H)
	}
	rects, _, _ := drawOps(pl.Gfx.Picture())
	if len(rects) != 1 {
		t.Fatalf("picture rects %v, want one", rects)
	}
	wantRect(t, rects[0], 0, 0, 7, 3)
}

// @(name|width) in a title draws the inline picture at that width, its height from the pixels.
func TestClassicTitleInlineImage(t *testing.T) {
	src := fmt.Sprintf(`element "E" {
  item Pic { pos 0 0; width 200; height 120; title "Icon @(logo|80)"; image "logo" @(%s) }
}`, pngData(t, 100, 50))
	tree, err := LoadSource(t.TempDir(), "a.tenn", src)
	if err != nil {
		t.Fatal(err)
	}
	screen := buildTree(t, tree, "E")
	rects, _, _ := drawOps(placed(t, screen, "Pic").Gfx.Picture())
	if len(rects) != 1 {
		t.Fatalf("picture rects %v, want one", rects)
	}
	if w, h := rects[0][2], rects[0][3]; w != 80 || h != 40 {
		t.Errorf("inline image %v x %v, want 80 x 40", w, h)
	}
}

// The same reference in a body draws the picture among its markdown.
func TestClassicBodyInlineImage(t *testing.T) {
	src := fmt.Sprintf(`element "E" {
  item Pic { pos 0 0; width 200; height 120; body "Pic @(logo|40)"; image "logo" @(%s) }
}`, pngData(t, 100, 50))
	tree, err := LoadSource(t.TempDir(), "a.tenn", src)
	if err != nil {
		t.Fatal(err)
	}
	screen := buildTree(t, tree, "E")
	rects, _, _ := drawOps(placed(t, screen, "Pic").Gfx.Picture())
	if len(rects) != 1 {
		t.Fatalf("picture rects %v, want one", rects)
	}
	if w, h := rects[0][2], rects[0][3]; w != 40 || h != 20 {
		t.Errorf("inline image %v x %v, want 40 x 20", w, h)
	}
}

// The file form of an image keeps drawing as the content of the box.
func TestSceneFileImageStillDraws(t *testing.T) {
	root := t.TempDir()
	pngFile(t, root, "pic.png", 9, 4)
	writeTestFile(t, root, "a.tenn", []byte("element \"E\" {\n  item Pic { pos 0 0; image \"pic.png\" }\n}"))
	tree, err := Load(root, "a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	screen := buildTree(t, tree, "E")
	pl := placed(t, screen, "Pic")
	if pl.W != 9 || pl.H != 4 {
		t.Errorf("box %v x %v, want the pixels 9 x 4", pl.W, pl.H)
	}
	rects, _, _ := drawOps(pl.Gfx.Picture())
	if len(rects) != 1 {
		t.Fatalf("picture rects %v, want one", rects)
	}
	wantRect(t, rects[0], 0, 0, 9, 4)
}
