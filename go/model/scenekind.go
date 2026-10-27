package model

import "github.com/haiodo/tenniarb/go/tenn"

// SceneElement picks the renderer of an element, the rule of spec 5a.17: go/scene when its own content
// uses a scene feature, go/render (Swift parity) otherwise.
func SceneElement(e *Element) bool {
	for _, p := range e.Properties.Nodes() {
		switch commandOf(p) {
		case "use":
			if useWithPath(p) {
				return true
			}
		case "item": // the classic model keeps the items at the root of a file as properties
			if b := lastBlock(p); b != nil && sceneBlock(b.Children) {
				return true
			}
		}
	}
	for _, it := range e.Items {
		if it.Kind != ItemKindLink && sceneBlock(it.Properties.Nodes()) {
			return true
		}
	}
	return false
}

// sceneBlock: the commands of an item give it child nodes or auto-layout.
func sceneBlock(cmds []*tenn.Node) bool {
	for _, p := range cmds {
		switch commandOf(p) {
		case "item", "slot", "auto-layout":
			return true
		case "use":
			if useWithPath(p) {
				return true
			}
		}
	}
	return false
}

func commandOf(n *tenn.Node) string {
	if n.Kind != tenn.KindCommand {
		return ""
	}
	s, _ := n.GetIdent(0)
	return s
}

func lastBlock(c *tenn.Node) *tenn.Node {
	if n := len(c.Children); n > 1 && c.Children[n-1].Kind == tenn.KindBlockExpr {
		return c.Children[n-1]
	}
	return nil
}

// useWithPath: a bare use (a user property, spec 9.4) has no arguments.
func useWithPath(n *tenn.Node) bool {
	return len(n.Children) > 1 && n.Children[1].Kind != tenn.KindBlockExpr
}

// DocKind says which renderers a document needs.
type DocKind string

const (
	DocDiagram DocKind = "diagram" // only classic elements: go/render
	DocScene   DocKind = "scene"   // scene elements and imports, no classic items
	DocMixed   DocKind = "mixed"
)

// KindOfModel classifies a document by SceneElement over its elements; an element with items that
// is not a scene element is classic. Imports make a document scene when nothing else decides.
func KindOfModel(m *ElementModel) DocKind {
	scene, classic, imports := false, false, false
	var walk func(e *Element)
	walk = func(e *Element) {
		switch {
		case SceneElement(e):
			scene = true
		case len(e.Items) > 0:
			classic = true
		}
		for _, p := range e.Properties.Nodes() {
			if commandOf(p) == "import" {
				imports = true
			}
		}
		for _, c := range e.Elements {
			walk(c)
		}
	}
	walk(&m.Element)
	switch {
	case (scene || imports) && classic:
		return DocMixed
	case scene || imports:
		return DocScene
	}
	return DocDiagram
}
