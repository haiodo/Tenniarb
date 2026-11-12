package jsoncanvas

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenn/go"
	"github.com/haiodo/tenniarb/go/scene"
)

// FromTree converts a loaded .tenn file: the items of the file root, then every top level element as
// a group. A file of one element and no root content is that element's content, as FromElement.
func FromTree(t *scene.Tree) (*Canvas, []string, error) {
	main, err := mainFile(t)
	if err != nil {
		return nil, nil, err
	}
	if len(main.Elems) == 1 && len(main.Content) == 0 {
		return FromElement(t, main.Elems[0])
	}
	e := newExport(t)
	screen, err := e.content(main, 0, 0)
	if err != nil {
		return nil, nil, err
	}
	bottom := 0.0
	if b := bounds(screen.Placed().Kids); len(screen.Placed().Kids) > 0 {
		bottom = b.y + b.h
	}
	if len(main.Elems) > 0 {
		e.rep.add("elements are not part of JSON Canvas; each top level element becomes a group")
	}
	offset := bottom + 80
	for _, el := range main.Elems {
		s, err := e.build(el)
		if err != nil {
			e.rep.add("element %q is skipped: %v", el.Name, err)
			continue
		}
		b := bounds(s.Placed().Kids)
		if b.w == 0 && b.h == 0 {
			b = rect{0, 0, s.W, s.H}
		}
		dy := offset - b.y
		e.addGroup(el.Name, b.x, offset, b.w, b.h)
		for _, pl := range s.Placed().Kids {
			e.walk(pl, 0, dy)
		}
		e.links(el)
		e.elementLoss(el)
		offset += b.h + 80
	}
	return e.c, e.rep.list, nil
}

// FromElement converts the content of one element, without the group around it: a node of the
// canvas can stand for a group, and an extra group would hide the element's own items.
func FromElement(t *scene.Tree, el *scene.Node) (*Canvas, []string, error) {
	if t == nil {
		return nil, nil, errors.New("json canvas: nil tree")
	}
	if el == nil {
		return nil, nil, errors.New("json canvas: nil element")
	}
	e := newExport(t)
	if _, err := e.content(el, 0, 0); err != nil {
		return nil, nil, err
	}
	e.elementLoss(el)
	return e.c, e.rep.list, nil
}

func mainFile(t *scene.Tree) (*scene.Node, error) {
	if t == nil {
		return nil, errors.New("json canvas: nil tree")
	}
	files := t.Files()
	if len(files) == 0 {
		return nil, errors.New("json canvas: the tree has no files")
	}
	main := t.File(files[0])
	if main == nil {
		return nil, errors.New("json canvas: the main file is not in the tree")
	}
	return main, nil
}

func newExport(t *scene.Tree) *export {
	return &export{t: t, c: &Canvas{Nodes: []Node{}, Edges: []Edge{}}, ids: map[*scene.Node]string{}, rep: &report{}}
}

// content adds the placed items of an element and its links.
func (e *export) content(el *scene.Node, x, y float64) (*scene.Screen, error) {
	s, err := e.build(el)
	if err != nil {
		return nil, err
	}
	for _, pl := range s.Placed().Kids {
		e.walk(pl, x, y)
	}
	e.links(el)
	return s, nil
}

type export struct {
	t     *scene.Tree
	c     *Canvas
	ids   map[*scene.Node]string
	rep   *report
	nodes int
	edges int
}

type rect struct{ x, y, w, h float64 }

func bounds(items []*scene.Placed) rect {
	if len(items) == 0 {
		return rect{}
	}
	b := rect{items[0].X, items[0].Y, items[0].W, items[0].H}
	var walk func([]*scene.Placed)
	walk = func(ps []*scene.Placed) {
		for _, p := range ps {
			b.x = min(b.x, p.X)
			b.y = min(b.y, p.Y)
			b.w = max(b.w, p.X+p.W)
			b.h = max(b.h, p.Y+p.H)
			walk(p.Kids)
		}
	}
	walk(items)
	b.w -= b.x
	b.h -= b.y
	return b
}

func (e *export) build(el *scene.Node) (*scene.Screen, error) {
	sc := gfx.NewScene()
	var screen *scene.Screen
	var err error
	sc.Update(func(tx *gfx.Tx) {
		screen, err = e.t.BuildElement(tx, nil, el, scene.Options{})
	})
	if err != nil {
		return nil, err
	}
	return screen, nil
}

