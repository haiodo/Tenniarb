package scene

import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"image"
	_ "image/gif" // decoders of the picture files an item names
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"math"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/markdown"
	"github.com/haiodo/tenniarb/go/tenn"
	_ "golang.org/x/image/webp"
)

// itemImage is a picture of an item and how it fills the box: the file of a scene, or the inline data
// of a classic `image "name" @(...)`. The raster is read when the item is built, not when it is laid
// out: validate lays out every element of the workspace and needs only the header.
type itemImage struct {
	key  imageKey
	data string // inline base64 of a classic image, "" for a file
	fit  string
	rel  string // the path or the name, for a warning
	pos  Pos
	w, h int
	// broken marks a picture that did not decode: the warning is once, the item stays empty.
	broken bool
}

// imageCacheBytes bounds the rasters kept between builds, at 4 bytes a pixel. A landing page of fifty
// photos stays decoded, so a rebuild reads no file twice; the least recently used go first.
const imageCacheBytes = 512 << 20

// imageKey is what a decoded picture is: the same path with other bytes is another picture; the data
// of an inline image is its path.
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

// fileImage reports whether pr is the `image "path"` of an item; the classic `image <name> @(...)` of an
// old diagram is not one.
func fileImage(pr *Prop) bool {
	return pr != nil && (pr.Val != nil || len(pr.Args) < 2 || pr.Args[1].Kind != tenn.KindImage)
}

// inlineImage reports whether pr is the classic `image "name" @(base64)` of an old diagram: the data is
// the picture, the name is how the text of the item refers to it.
func inlineImage(pr *Prop) bool {
	return pr != nil && pr.Val == nil && len(pr.Args) >= 2 && pr.Args[1].Kind == tenn.KindImage
}

