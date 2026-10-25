package markdown

import "strconv"

type Size struct{ Width, Height float64 }

type Rect struct{ X, Y, Width, Height float64 }

// CachedImage.Image is whatever the renderer loads (e.g. a data URL), it is not interpreted here.
type CachedImage struct {
	Image string
	Size  Size
}

// ResolvedImage is the result of ImagePathResolver.
type ResolvedImage struct {
	Image string
	Rect  Rect
}

// ImagePathResolver is what ToHTML needs from an ImageProvider.
type ImagePathResolver interface {
	ResolveImagePath(path string) (ResolvedImage, bool)
}

// ImageProvider caches images inside items with required processing.
type ImageProvider struct {
	ScaleFactor float64
	Images      map[string]*CachedImage
	// Resolve loads an image by name; nil means "not implemented", as in the Swift base class.
	Resolve func(name string) *CachedImage
}

func NewImageProvider(scaleFactor float64) *ImageProvider {
	return &ImageProvider{ScaleFactor: scaleFactor, Images: map[string]*CachedImage{}}
}

// parseInt10 is Swift Int(_:radix: 10): optional sign and ASCII digits only, false outside Int64.
func parseInt10(s string) (float64, bool) {
	v, err := strconv.ParseInt(s, 10, 64)
	return float64(v), err == nil
}

// ResolveImagePath: path is name and style combined, parsed from @(name|style).
func (p *ImageProvider) ResolveImagePath(path string) (ResolvedImage, bool) {
	// If image already cached.
	if image, ok := p.Images[path]; ok {
		return ResolvedImage{Image: image.Image, Rect: Rect{Width: image.Size.Width, Height: image.Size.Height}}, true
	}
	name := path
	style := ""
	if pos := firstIndexOf(path, "|"); pos >= 0 {
		name = path[:pos]
		style = path[pos+1:]
	}

	// Retrieve image data from properties, if not cached
	if p.Resolve == nil {
		return ResolvedImage{}, false
	}
	image := p.Resolve(name)
	if image == nil {
		return ResolvedImage{}, false
	}

	width := image.Size.Width
	height := image.Size.Height
	if style != "" {
		widthStr := style
		heightStr := ""
		// We need to apply style is applicable
		if xPos := firstIndexOf(style, "x"); xPos >= 0 {
			widthStr = trimmed(style[:xPos])
			heightStr = trimmed(style[xPos+1:])
		}

		// This is aspect scale.
		if widthStr != "" {
			if v, ok := parseInt10(widthStr); ok {
				width = v
			}
		}
		if heightStr != "" {
			if v, ok := parseInt10(heightStr); ok {
				height = v
			}
		}

		if widthStr == "" || heightStr == "" {
			r := GetMaxRect(width, height, image.Size.Width, image.Size.Height)
			width = r.Width
			height = r.Height
		}
	}
	// The Swift version also rescales the bitmap to size * scaleFactor; the renderer does that when drawing.
	image.Size = Size{width, height}

	if p.Images == nil {
		p.Images = map[string]*CachedImage{}
	}
	p.Images[path] = image
	return ResolvedImage{Image: image.Image, Rect: Rect{Width: width, Height: height}}, true
}

func GetMaxRect(maxWidth, maxHeight, imageWidth, imageHeight float64) Rect {
	// Get ratio (landscape or portrait)
	ratiox := maxWidth / imageWidth
	ratioy := maxHeight / imageHeight

	// Swift.min(a, b) is b < a ? b : a, which differs from math.Min on NaN.
	ratio := ratiox
	if ratioy < ratiox {
		ratio = ratioy
	}
	if ratio > 1 {
		ratio = 1
	}
	return Rect{Width: imageWidth * ratio, Height: imageHeight * ratio}
}
