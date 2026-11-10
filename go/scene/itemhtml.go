package scene

import (
	"fmt"
	"html"
	"strings"

	"github.com/haiodo/gowt/gfx"
)

// ItemHTML is the text of the node as HTML, for "Export text as html": the title and the body as the
// canvas lays them out - the words, their weight, their slant, their colour, the code and the links.
// It is built from what the layout drew, so a copy of the text matches the picture on the screen.
func (s *Screen) ItemHTML(n *Node) string {
	if s == nil || s.p == nil || n == nil {
		return ""
	}
	var b strings.Builder
	if ti := s.titleInfo(n); ti != nil {
		b.WriteString("<div>")
		b.WriteString(runsHTML(titleRuns(ti)))
		b.WriteString("</div>")
	}
	if body := s.bodyOf(n); body != nil {
		b.WriteString(blocksHTML(body))
	}
	return b.String()
}

// titleInfo is the laid out title of the node, nil when it has none.
func (s *Screen) titleInfo(n *Node) *textInfo {
	if rn := s.rnodeOf(n); rn != nil {
		return s.p.texts[rn]
	}
	return nil
}

// bodyOf is the laid out body of the node, nil when it has none.
func (s *Screen) bodyOf(n *Node) *bodyText {
	if rn := s.rnodeOfBody(n); rn != nil {
		return s.p.bodies[rn]
	}
	return nil
}

// titleRuns is the title as one run list: the whole title is one line here, and its parts carry the fonts.
func titleRuns(ti *textInfo) []bodyRun {
	if ti.parts == nil {
		return []bodyRun{{text: ti.text, font: ti.font}}
	}
	out := make([]bodyRun, 0, len(ti.parts))
	for _, part := range ti.parts {
		if part.img != nil {
			out = append(out, bodyRun{text: "[image]", font: part.font})
			continue
		}
		out = append(out, bodyRun{text: part.text, font: partFontOf(ti, part), color: part.color})
	}
	return out
}

// partFontOf is the font of a part, the font of the title when the part names none.
func partFontOf(ti *textInfo, part titlePart) gfx.Font {
	if part.font != (gfx.Font{}) {
		return part.font
	}
	return ti.font
}

// blocksHTML is the body of an item as HTML, a block at a time.
func blocksHTML(body *bodyText) string {
	var b strings.Builder
	for _, blk := range body.blocks {
		switch blk.kind {
		case bodyHeading:
			level := blk.level
			if level < 1 || level > 6 {
				level = 1
			}
			fmt.Fprintf(&b, "<h%d>%s</h%d>", level, runsHTML(blk.runs), level)
		case bodyParagraph:
			b.WriteString("<div>" + runsHTML(blk.runs) + "</div>")
		case bodyQuote:
			b.WriteString("<blockquote>" + runsHTML(blk.runs) + "</blockquote>")
		case bodyList:
			b.WriteString("<ul>")
			for _, item := range blk.items {
				b.WriteString("<li>" + runsHTML(item) + "</li>")
			}
			b.WriteString("</ul>")
		case bodyCode:
			b.WriteString("<pre><code>" + html.EscapeString(strings.Join(blk.code, "\n")) + "</code></pre>")
		}
	}
	return b.String()
}

// runsHTML is a run of text as HTML: the style of the run wraps its words, and the links nest inside.
func runsHTML(runs []bodyRun) string {
	var b strings.Builder
	for _, r := range runs {
		if r.text == "" {
			continue
		}
		text := html.EscapeString(r.text)
		style := ""
		if r.color.A > 0 {
			style += fmt.Sprintf("color:#%02x%02x%02x;", to8(r.color.R), to8(r.color.G), to8(r.color.B))
		}
		if r.font.Size > 0 {
			style += fmt.Sprintf("font-size:%gpx;", r.font.Size)
		}
		if style != "" {
			text = `<span style="` + style + `">` + text + "</span>"
		}
		if r.code {
			text = "<code>" + text + "</code>"
		}
		if r.font.Italic {
			text = "<i>" + text + "</i>"
		}
		if r.font.Weight >= 600 {
			text = "<b>" + text + "</b>"
		}
		if r.link != "" {
			text = `<a href="` + html.EscapeString(r.link) + `">` + text + "</a>"
		}
		b.WriteString(text)
	}
	return b.String()
}

// to8 is a colour channel of gfx, 0...1, as the two digits of a hex colour.
func to8(v float64) int {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return int(v*255 + 0.5)
}
