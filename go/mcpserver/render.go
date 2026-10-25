package mcpserver

import (
	"errors"
	"fmt"
	"strings"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
)

const (
	defaultScale = 1.0
	maxScale     = 8.0
	maxPixels    = 16 << 20
)

// rasterizer draws a built scene into PNG bytes. The platform file provides it.
type rasterizer interface {
	Measurer() gfx.TextMeasurer
	PNG(sc *gfx.Scene, root *gfx.Node, scale float64) ([]byte, error)
}

type rendered struct {
	png  []byte
	text string
}

func (s *Server) render(file, element string, scale float64) (*rendered, error) {
	_, rel, err := s.resolve(file)
	if err != nil {
		return nil, err
	}
	if scale == 0 {
		scale = defaultScale
	}
	if scale < 0 || scale > maxScale {
		return nil, fmt.Errorf("scale %v is outside (0, %v]", scale, maxScale)
	}
	rast, err := newRasterizer()
	if err != nil {
		return nil, err
	}
	diags, tree, err := s.diagnose(rel)
	if err != nil {
		return nil, err
	}
	if tree == nil || diags.Fatal() {
		return nil, fmt.Errorf("cannot render, fix the errors first:\n%s", summary(diags, false))
	}
	if element == "." {
		element = ""
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.Build(tx, nil, element, scene.Options{Measurer: rast.Measurer(), FontName: "Helvetica"})
	})
	if err != nil {
		return nil, err
	}
	var bounds gfx.Rect
	sc.Commit()
	sc.Read(func() { bounds = screen.Root.SubtreeBounds() })
	if bounds.Empty() {
		return nil, errors.New("the element has nothing to draw")
	}
	if px := bounds.W() * scale * bounds.H() * scale; px > maxPixels {
		return nil, fmt.Errorf("image would be %.0f pixels (limit %d): lower scale or render a smaller element", px, maxPixels)
	}
	png, err := rast.PNG(sc, screen.Root, scale)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "rendered %s %q: %.0fx%.0f scene units, scale %v\n", rel, element, bounds.W(), bounds.H(), scale)
	for _, w := range screen.Warnings {
		b.WriteString(w.Error() + "\n")
	}
	fmt.Fprintf(&b, "%d warnings", len(screen.Warnings))
	return &rendered{png: png, text: b.String()}, nil
}