func (e *export) addGroup(label string, x, y, w, h float64) string {
	id := e.id()
	e.c.Nodes = append(e.c.Nodes, Node{ID: id, Type: "group", X: x, Y: y, Width: w, Height: h, Label: label})
	return id
}

func (e *export) id() string {
	e.nodes++
	return fmt.Sprintf("n%d", e.nodes)
}

// walk adds one placed item and its children. x and y shift the coordinates of an element that is
// placed below the previous one.
func (e *export) walk(pl *scene.Placed, x, y float64) string {
	src := pl.Src
	if src == nil {
		return ""
	}
	if src.Kind == scene.KUse {
		e.rep.add("use %q is not exported: JSON Canvas has no components", src.Name)
		return ""
	}
	e.loss(pl)
	id := e.id()
	node := Node{ID: id, X: x + pl.X, Y: y + pl.Y, Width: pl.W, Height: pl.H,
		Color: e.color(pl.Prop("color"))}
	e.ids[src] = id
	switch {
	case len(pl.Kids) > 0:
		node.Type = "group"
		node.Label = propText(pl, "title")
		if node.Label == "" {
			node.Label = src.Name
		}
		e.c.Nodes = append(e.c.Nodes, node)
		for _, k := range pl.Kids {
			e.walk(k, x, y)
		}
	case fileImage(pl.Prop("image")):
		node.Type = "file"
		node.File = propText(pl, "image")
		if inline(pl.Prop("image")) {
			e.rep.add("item %q: a base64 image is not written into the canvas, only its name %q", src.Name, node.File)
		}
		e.c.Nodes = append(e.c.Nodes, node)
	default:
		title, body := propText(pl, "title"), propText(pl, "body")
		if title == "" && body == "" {
			title = src.Name
		}
		if url, ok := singleLink(body); ok && (title == "" || title == url) {
			node.Type, node.URL = "link", url
		} else {
			node.Type = "text"
			node.Text = title
			if body != "" && title != "" {
				node.Text += "\n"
			}
			node.Text += body
		}
		e.c.Nodes = append(e.c.Nodes, node)
	}
	return id
}

// fileImage is the `image "path"` form, not the classic `image "name" @(base64)`.
func fileImage(pr *scene.Prop) bool {
	return pr != nil && textOf(pr) != "" && !inline(pr)
}

func inline(pr *scene.Prop) bool {
	return pr != nil && len(pr.Args) >= 2 && pr.Args[1].Kind == tenn.KindImage
}

var linkRe = regexp.MustCompile(`^\[[^\]]*\]\(([^)\s]+)\)$`)

func singleLink(body string) (string, bool) {
	m := linkRe.FindStringSubmatch(strings.TrimSpace(body))
	if m == nil {
		return "", false
	}
	return m[1], true
}

func propText(pl *scene.Placed, name string) string {
	if pr := pl.Prop(name); pr != nil {
		return textOf(pr)
	}
	return ""
}

