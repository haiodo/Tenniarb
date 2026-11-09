package mcpserver

import (
	"errors"
	"fmt"
	"strings"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/render"
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

// buildScene lays the element out as the canvas does; measurer gives the text metrics of the platform.
// A classic diagram takes its ${...} values from go/render, the profile scene builds for it.
func (s *Server) buildScene(tree *scene.Tree, rel, element string, measurer gfx.TextMeasurer) (*gfx.Scene, *scene.Screen, error) {
	sc := gfx.NewScene()
	var screen *scene.Screen
	var err error
	classic := render.ClassicHook(tree, s.root, rel, element)
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.Build(tx, nil, element, scene.Options{Measurer: measurer, FontName: "Helvetica", EvalVars: exec.SceneEvalVars, Classic: classic})
	})
	if err != nil {
		return nil, nil, err
	}
	return sc, screen, nil
}

// rasterizerFor is newRasterizer, or a stand-in a test uses to check a platform without a backend.
var rasterizerFor = newRasterizer

// layoutOf builds an element and reports its geometry, without rasterizing anything.
func (s *Server) layoutOf(file, element string, levels int) (string, error) {
	_, rel, err := s.resolve(file)
	if err != nil {
		return "", err
	}
	diags, tree, err := s.diagnose(rel)
	if err != nil {
		return "", err
	}
	if tree == nil || diags.Fatal() {
		return "", fmt.Errorf("cannot lay out, fix the errors first:\n%s", summary(diags, false, rel))
	}
	if element == "." {
		element = ""
	}
	if _, err := elementAt(tree, rel, element); err != nil {
		return "", err
	}
	// Text metrics are all layout needs from the platform: where there is no CoreGraphics (CI on Linux
	// and Windows) estimate them instead of failing, since a render there would not be checked anyway.
	measurer := gfx.TextMeasurer(gfx.EstimateText{})
	if rast, err := rasterizerFor(); err == nil {
		measurer = rast.Measurer()
	}
	s.host.renderMu.Lock()
	defer s.host.renderMu.Unlock()
	sc, screen, err := s.buildScene(tree, rel, element, measurer)
	if err != nil {
		return "", err
	}
	sc.Commit()
	return fmt.Sprintf("%s %q\n%s", rel, element, inspectScene(screen, levels)), nil
}

type rendered struct {
	png    []byte
	text   string
	screen *scene.Screen
}

func (s *Server) render(file, element string, scale float64, view bool) (*rendered, error) {
	abs, rel, err := s.resolve(file)
	if err != nil {
		return nil, err
	}
	if scale == 0 {
		scale = defaultScale
	}
	if scale < 0 || scale > maxScale {
		return nil, fmt.Errorf("scale %v is outside (0, %v]", scale, maxScale)
	}
	if view {
		return s.renderView(abs, rel, scale)
	}
	diags, tree, err := s.diagnose(rel)
	if err != nil {
		return nil, err
	}
	if tree == nil || diags.Fatal() {
		return nil, fmt.Errorf("cannot render, fix the errors first:\n%s", summary(diags, false, rel))
	}
	if element == "." {
		element = ""
	}
	if _, err := elementAt(tree, rel, element); err != nil {
		return nil, err
	}
	s.host.renderMu.Lock()
	defer s.host.renderMu.Unlock()
	rast, err := newRasterizer()
	if err != nil {
		return nil, err
	}
	sc, screen, err := s.buildScene(tree, rel, element, rast.Measurer())
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
	fmt.Fprintf(&b, "rendered %s %q as a scene: %.0fx%.0f units, scale %v\n", rel, element, bounds.W(), bounds.H(), scale)
	// A library warning repeats once per instance: count those, list the file's own.
	other := 0
	for _, w := range screen.Warnings {
		if w.Pos.File != rel {
			other++
			continue
		}
		b.WriteString(w.Error() + "\n")
	}
	if other > 0 {
		fmt.Fprintf(&b, "%d warnings come from imported files: validate them to see these\n", other)
	}
	fmt.Fprintf(&b, "%d warnings", len(screen.Warnings))
	return &rendered{png: png, text: b.String(), screen: screen}, nil
}
