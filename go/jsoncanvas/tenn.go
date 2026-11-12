package jsoncanvas

import (
	"errors"
	"fmt"
	"math"
	"path"
	"strings"

	"github.com/haiodo/tenn/go"
)

// ToTenn converts a canvas to the text of a .tenn file: one element named name holds an item per
// node, groups become containers and edges become links. An empty name means "Canvas".
func ToTenn(c *Canvas, name string) (string, []string, error) {
	if c == nil {
		return "", nil, errors.New("json canvas: nil canvas")
	}
	if strings.TrimSpace(name) == "" {
		name = "Canvas"
	}
	cv := &importConv{c: c, byID: map[string]*inNode{}, rep: &report{}}
	cv.build()
	stmts := tenn.NewNode(tenn.KindStatements, nil)
	for _, r := range cv.roots {
		stmts.Add(cv.item(r))
	}
	for _, l := range cv.rootLinks {
		stmts.Add(l)
	}
	el := tenn.NewCommand("element", tenn.NewStrNode(name), tenn.NewBlockExpr(stmts.Children...))
	return tenn.CompactAs(el, tenn.CompactStyle{Indent: "\t", Pack: true}) + "\n", cv.rep.list, nil
}

// inNode is a canvas node in the group tree the import builds by containment.
type inNode struct {
	node   *Node
	name   string
	parent *inNode
	kids   []*inNode
	links  []*tenn.Node
}

type importConv struct {
	c         *Canvas
	byID      map[string]*inNode
	roots     []*inNode
	rootLinks []*tenn.Node
	rep       *report
}

type report struct {
	list []string
	seen map[string]bool
}

func (r *report) add(format string, args ...any) {
	if r.seen == nil {
		r.seen = map[string]bool{}
	}
	msg := fmt.Sprintf(format, args...)
	if r.seen[msg] {
		return
	}
	r.seen[msg] = true
	r.list = append(r.list, msg)
}

// build makes the containment tree: the smallest group that contains a node holds it. A group
// inside a group must be strictly larger, so equal boxes cannot form a cycle.
func (cv *importConv) build() {
	var all, groups []*inNode
	for i := range cv.c.Nodes {
		n := &cv.c.Nodes[i]
		in := &inNode{node: n}
		cv.byID[n.ID] = in
		all = append(all, in)
		if n.Type == "group" {
			groups = append(groups, in)
		}
	}
	for _, n := range all {
		var best *inNode
		for _, g := range groups {
			if g == n || !contains(g.node, n.node) {
				continue
			}
			if n.node.Type == "group" && area(g.node) <= area(n.node) {
				continue
			}
			if best == nil || area(g.node) < area(best.node) ||
				area(g.node) == area(best.node) && g.node.ID < best.node.ID {
				best = g
			}
		}
		n.parent = best
	}
	for _, n := range all {
		if n.parent != nil {
			n.parent.kids = append(n.parent.kids, n)
		} else {
			cv.roots = append(cv.roots, n)
		}
	}
	cv.name(cv.roots)
	for _, e := range cv.c.Edges {
		cv.edge(e)
	}
}

func contains(group, n *Node) bool {
	const eps = 1e-6
	return n.X >= group.X-eps && n.Y >= group.Y-eps &&
		n.X+n.Width <= group.X+group.Width+eps && n.Y+n.Height <= group.Y+group.Height+eps
}

func area(n *Node) float64 { return n.Width * n.Height }

// name gives every sibling set unique item names; edges refer to the ends by these names.
func (cv *importConv) name(siblings []*inNode) {
	used := map[string]int{}
	for _, n := range siblings {
		base := cv.baseName(n.node)
		name := base
		if k := used[base]; k > 0 {
			name = fmt.Sprintf("%s %d", base, k+1)
		}
		used[base]++
		n.name = name
		cv.name(n.kids)
	}
}

func (cv *importConv) baseName(n *Node) string {
	var base string
	switch n.Type {
	case "group":
		base = n.Label
	case "text":
		base, _ = splitText(n.Text)
	case "file":
		if !external(n.File) {
			base = path.Base(n.File)
		}
	case "link":
		base = n.URL
	}
	base = strings.TrimSpace(base)
	if base == "" {
		base = n.Type
	}
	return base
}

// splitText takes the first line as the title and leaves the rest as markdown. Both halves are the
// original text: only the line break between them is not written.
func splitText(text string) (title, body string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		return text[:i], text[i+1:]
	}
	return text, ""
}

