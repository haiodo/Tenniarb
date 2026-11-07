package scene

import (
	"bytes"
	"container/list"
	"errors"
	"image"
	_ "image/gif" // decoders of the picture files an item names
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"math"
	"os"
	"sync"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/tenn"
	_ "golang.org/x/image/webp"
)

// itemImage is the picture file of an item and how it fills the box. The raster is read when the item
// is built, not when it is laid out: validate lays out every element of the workspace and needs only
// the header.
type itemImage struct {
	key  imageKey
	fit  string
	rel  string
	pos  Pos
	w, h int
	// broken marks a file that did not decode: the warning is once, the item stays empty.
	broken bool
}

// imageCacheBytes bounds the rasters kept between builds, at 4 bytes a pixel. A landing page of fifty
// photos stays decoded, so a rebuild reads no file twice; the least recently used go first.
const imageCacheBytes = 512 << 20

// imageKey is what a decoded file is: the same path with other bytes is another picture.
type imageKey struct {
	path          string
	size, modTime int64
}

var imageCache = &rasterCache{limit: imageCacheBytes, byKey: map[imageKey]*list.Element{}, order: list.New()}

type rasterCache struct {
	mu    sync.Mutex
	limit int
	size  int
	byKey map[imageKey]*list.Element
	order *list.List // front is the most recently used; values are *cachedRaster
}

type cachedRaster struct {
	key imageKey
	img *gfx.Raster
}

func (c *rasterCache) get(key imageKey) *gfx.Raster {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.byKey[key]; ok {
		c.order.MoveToFront(el)
		return el.Value.(*cachedRaster).img
	}
	return nil
}

// put keeps img and returns the raster a concurrent decode stored first; nil keeps the picture of the
// caller without the cache, for one larger than the whole budget.
func (c *rasterCache) put(key imageKey, img *gfx.Raster) *gfx.Raster {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.byKey[key]; ok {
		return el.Value.(*cachedRaster).img
	}
	size := img.W * img.H * 4
	if size > c.limit {
		return nil
	}
	c.byKey[key] = c.order.PushFront(&cachedRaster{key, img})
	c.size += size
	for c.size > c.limit {
		last := c.order.Back()
		ci := last.Value.(*cachedRaster)
		c.order.Remove(last)
		delete(c.byKey, ci.key)
		c.size -= ci.img.W * ci.img.H * 4
	}
	return img
}

// reset forgets every decoded picture; tests use it to count the decodes of a file.
func (c *rasterCache) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byKey = map[imageKey]*list.Element{}
	c.order.Init()
	c.size = 0
}

// fileImage reports whether pr is the `image "path"` of an item. The classic `image <name> <data>` of a
// diagram is not one and stays with go/render.
func fileImage(pr *Prop) bool {
	return pr != nil && (pr.Val != nil || len(pr.Args) < 2 || pr.Args[1].Kind != tenn.KindImage)
}

// loadImage reads the picture of an item: the path is relative to the file that writes the property, as
// with `element --from-file`. A picture that cannot be drawn is a warning, so the item stays empty and
// check and validate pass.
func (p *prep) loadImage(rn *RNode) *itemImage {
	pr := p.prop(rn, "image")
	if !fileImage(pr) {
		return nil
	}
	rel, _ := pr.text(0)
	if rel == "" {
		p.t.warn(pr.Pos, `image takes a path: image "assets/photo.png"`)
		return nil
	}
	abs, _, ok := p.t.resolveIn(pr.Pos.File, rel)
	if !ok {
		p.t.fail(pr.Pos, "image %q leaves the workspace root", rel)
		return nil
	}
	st, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			p.t.warn(pr.Pos, "image %q: no such file, the item is drawn empty", rel)
		} else {
			p.t.warn(pr.Pos, "image %q: %v, the item is drawn empty", rel, err)
		}
		return nil
	}
	f, err := os.Open(abs)
	if err != nil {
		p.t.warn(pr.Pos, "image %q: the file cannot be read, the item is drawn empty", rel)
		return nil
	}
	hdr := make([]byte, 16)
	n, _ := f.ReadAt(hdr, 0)
	cfg, _, err := image.DecodeConfig(f)
	f.Close()
	if err != nil {
		p.t.warn(pr.Pos, "image %q: %s, the item is drawn empty", rel, whyNoImage(hdr[:n]))
		return nil
	}
	return &itemImage{key: imageKey{abs, st.Size(), st.ModTime().UnixNano()},
		fit: p.fitOf(rn, pr), rel: rel, pos: pr.Pos, w: cfg.Width, h: cfg.Height}
}