// textOf is the first argument of a property as text.
func textOf(pr *scene.Prop) string {
	if pr == nil {
		return ""
	}
	vals := pr.Values()
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func (e *export) color(pr *scene.Prop) string {
	word := strings.ToLower(strings.TrimSpace(textOf(pr)))
	switch word {
	case "", "none", "transparent":
		return ""
	}
	if word == "linear" {
		e.rep.add("a gradient is not a JSON Canvas colour, the fill is left out")
		return ""
	}
	key, ok := presetKey(word)
	if !ok {
		e.rep.add("colour %q has no JSON Canvas form, the fill is left out", word)
		return ""
	}
	if name, _ := presetName(key); name != word {
		e.rep.add("colour %q is approximated by JSON Canvas colour %q", word, key)
	}
	return key
}

// loss reports the properties JSON Canvas has no field for.
func (e *export) loss(pl *scene.Placed) {
	for _, name := range []string{"auto-layout", "use-style", "layer", "rotate", "hidden", "animation", "opacity"} {
		if pl.Prop(name) != nil {
			e.rep.add("item %q: %s is lost in JSON Canvas", pl.Src.Name, name)
		}
	}
	if pr := pl.Prop("display"); pr != nil {
		if word := textOf(pr); word != "" && word != "rect" {
			e.rep.add("item %q: display %s is lost in JSON Canvas", pl.Src.Name, word)
		}
	}
	for _, pr := range pl.Props() {
		if hasExpr(pr) {
			e.rep.add("item %q: expressions are not evaluated, the canvas holds the written value", pl.Src.Name)
			break
		}
	}
}

func (e *export) elementLoss(el *scene.Node) {
	if el.Mount != nil {
		e.rep.add("element %q: the mount of %q is flattened into a group", el.Name, el.Mount.Pos.File)
	}
	if len(el.Styles) > 0 {
		e.rep.add("element %q: styles are lost in JSON Canvas", el.Name)
	}
	if len(el.Params) > 0 {
		e.rep.add("element %q: parameters are lost in JSON Canvas", el.Name)
	}
	if el.Template != nil {
		e.rep.add("element %q: the template is lost in JSON Canvas", el.Name)
	}
	if len(el.Elems) > 0 {
		e.rep.add("element %q: nested elements are not exported", el.Name)
	}
	for _, pr := range el.Props {
		if hasExpr(pr) {
			e.rep.add("element %q: expressions are not evaluated, the canvas holds the written value", el.Name)
			break
		}
	}
}

func hasExpr(pr *scene.Prop) bool {
	for _, a := range pr.Args {
		switch a.Kind {
		case tenn.KindExpression, tenn.KindExpressionBlock:
			return true
		case tenn.KindStringLit:
			if s, ok := a.GetIdentText(); ok && strings.Contains(s, "${") {
				return true
			}
		}
	}
	return false
}

func (e *export) links(parent *scene.Node) {
	for _, c := range parent.Content {
		switch c.Kind {
		case scene.KLink:
			e.edge(c)
		case scene.KItem:
			if e.ids[c] != "" {
				e.links(c)
			}
		}
	}
}

func (e *export) edge(lk *scene.Node) {
	from := e.end(lk, lk.From, "source-index")
	to := e.end(lk, lk.To, "target-index")
	if from == "" || to == "" {
		e.rep.add("link %q %q is not exported: an end is a use or is not drawn", lk.From, lk.To)
		return
	}
	e.edges++
	edge := Edge{ID: fmt.Sprintf("e%d", e.edges), FromNode: from, ToNode: to}
	if pr := findProp(lk, "label"); pr != nil {
		edge.Label = textOf(pr)
	}
	if pr := findProp(lk, "display"); pr != nil && textOf(pr) == "arrow" {
		edge.ToEnd = "arrow"
	}
	if pr := findProp(lk, "color"); pr != nil {
		edge.Color = e.color(pr)
	}
	if pr := findProp(lk, "route"); pr != nil {
		vals := pr.Values()
		if len(vals) > 0 && vals[0] == "elbow" {
			if len(vals) > 1 {
				edge.FromSide = vals[1]
			}
			if len(vals) > 2 {
				edge.ToSide = vals[2]
			}
			if !side(edge.FromSide) || !side(edge.ToSide) {
				e.rep.add("link %q %q: side %q %q is not a JSON Canvas side", lk.From, lk.To, edge.FromSide, edge.ToSide)
				edge.FromSide, edge.ToSide = "", ""
			}
		}
	}
	e.c.Edges = append(e.c.Edges, edge)
}

func side(s string) bool {
	return s == "top" || s == "right" || s == "bottom" || s == "left"
}

// end is the canvas id of the n-th sibling item with the name of a link end.
func (e *export) end(lk *scene.Node, name, indexProp string) string {
	idx := 0
	if pr := findProp(lk, indexProp); pr != nil {
		if v, ok := pr.Num(0); ok {
			idx = int(v)
		}
	}
	n := 0
	for _, c := range lk.Parent.Content {
		if c.Kind != scene.KItem || c.Name != name {
			continue
		}
		if n == idx {
			return e.ids[c]
		}
		n++
	}
	return ""
}

func findProp(n *scene.Node, name string) *scene.Prop {
	for _, pr := range n.Props {
		if pr.Name == name {
			return pr
		}
	}
	return nil
}