func (cv *importConv) item(in *inNode) *tenn.Node {
	n := in.node
	var body []*tenn.Node
	body = append(body, cmd("pos", num(n.X-in.parentX()), num(-(n.Y-in.parentY()))))
	body = append(body, cmd("width", num(n.Width)), cmd("height", num(n.Height)))

	switch n.Type {
	case "text":
		title, rest := splitText(n.Text)
		body = append(body, cmd("title", str(title)))
		if rest != "" {
			body = append(body, cmd("body", str(rest)))
		}
	case "file":
		switch {
		case external(n.File):
			body = append(body, cmd("title", str(n.File)), cmd("body", str(linkText(n.File))))
			cv.rep.add("file node %q: %q is outside the workspace and became a link item", n.ID, n.File)
		default:
			body = append(body, cmd("image", str(n.File)))
			if n.Subpath != "" {
				cv.rep.add("file node %q: subpath %q is lost, an image has no anchor", n.ID, n.Subpath)
			}
		}
	case "link":
		body = append(body, cmd("title", str(n.URL)), cmd("body", str(linkText(n.URL))))
	case "group":
		if n.Label != "" {
			body = append(body, cmd("title", str(n.Label)))
		}
		if n.Background != "" || n.BackgroundStyle != "" {
			cv.rep.add("group %q: background %q is lost, .tenn groups have no background image", n.ID, n.Background)
		}
	}
	if c := cv.color(n); c != nil {
		body = append(body, c)
	} else if n.Type == "group" {
		// A white default box would cover the canvas; a frame keeps the children visible.
		body = append(body, cmd("color", str("none")), cmd("border-color", str("grey-400")))
	}
	for _, k := range in.kids {
		body = append(body, cv.item(k))
	}
	body = append(body, in.links...)
	return withBlock(tenn.NewCommand("item", str(in.name)), body)
}

func (in *inNode) parentX() float64 {
	if in.parent == nil {
		return 0
	}
	return in.parent.node.X
}

func (in *inNode) parentY() float64 {
	if in.parent == nil {
		return 0
	}
	return in.parent.node.Y
}

func (cv *importConv) color(n *Node) *tenn.Node {
	if n.Color == "" {
		return nil
	}
	name, ok := presetName(n.Color)
	if !ok {
		cv.rep.add("node %q: colour %q is not 1..6, the fill is left to the default", n.ID, n.Color)
		return nil
	}
	if !sameColor(name, n.Color) {
		cv.rep.add("colour %q has no exact .tenn shade, using %s", n.Color, name)
	}
	return cmd("color", str(name))
}

func sameColor(name, key string) bool {
	for _, p := range presets {
		if p.key == key {
			r1, g1, b1, ok1 := parseRGB(name)
			r2, g2, b2, ok2 := parseRGB(p.hex)
			return ok1 && ok2 && r1 == r2 && g1 == g2 && b1 == b2
		}
	}
	return false
}

func (cv *importConv) edge(e Edge) {
	from, ok := cv.byID[e.FromNode]
	if !ok {
		cv.rep.add("edge %q: fromNode %q is not a node", e.ID, e.FromNode)
		return
	}
	to, ok := cv.byID[e.ToNode]
	if !ok {
		cv.rep.add("edge %q: toNode %q is not a node", e.ID, e.ToNode)
		return
	}
	if from.parent != to.parent {
		cv.rep.add("edge %q: .tenn links join siblings only, %q and %q are in different groups", e.ID, from.name, to.name)
		return
	}
	var body []*tenn.Node
	if e.Label != "" {
		body = append(body, cmd("label", str(e.Label)))
	}
	if e.FromEnd == "arrow" {
		cv.rep.add("edge %q: an arrow at the source has no .tenn form, only the arrow at the target is kept", e.ID)
	}
	switch e.ToEnd {
	case "", "none":
	case "arrow":
		body = append(body, cmd("display", str("arrow")))
	default:
		cv.rep.add("edge %q: end %q is unknown, the line has no arrow", e.ID, e.ToEnd)
	}
	if e.Color != "" {
		if name, ok := presetName(e.Color); ok {
			if !sameColor(name, e.Color) {
				cv.rep.add("colour %q has no exact .tenn shade, using %s", e.Color, name)
			}
			body = append(body, cmd("color", str(name)))
		} else {
			cv.rep.add("edge %q: colour %q is not 1..6, the line keeps the default", e.ID, e.Color)
		}
	}
	switch {
	case e.FromSide != "" && e.ToSide != "":
		body = append(body, cmd("route", str("elbow"), str(e.FromSide), str(e.ToSide)))
	case e.FromSide != "":
		body = append(body, cmd("route", str("elbow"), str(e.FromSide)))
	case e.ToSide != "":
		cv.rep.add("edge %q: only the target side is given, .tenn writes the source side first; the line is straight", e.ID)
	}
	link := withBlock(tenn.NewCommand("link", str(from.name), str(to.name)), body)
	if from.parent == nil {
		cv.rootLinks = append(cv.rootLinks, link)
	} else {
		from.parent.links = append(from.parent.links, link)
	}
}

// external reports whether a file node points outside the workspace.
func external(s string) bool {
	return strings.Contains(s, "://")
}

func linkText(url string) string {
	label := strings.NewReplacer("[", `\[`, "]", `\]`).Replace(url)
	return "[" + label + "](" + url + ")"
}

func cmd(name string, args ...*tenn.Node) *tenn.Node {
	return tenn.NewCommand(name, args...)
}

func str(s string) *tenn.Node { return tenn.NewStrNode(s) }

func num(v float64) *tenn.Node { return tenn.NewFloatNode(round2(v)) }

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func withBlock(cmd *tenn.Node, body []*tenn.Node) *tenn.Node {
	if len(body) > 0 {
		cmd.Add(tenn.NewBlockExpr(body...))
	}
	return cmd
}
