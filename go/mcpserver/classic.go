package mcpserver

import (
	"errors"
	"fmt"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/scene"
)

const (
	rendererScene   = "scene"
	rendererDiagram = "diagram"
)

// renderers picks the renderer of elements with model.SceneElement, the rule the app draws by, so
// a slot written directly in an element is a property there, as go/scene reads it. The classic
// model of each file is parsed once.
type renderers struct {
	tree   *scene.Tree
	models map[string]*model.ElementModel
}

func newRenderers(tree *scene.Tree) *renderers {
	return &renderers{tree: tree, models: map[string]*model.ElementModel{}}
}

func (r *renderers) of(n *scene.Node) string {
	if n.Mount != nil {
		n = n.Mount // an element with --from-file is the root of its file
	}
	m, ok := r.models[n.Pos.File]
	if !ok {
		src, _ := r.tree.Source(n.Pos.File)
		m = model.ReadTenn(src)
		r.models[n.Pos.File] = m
	}
	if m == nil {
		return rendererDiagram
	}
	el, err := diagramElement(&m.Element, n)
	if err == nil && model.SceneElement(el) {
		return rendererScene
	}
	return rendererDiagram
}

func hasContent(n *scene.Node) bool { return len(n.Content) > 0 }

// elementAt resolves the element a render names: "" is the file itself, an import alias is the
// imported file.
func elementAt(tree *scene.Tree, rel, path string) (*scene.Node, error) {
	root := tree.File(rel)
	if root == nil {
		return nil, fmt.Errorf("%s is not loaded", rel)
	}
	if path == "" {
		return root, nil
	}
	n, err := tree.Resolve(root, path)
	if err != nil {
		return nil, err
	}
	if n.Kind == scene.KMount {
		n = n.Target
	}
	if n.Kind != scene.KElement && n.Kind != scene.KFile {
		return nil, fmt.Errorf("path %q names a %s, not an element: render its element", path, n.Kind)
	}
	return n, nil
}

// diagramElement finds the go/model element of n by the chain of element names from its file root.
func diagramElement(root *model.Element, n *scene.Node) (*model.Element, error) {
	var names []string
	for a := n; a != nil && a.Kind == scene.KElement; a = a.Parent {
		names = append([]string{a.Name}, names...)
	}
	el := root
	for _, name := range names {
		var next *model.Element
		for _, c := range el.Elements {
			if c.Name == name {
				next = c
				break
			}
		}
		if next == nil {
			return nil, fmt.Errorf("element %q is missing from the diagram model", name)
		}
		el = next
	}
	return el, nil
}

// renderDiagram draws a classic element with go/render, the renderer of the app's canvas.
func renderDiagram(r *renderers, n *scene.Node, rel, element string, scale float64) (*rendered, error) {
	if n.Mount != nil {
		n = n.Mount
	}
	m := r.models[n.Pos.File]
	if m == nil {
		return nil, fmt.Errorf("%s does not parse as a diagram", n.Pos.File)
	}
	el, err := diagramElement(&m.Element, n)
	if err != nil {
		return nil, err
	}
	if len(el.Items) == 0 {
		return nil, errors.New("the element has nothing to draw")
	}
	opts := render.Options{FontFamily: "Helvetica"}
	size := render.SceneSize(el, opts)
	if px := size.Width * scale * size.Height * scale; px > maxPixels {
		return nil, fmt.Errorf("image would be %.0f pixels (limit %d): lower scale or render a smaller element", px, maxPixels)
	}
	png, err := render.PNG(el, opts, scale, gfx.Color{R: 1, G: 1, B: 1, A: 1})
	if err != nil {
		return nil, err
	}
	text := fmt.Sprintf("rendered %s %q as a diagram: %.0fx%.0f units, scale %v\n0 warnings", rel, element, size.Width, size.Height, scale)
	return &rendered{png: png, text: text}, nil
}
