package scene

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/haiodo/gowt/gfx"
)

// WriteSVG writes the screen as an SVG document of area (the zero Rect for the bounds of the screen) at
// scale, with the animations of its items as CSS (spec 13): a @keyframes block for every animation named
// in <style> and the animation shorthand on the group of each item that plays one. Without running the
// animation the document is the still picture, the first frame of every animation.
//
// gfx writes no CSS of its own, so the animation goes in over its output: the items it draws are the
// <g transform="matrix(...)"> groups, in the order the scene draws them. sc must be committed.
//
// shortcut: the groups are matched by that order, not by an id, which gfx does not write; a picture that
// draws a matrix group of its own would shift the match. Take an id or a CSS hook from gowt when one
// appears.
func (s *Screen) WriteSVG(w io.Writer, sc *gfx.Scene, area gfx.Rect, scale float64) error {
	var buf bytes.Buffer
	if err := gfx.WriteSubtreeSVGWith(&buf, sc, s.Root, area, scale, gfx.SVGOptions{}); err != nil {
		return err
	}
	if len(s.Animations) == 0 {
		_, err := w.Write(buf.Bytes())
		return err
	}
	nodes := s.drawOrder(area)
	var css strings.Builder
	styles := make([]string, len(nodes))
	seen := map[string]bool{}
	for i, n := range nodes {
		a := s.Animations[n]
		if a == nil {
			continue
		}
		styles[i] = "animation:" + a.shorthand() + ";transform-box:fill-box;transform-origin:center"
		if seen[a.Name] {
			continue
		}
		seen[a.Name] = true
		fmt.Fprintf(&css, "@keyframes %s{", a.Name)
		for _, f := range a.Frames {
			fmt.Fprintf(&css, "%s{%s}", f.Percent, f.css())
		}
		css.WriteString("}")
	}
	_, err := io.WriteString(w, injectCSS(buf.String(), css.String(), styles))
	return err
}

// drawOrder lists the scene nodes WriteSubtreeSVGWith writes, in the order it writes them: the nodes
// with a picture inside the area, depth first.
func (s *Screen) drawOrder(area gfx.Rect) []*gfx.Node {
	box := area
	if box.Empty() || box == (gfx.Rect{}) {
		box = s.Root.SubtreeBounds()
	}
	var out []*gfx.Node
	var walk func(n *gfx.Node)
	walk = func(n *gfx.Node) {
		if n.Picture() != nil && !n.Bounds().Empty() && n.Bounds().Intersects(box) {
			out = append(out, n)
		}
		for _, k := range n.Children() {
			walk(k)
		}
	}
	walk(s.Root)
	return out
}

// injectCSS puts the keyframes into the defs of the document and the style of an item onto its group:
// the k-th group of the document is the k-th item of drawOrder.
func injectCSS(doc, css string, styles []string) string {
	if css != "" {
		if i := strings.Index(doc, "</defs>"); i >= 0 {
			doc = doc[:i] + "<style>" + css + "</style>" + doc[i:]
		}
	}
	const open = `<g transform="matrix(`
	var b strings.Builder
	b.Grow(len(doc) + len(styles)*48)
	for k := 0; ; k++ {
		i := strings.Index(doc, open)
		if i < 0 {
			b.WriteString(doc)
			return b.String()
		}
		b.WriteString(doc[:i])
		if k < len(styles) && styles[k] != "" {
			b.WriteString(`<g style="` + styles[k] + `"`)
			doc = doc[i+2:] // the group goes on with its transform
			continue
		}
		b.WriteString(open)
		doc = doc[i+len(open):]
	}
}
