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
// gfx writes the CSS into the defs and gives each animated group the id the rules name. sc must be
// committed.
func (s *Screen) WriteSVG(w io.Writer, sc *gfx.Scene, area gfx.Rect, scale float64) error {
	if len(s.Animations) == 0 {
		var buf bytes.Buffer
		if err := gfx.WriteSubtreeSVGWith(&buf, sc, s.Root, area, scale, gfx.SVGOptions{}); err != nil {
			return err
		}
		_, err := w.Write(buf.Bytes())
		return err
	}
	ids := make(map[*gfx.Node]string, len(s.Animations))
	var css strings.Builder
	seen := map[string]bool{}
	var nodes []*gfx.Node
	var walk func(n *gfx.Node)
	walk = func(n *gfx.Node) {
		if _, ok := s.Animations[n]; ok {
			nodes = append(nodes, n)
		}
		for _, k := range n.Children() {
			walk(k)
		}
	}
	walk(s.Root)
	for i, n := range nodes {
		a := s.Animations[n]
		id := fmt.Sprintf("tenniarb-anim-%d", i+1)
		ids[n] = id
		fmt.Fprintf(&css, "#%s{animation:%s;transform-box:fill-box;transform-origin:center}", id, a.shorthand())
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
	sc.Update(func(tx *gfx.Tx) {
		for n, id := range ids {
			tx.SetName(n, id)
		}
	})
	sc.Commit()
	var buf bytes.Buffer
	if err := gfx.WriteSubtreeSVGWith(&buf, sc, s.Root, area, scale, gfx.SVGOptions{CSS: css.String()}); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}
