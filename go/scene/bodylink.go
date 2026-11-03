package scene

import (
	"slices"
	"strings"
)

// BodyLink is a link of the body of an item: the label as drawn, where it is, and where it leads. Box is
// in the coordinates of the screen, as Placed is, so the app can hit-test a click on the canvas.
type BodyLink struct {
	Box
	// File is the markdown file the link names, a slash path from the workspace root; "" is the text the
	// body shows, whose headings are in this screen.
	File string
	// Anchor is the heading the link names, normalized as a heading anchor; "" for a link to a file only.
	Anchor string
}

// BodyAnchor is a heading of a body: where its line starts and its anchor.
type BodyAnchor struct {
	Box
	Anchor string
}

// BodyLinks lists the links of the bodies of the screen, in tree order.
func (s *Screen) BodyLinks() []BodyLink { return s.links }

// BodyAnchors lists the headings of the bodies of the screen, in tree order.
func (s *Screen) BodyAnchors() []BodyAnchor { return s.anchors }

// bodyPiece is a link label or a heading line of a body and its box in the coordinates of the body.
type bodyPiece struct {
	x, y, w, h float64
	link       string
	anchor     string
}

// pieces walks the lines of the body at width: a box per line of a heading, a box per linked run. Widths
// are measured as draw measures them, so a box matches the glyphs of the run.
func (b *bodyText) pieces(width float64) []bodyPiece {
	var out []bodyPiece
	y := 0.0
	for _, l := range b.lines(width) {
		y += l.gap
		if l.anchor != "" {
			out = append(out, bodyPiece{x: l.x, y: y, w: l.w, h: l.h, anchor: l.anchor})
		}
		x := l.x
		for _, r := range l.runs {
			w := b.widthOf(r.text, r.font)
			if r.link != "" && w > 0 {
				out = append(out, bodyPiece{x: x, y: y, w: w, h: l.h, link: r.link})
			}
			x += w
		}
		y += l.h
	}
	return out
}

// collectBodies records where the links and the headings of the bodies are drawn, for the app.
func (s *Screen) collectBodies() {
	var walk func(rn *RNode, ox, oy float64)
	walk = func(rn *RNode, ox, oy float64) {
		ln := s.p.frames[rn]
		if ln == nil {
			return
		}
		x, y := ox+ln.RX, oy+ln.RY
		if body := s.p.bodies[rn]; body != nil && s.p.icons[rn] == nil {
			bx, by, w := body.padX, body.padY, ln.RW-2*body.padX
			if bn := s.p.bodyAt[rn]; bn != nil {
				bx, by, w = bn.RX, bn.RY, bn.RW
			} else if ti := s.p.texts[rn]; ti != nil {
				by = ti.padY + float64(len(ti.wrap(ln.RW-2*ti.padX)))*ti.lineH
			}
			for _, p := range body.pieces(w) {
				switch {
				case p.link != "":
					file, anchor, ok := s.bodyTarget(body, p.link)
					if !ok {
						continue
					}
					s.links = append(s.links, BodyLink{
						Box:    Box{X: x + bx + p.x, Y: y + by + p.y, W: p.w, H: p.h},
						File:   file,
						Anchor: anchor,
					})
				case p.anchor != "":
					s.anchors = append(s.anchors, BodyAnchor{
						Box:    Box{X: x + bx + p.x, Y: y + by + p.y, W: p.w, H: p.h},
						Anchor: p.anchor,
					})
				}
			}
		}
		for _, k := range rn.Kids {
			if k.Kind == RItem {
				walk(k, x, y)
			}
		}
	}
	walk(s.tree, 0, 0)
}

// bodyTarget reads the target of a link as checkBodyLinks does: the file relative to the markdown the
// body came from and the normalized anchor. A link to the file the body itself shows names this text.
func (s *Screen) bodyTarget(body *bodyText, target string) (file, anchor string, ok bool) {
	f, a, _ := strings.Cut(target, "#")
	a = anchorOf(strings.TrimSpace(a))
	if f == "" {
		return "", a, a != ""
	}
	if strings.Contains(f, "://") {
		return "", "", false
	}
	_, rel, in := s.p.t.resolveIn(body.base, f)
	if !in {
		return "", "", false
	}
	if rel == body.base {
		return "", a, a != ""
	}
	return rel, a, true
}

// AnchorElement is the path of the elements to the first node of the tree whose body shows a heading with
// this anchor: the text written on the node, or the markdown file it reads. nil is the root element; ok is
// false when no body of the tree has the heading. A path of nested elements with a repeated name does not
// carry the #n of PathOf: a heading in such an element is found by its first row.
func (t *Tree) AnchorElement(anchor string) (path []string, ok bool) {
	var found []string
	var walk func(n *Node, path []string) bool
	walk = func(n *Node, path []string) bool {
		if text, _, has := t.bodyMarkdown(n); has {
			if _, anchors := headingsOf(text); anchors[anchor] {
				found = slices.Clone(path)
				return true
			}
		}
		if n.Kind == KElement {
			path = append(slices.Clone(path), n.Name)
		}
		for _, c := range n.Elems {
			if walk(c, path) {
				return true
			}
		}
		for _, c := range n.Content {
			if c.Kind == KItem && walk(c, path) {
				return true
			}
		}
		return false
	}
	if t.main != nil && walk(t.main, nil) {
		return found, true
	}
	return nil, false
}

// bodyMarkdown is the markdown text of the body of n, as bodyInfo reads it, and the file it comes from:
// "" for text written in the .tenn. ok is false for a node without a body, one whose file cannot be
// read, or an empty text.
func (t *Tree) bodyMarkdown(n *Node) (text, from string, ok bool) {
	var pr *Prop
	for i := len(n.Props) - 1; i >= 0; i-- {
		if n.Props[i].Name == "body" {
			pr = n.Props[i]
			break
		}
	}
	if pr == nil {
		return "", "", false
	}
	if file, has := bodyFileArg(pr.Args); has {
		name, _, _ := strings.Cut(file, "#")
		if name == "" {
			return "", "", false
		}
		abs, rel, in := t.resolveIn(n.Pos.File, name)
		if !in {
			return "", "", false
		}
		src, err := t.read(abs, rel)
		if err != nil {
			return "", "", false
		}
		return src, rel, true
	}
	if len(pr.Args) > 0 {
		text, _ = pr.Args[0].GetIdentText()
	} else if pr.Block != nil {
		for _, c := range pr.Block.Children {
			if cmdName(c) != "text" {
				continue
			}
			args, _ := split(c)
			text = argText(args, 0)
		}
	}
	return text, "", strings.TrimSpace(text) != ""
}
