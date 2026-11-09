package scene_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"math"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
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

// An inline picture in a title hangs from the cap height of its text, as go/render draws it, not from
// the middle of the line box.
func TestClassicTitleImageYMatchesRender(t *testing.T) {
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
	got := imageY(placedByName(screen.Placed(), "Pic").Gfx.Picture())
	if len(got) != 1 {
		t.Fatalf("scene image ops %v, want one", got)
	}

	m := model.ReadTenn(src)
	ds := render.Build(m.Elements[0], render.Options{Measurer: gfx.EstimateText{}})
	var want []float64
	for _, item := range ds.Items() {
		if d := ds.Drawables[item]; d != nil {
			want = imageY(ds.Picture(d))
		}
	}
	if len(want) != 1 {
		t.Fatalf("render image ops %v, want one", want)
	}
	if dy := math.Abs(got[0] - want[0]); dy > 0.5 {
		t.Errorf("picture y %v, go/render %v: off by %v", got[0], want[0], dy)
	}
}
