package render

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/gif" // decoders for inline images
	_ "image/jpeg"
	_ "image/png"
	"regexp"
	"strings"

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

// StdImageDecoder decodes PNG, JPEG and GIF with the standard library.
func StdImageDecoder(b64 string) *DecodedImage {
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(b64, "="))
	if err != nil {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	r := gfx.RasterFrom(img)
	return &DecodedImage{Image: r, Width: float64(r.W), Height: float64(r.H)}
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
