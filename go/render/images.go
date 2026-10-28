package render

import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"encoding/base64"
	"image"
	_ "image/gif" // decoders for inline images
	_ "image/jpeg"
	_ "image/png"
	"regexp"
	"strings"
	"sync"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/markdown"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/tenn"
)

// DecodedImage is an inline image ready to draw; Width and Height are its natural size.
type DecodedImage struct {
	Image         gfx.Image
	Width, Height float64
}

// ImageDecoder turns the (sanitized) base64 payload of an `image name data` property into an image, nil
// when it does not decode. Truncated or broken data must give nil.
type ImageDecoder func(base64 string) *DecodedImage

var nonBase64 = regexp.MustCompile(`[^A-Za-z0-9+/=]`)

// sanitizeBase64 is Data(base64Encoded:, options: .ignoreUnknownCharacters).
func sanitizeBase64(data string) string { return nonBase64.ReplaceAllString(data, "") }

// imageCacheBytes bounds the pixels StdImageDecoder keeps; the least recently used images go first.
const imageCacheBytes = 128 << 20

var imageCache = &decodedCache{limit: imageCacheBytes, byKey: map[[sha256.Size]byte]*list.Element{}, order: list.New()}

type decodedCache struct {
	mu    sync.Mutex
	limit int
	size  int
	byKey map[[sha256.Size]byte]*list.Element
	order *list.List // front is the most recently used; values are *cachedImage
}

type cachedImage struct {
	key [sha256.Size]byte
	img *DecodedImage
}

func (c *decodedCache) get(key [sha256.Size]byte) *DecodedImage {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.byKey[key]; ok {
		c.order.MoveToFront(el)
		return el.Value.(*cachedImage).img
	}
	return nil
}

// put keeps img and returns it, or the image a concurrent decode stored first.
func (c *decodedCache) put(key [sha256.Size]byte, img *DecodedImage) *DecodedImage {
	size := int(img.Width * img.Height * 4)
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.byKey[key]; ok {
		return el.Value.(*cachedImage).img
	}
	if size > c.limit {
		return img
	}
	c.byKey[key] = c.order.PushFront(&cachedImage{key: key, img: img})
	c.size += size
	for c.size > c.limit {
		last := c.order.Back()
		ci := last.Value.(*cachedImage)
		c.order.Remove(last)
		delete(c.byKey, ci.key)
		c.size -= int(ci.img.Width * ci.img.Height * 4)
	}
	return img
}

// StdImageDecoder decodes PNG, JPEG and GIF with the standard library. The same payload gives the same
// image (and Raster pointer) in every scene: a renderer that caches rasters by pointer must not see a new
// one on each rebuild.
func StdImageDecoder(b64 string) *DecodedImage {
	key := sha256.Sum256([]byte(b64))
	if img := imageCache.get(key); img != nil {
		return img
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(b64, "="))
	if err != nil {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	r := gfx.RasterFrom(img)
	return imageCache.put(key, &DecodedImage{Image: r, Width: float64(r.W), Height: float64(r.H)})
}

// ElementImageProvider is ElementImageProvider from ElementScene.swift: it caches images inside an item.
type ElementImageProvider struct {
	*markdown.ImageProvider
	Item   *model.DiagramItem
	decode ImageDecoder
	// decoded maps the base64 string the markdown package keeps as CachedImage.Image to the image.
	decoded map[string]*DecodedImage
}

func NewElementImageProvider(item *model.DiagramItem, scaleFactor float64, decode ImageDecoder) *ElementImageProvider {
	p := &ElementImageProvider{ImageProvider: markdown.NewImageProvider(scaleFactor), Item: item, decode: decode, decoded: map[string]*DecodedImage{}}
	p.Resolve = p.resolveImage
	return p
}

func (p *ElementImageProvider) resolveImage(name string) *markdown.CachedImage {
	var result *markdown.CachedImage
	if p.decode == nil || p.Item == nil || p.Item.Properties == nil {
		return nil
	}
	p.Item.Properties.AsNode().Traverse(func(child *tenn.Node) bool {
		cmdName, _ := child.GetIdent(0)
		imgName, ok1 := child.GetIdent(1)
		imgData, ok2 := child.GetIdent(2)
		if cmdName == "image" && ok1 && imgName == name && ok2 {
			b64 := sanitizeBase64(imgData)
			if img := p.decode(b64); img != nil {
				p.decoded[b64] = img
				result = &markdown.CachedImage{Image: b64, Size: markdown.Size{Width: img.Width, Height: img.Height}}
				return false
			}
		}
		return true
	})
	return result
}