// loadImage reads the picture of an item: the path is relative to the file that writes the property, as
// with `element --from-file`. A picture that cannot be drawn is a warning, so the item stays empty and
// check and validate pass.
func (p *prep) loadImage(rn *RNode) *itemImage {
	pr := p.prop(rn, "image")
	if p.classic && inlineImage(pr) {
		// An image the text names with @(name) is drawn there; one it does not name is the content of
		// the box, as a file image is.
		name, _ := pr.text(0)
		im := p.inlineImages(rn)[name]
		if im == nil || p.namesInline(rn, name) {
			return nil
		}
		return im
	}
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

// inlineImages collects the classic `image "name" @(data)` of rn by name: the pictures its text draws
// with @(name) or @(name|width).
func (p *prep) inlineImages(rn *RNode) map[string]*itemImage {
	if !p.classic {
		return nil
	}
	if imgs, ok := p.inlines[rn]; ok {
		return imgs
	}
	imgs := map[string]*itemImage{}
	p.inlines[rn] = imgs
	for _, pr := range rn.Props {
		if !inlineImage(pr) {
			continue
		}
		name, _ := pr.text(0)
		if name == "" {
			p.t.warn(pr.Pos, `image takes a name: image "photo" @(...)`)
			continue
		}
		if im := p.loadInline(rn, pr, name); im != nil {
			imgs[name] = im
		}
	}
	return imgs
}

// loadInline reads the header of an inline picture; the raster comes at the build of the screen.
func (p *prep) loadInline(rn *RNode, pr *Prop, name string) *itemImage {
	data, _ := pr.text(1)
	cfg, why := inlineConfig(data)
	if why != "" {
		p.t.warn(pr.Pos, "image %q: %s, the item is drawn empty", name, why)
		return nil
	}
	return &itemImage{data: data, fit: p.fitOf(rn, pr), rel: name, pos: pr.Pos,
		w: cfg.Width, h: cfg.Height}
}

// namesInline reports whether the title or the body of rn refers to the inline image name.
func (p *prep) namesInline(rn *RNode, name string) bool {
	if name == "" {
		return false
	}
	ref := "@(" + name
	for _, prop := range []string{"title", "body"} {
		if pr := p.prop(rn, prop); pr != nil && propRefers(pr, ref) {
			return true
		}
	}
	return false
}

// propRefers reports whether the text of a title or body command names ref: in an argument, an
// evaluated value or the string of its block.
func propRefers(pr *Prop, ref string) bool {
	has := func(nodes []*tenn.Node) bool {
		for _, n := range nodes {
			if s, _ := n.GetIdentText(); strings.Contains(s, ref) {
				return true
			}
		}
		return false
	}
	for _, v := range pr.Val {
		if strings.Contains(v, ref) {
			return true
		}
	}
	if has(pr.Args) {
		return true
	}
	if pr.Block != nil {
		for _, c := range pr.Block.Children {
			args, _ := split(c)
			if has(args) {
				return true
			}
		}
	}
	return false
}

var nonBase64 = regexp.MustCompile(`[^A-Za-z0-9+/=]`)

// decodeBase64 reads the payload of an inline image; unknown characters are dropped, as Swift's
// Data(base64Encoded:options:.ignoreUnknownCharacters) does.
func decodeBase64(data string) ([]byte, error) {
	s := strings.TrimRight(nonBase64.ReplaceAllString(data, ""), "=")
	return base64.RawStdEncoding.DecodeString(s)
}

// inlineConfig tells the pixel size of an inline picture without keeping its pixels.
func inlineConfig(data string) (image.Config, string) {
	raw, err := decodeBase64(data)
	if err != nil {
		return image.Config{}, "the data is not base64"
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return image.Config{}, whyNoImage(raw)
	}
	return cfg, ""
}

// decodeInline decodes an inline picture and keeps it in the raster cache, like a file.
func decodeInline(data string) (*gfx.Raster, string) {
	sum := sha256.Sum256([]byte(data))
	key := imageKey{path: "\x00inline:" + string(sum[:])}
	if img := imageCache.get(key); img != nil {
		return img, ""
	}
	raw, err := decodeBase64(data)
	if err != nil {
		return nil, "the data is not base64"
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

// raster decodes the picture once per build and reports one the header did not tell about as a warning.
func (im *itemImage) raster(p *prep) *gfx.Raster {
	if im.broken {
		return nil
	}
	var (
		img *gfx.Raster
		why string
	)
	if im.data != "" {
		img, why = decodeInline(im.data)
	} else {
		img, why = decodeImage(im.key)
	}
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
	if word, ok := pr.text(1); ok && !inlineImage(pr) {
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

// textImage is an inline picture drawn inside a title or a body: the picture to decode and the box the
// reference @(name|width) asks for.
type textImage struct {
	im   *itemImage
	w, h float64
}

func (t *textImage) raster(p *prep) *gfx.Raster {
	if t.im == nil {
		return nil
	}
	return t.im.raster(p)
}

// inlineProvider resolves the name and style of @(name|width) among the inline pictures of an item with
// the rules of go/render.
func inlineProvider(imgs map[string]*itemImage) *markdown.ImageProvider {
	if len(imgs) == 0 {
		return nil
	}
	prov := markdown.NewImageProvider(1)
	prov.Resolve = func(name string) *markdown.CachedImage {
		im := imgs[name]
		if im == nil {
			return nil
		}
		return &markdown.CachedImage{Image: name, Size: markdown.Size{Width: float64(im.w), Height: float64(im.h)}}
	}
	return prov
}

// resolveInline is the picture ref names, nil when the item has no inline image of that name.
func resolveInline(prov *markdown.ImageProvider, imgs map[string]*itemImage, ref string) *textImage {
	if prov == nil {
		return nil
	}
	res, ok := prov.ResolveImagePath(ref)
	if !ok {
		return nil
	}
	im := imgs[res.Image]
	if im == nil {
		return nil
	}
	return &textImage{im: im, w: res.Rect.Width, h: res.Rect.Height}
}

// splitTitleImages cuts a title at the @(name) references of the inline images of its item; ok is false
// when the title names none.
func (p *prep) splitTitleImages(title string, imgs map[string]*itemImage) ([]titlePart, bool) {
	prov := inlineProvider(imgs)
	if prov == nil {
		return nil, false
	}
	var parts []titlePart
	plain, found := 0, false
	for i := 0; i+1 < len(title); {
		if title[i] != '@' || title[i+1] != '(' {
			i++
			continue
		}
		j := strings.IndexByte(title[i+2:], ')')
		if j < 0 {
			break
		}
		img := resolveInline(prov, imgs, title[i+2:i+2+j])
		if img == nil {
			i += 2
			continue
		}
		if i > plain {
			parts = append(parts, titlePart{text: title[plain:i]})
		}
		parts = append(parts, titlePart{img: img})
		i += j + 3
		plain, found = i, true
	}
	if !found {
		return nil, false
	}
	if plain < len(title) {
		parts = append(parts, titlePart{text: title[plain:]})
	}
	return parts, true
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
