package scene_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
)

// parityPNG is a w by h PNG in base64, the payload of a classic `image "name" @(...)`.
func parityPNG(t *testing.T, w, h int) string {
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

// imageY lists the y of every image op in a picture.
func imageY(p *gfx.Picture) []float64 {
	var out []float64
	var it gfx.Iter
	it.Reset(p)
	for it.Next() {
		if op := it.Op(); op.Kind == gfx.OpImage {
			out = append(out, op.B)
		}
	}
	return out
}

func placedByName(p *scene.Placed, name string) *scene.Placed {
	if p.Src != nil && p.Src.Name == name {
		return p
	}
	for _, k := range p.Kids {
		if r := placedByName(k, name); r != nil {
			return r
		}
	}
	return nil
}

// An inline picture in a title is drawn once, inside the box of its item: the title names the picture and
// the picture lands with the text, wherever the line of the title runs.
func TestClassicTitleImageIsInsideItsItem(t *testing.T) {
	src := fmt.Sprintf(`element "E" {
  item Pic { pos 0 0; width 200; height 120; title "Icon @(logo|40)\nSecond line"; image "logo" @(%s) }
}`, parityPNG(t, 100, 50))

	tree, err := scene.LoadSource(t.TempDir(), "a.tenn", src)
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.Build(tx, nil, "E", scene.Options{Measurer: gfx.EstimateText{}})
	})
	if err != nil {
		t.Fatal(err)
	}
	sc.Commit()
	pic := placedByName(screen.Placed(), "Pic")
	if pic == nil {
		t.Fatal("no item Pic")
	}
	got := imageY(pic.Gfx.Picture())
	if len(got) != 1 {
		t.Fatalf("scene image ops %v, want one", got)
	}
	if got[0] < pic.Y-1 || got[0] > pic.Y+pic.H+40+1 {
		t.Errorf("the picture starts at %v, the item spans %v..%v", got[0], pic.Y, pic.Y+pic.H)
	}
}
