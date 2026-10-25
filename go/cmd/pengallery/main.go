package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/offscreen"
	"github.com/haiodo/gowt/gfx/pen"
	"github.com/haiodo/tenniarb/go/penimport"
	"github.com/haiodo/tenniarb/go/scene"
)

// usage: pengallery FONTDIR OUTDIR MAXPERFILE file.pen...
func main() {
	a := os.Args[1:]
	_, err := offscreen.RegisterFonts(a[0])
	must(err)
	out := a[1]
	var max int
	fmt.Sscan(a[2], &max)
	r, err := offscreen.Default()
	must(err)
	m := r.(gfx.TextMeasurer)
	white := gfx.Color{R: 1, G: 1, B: 1, A: 1}
	tdir := filepath.Join(out, "tenn")
	res, err := penimport.Convert(a[3:], penimport.Options{})
	must(err)
	must(res.Write(tdir))
	for _, f := range res.Files {
		ids := map[string]string{}
		for _, ln := range strings.Split(res.Map(f), "\n") {
			if id, p, ok := strings.Cut(ln, "\t"); ok {
				ids[id] = p
			}
		}
		d, err := pen.Open(f.Path, pen.Options{Measurer: m})
		must(err)
		for i, in := range d.Screens() {
			if i >= max {
				break
			}
			sc := gfx.NewScene()
			var ps *pen.Screen
			sc.Update(func(tx *gfx.Tx) { ps, err = d.Build(tx, nil, in.Index) })
			must(err)
			pb, err := offscreen.PNG(sc, ps.Root, 1, white)
			must(err)
			// A screen is an item of the file root, so a wrapper file uses it as its only content.
			wrap := fmt.Sprintf("import %s.tenn as f\nuse f.%s as \"W\"\n", f.Name, ids[in.ID])
			tree, err := scene.LoadSource(tdir, "wrap.tenn", wrap)
			if err != nil {
				fmt.Fprintln(os.Stderr, f.Name, in.Name, err)
				continue
			}
			sc2 := gfx.NewScene()
			var ts *scene.Screen
			sc2.Update(func(tx *gfx.Tx) { ts, err = tree.Build(tx, nil, "", scene.Options{Measurer: m}) })
			if err != nil {
				fmt.Fprintln(os.Stderr, f.Name, in.Name, err)
				continue
			}
			tb, err := offscreen.PNG(sc2, ts.Root, 1, white)
			must(err)
			name := fmt.Sprintf("%s-%02d.png", f.Name, i)
			must(os.WriteFile(filepath.Join(out, name), pair(pb, tb), 0o644))
			fmt.Println(name, in.Name, in.W, in.H)
		}
	}
}

func pair(a, b []byte) []byte {
	ia, err := png.Decode(bytes.NewReader(a))
	must(err)
	ib, err := png.Decode(bytes.NewReader(b))
	must(err)
	gap := 24
	w := ia.Bounds().Dx() + gap + ib.Bounds().Dx()
	h := max(ia.Bounds().Dy(), ib.Bounds().Dy())
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{color.RGBA{0xc8, 0xc8, 0xf0, 0xff}}, image.Point{}, draw.Src)
	draw.Draw(dst, ia.Bounds(), ia, ia.Bounds().Min, draw.Src)
	draw.Draw(dst, ib.Bounds().Add(image.Pt(ia.Bounds().Dx()+gap, 0)), ib, ib.Bounds().Min, draw.Src)
	var buf bytes.Buffer
	must(png.Encode(&buf, dst))
	return buf.Bytes()
}

func must(v any) {
	if err, ok := v.(error); ok && err != nil {
		panic(err)
	}
}
