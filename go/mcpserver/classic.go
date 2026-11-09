package mcpserver

import (
	"fmt"

	"github.com/haiodo/tenniarb/go/model"
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

// elementAt resolves the element a render names: "" is the file itself.
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