// raster decodes the file once per build and reports a file the header did not tell about as a warning.
func (im *itemImage) raster(p *prep) *gfx.Raster {
	if im.broken {
		return nil
	}
	img, why := decodeImage(im.key)
	if img == nil {
		im.broken = true
		p.t.warn(im.pos, "image %q: %s, the item is drawn empty", im.rel, why)
		return nil
	}
	return img
}

// decodeImage reads and decodes a file; why says what kept it from decoding.
func decodeImage(key imageKey) (*gfx.Raster, string) {
	if img := imageCache.get(key); img != nil {
		return img, ""
	}
	raw, err := os.ReadFile(key.path)
	if err != nil {
		return nil, "the file cannot be read"
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, whyNoImage(raw)
	}
	r := gfx.RasterFrom(img)
	if kept := imageCache.put(key, r); kept != nil {
		return kept, ""
	}
	return r, ""
}

// whyNoImage tells a format the decoders of the scene do not read from a file that is no picture at all.
func whyNoImage(raw []byte) string {
	if len(raw) >= 12 && string(raw[4:8]) == "ftyp" {
		if brand := string(raw[8:12]); brand == "avif" || brand == "avis" {
			return "the AVIF format is not read (PNG, JPEG, GIF and WebP are)"
		}
	}
	return "the file is not a PNG, JPEG, GIF or WebP image"
}

// fitOf reads fit: fill stretches the picture into the box, cover fills the box and cuts the sides,
// contain fits all of it inside with empty bands. It goes after the path, `image "a.png" fit cover`, or
// in a command of its own, `fit cover`.
func (p *prep) fitOf(rn *RNode, pr *Prop) string {
	s := "fill"
	if word, ok := pr.text(1); ok {
		if word != "fit" {
			p.t.warn(pr.Pos, `image takes a path and fit fill, cover or contain: image "a.png" fit cover`)
			return s
		}
		s, _ = pr.text(2)
	}
	if fp := p.prop(rn, "fit"); fp != nil {
		s, _ = fp.text(0)
	}
	switch s {
	case "fill", "cover", "contain":
	case "":
		s = "fill"
	default:
		p.t.warn(pr.Pos, "fit takes fill, cover or contain, got %q; fill is used", s)
		s = "fill"
	}
	return s
}

// drawImage puts img into a box of w by h with the fit, the corners of radius cut off.
func drawImage(r *gfx.Recorder, img *gfx.Raster, fit string, w, h float64, radius [4]float64) {
	iw, ih := img.Size()
	if iw <= 0 || ih <= 0 {
		return
	}
	x, y, dw, dh := 0.0, 0.0, w, h
	switch fit {
	case "cover":
		s := math.Max(w/iw, h/ih)
		dw, dh = iw*s, ih*s
		x, y = (w-dw)/2, (h-dh)/2
	case "contain":
		s := math.Min(w/iw, h/ih)
		dw, dh = iw*s, ih*s
		x, y = (w-dw)/2, (h-dh)/2
	}
	cut := fit == "cover" || radius != [4]float64{}
	if cut {
		r.Save()
		if radius == [4]float64{} {
			r.ClipRect(gfx.R(0, 0, w, h))
		} else {
			r.ClipPath(gfx.RoundRectPath4(gfx.R(0, 0, w, h), radius))
		}
	}
	r.DrawImage(img, x, y, dw, dh)
	if cut {
		r.Restore()
	}
}

// turn is the transform of an item turned by deg degrees clockwise, as CSS counts them, about the center
// of its box at (x, y).
func turn(x, y, w, h, deg float64) gfx.Affine {
	cx, cy := w/2, h/2
	m := gfx.TranslateBy(cx, cy).Mul(gfx.RotateBy(deg * math.Pi / 180)).Mul(gfx.TranslateBy(-cx, -cy))
	return gfx.TranslateBy(x, y).Mul(m)
}

// rotation reads the static rotate of an item: degrees, clockwise, about the center of what it draws.
func (p *prep) rotation(rn *RNode) float64 {
	pr := p.prop(rn, "rotate")
	if pr == nil {
		return 0
	}
	if v, ok := pr.num(0); ok {
		return v
	}
	p.t.warn(pr.Pos, "rotate takes degrees, as in rotate 15; ignored")
	return 0
}

// radiiOf reads corner-radius of a node that has no style of its own, the root of a screen.
func (p *prep) radiiOf(rn *RNode) [4]float64 {
	if pr := p.prop(rn, "corner-radius"); pr != nil {
		return p.radii(pr)
	}
	return [4]float64{}
}
