package scene

import (
	"strconv"
	"strings"
	"unicode"
)

// anchorOf is the link target of a heading: lowercase, a space becomes "-", every character that is not
// a letter, a digit or "-" is dropped and Cyrillic stays; "" when nothing is left.
func anchorOf(s string) string {
	var out strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			out.WriteRune(unicode.ToLower(r))
			dash = false
		case r == '-' || unicode.IsSpace(r):
			if out.Len() > 0 && !dash {
				out.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(out.String(), "-")
}

// uniqueAnchor numbers a repeated anchor -2, -3 and so on in document order.
func uniqueAnchor(seen map[string]int, base string) string {
	if base == "" {
		return ""
	}
	seen[base]++
	if n := seen[base]; n > 1 {
		return base + "-" + strconv.Itoa(n)
	}
	return base
}

func runsString(runs []bodyRun) string {
	var out strings.Builder
	for _, r := range runs {
		out.WriteString(r.text)
	}
	return out.String()
}

// headingsOf lists the headings of markdown text as a reader sees them, with their anchors.
func headingsOf(text string) (names []string, anchors map[string]bool) {
	anchors = map[string]bool{}
	seen := map[string]int{}
	for _, line := range strings.Split(text, "\n") {
		_, title, ok := headingAt(line)
		if !ok {
			continue
		}
		name := headingKey(title)
		if name == "" {
			continue
		}
		names = append(names, name)
		if a := uniqueAnchor(seen, anchorOf(name)); a != "" {
			anchors[a] = true
		}
	}
	return names, anchors
}

// tocDepth reads the depth of a toc command: how many heading levels it lists, 3 when it gives none.
func (p *prep) tocDepth(pr *Prop) int {
	if len(pr.Args) == 0 {
		return 3
	}
	v, ok := pr.num(0)
	if !ok || v < 1 || v > 6 {
		p.t.fail(pr.Pos, "toc takes a depth from 1 to 6, got %q", argText(pr.Args, 0))
		return 0
	}
	return int(v)
}

// toc puts the table of contents of the headings before the body: levels up to depth, the font of the
// body less 2 points, one level indented by the width of a list marker.
func (b *bodyText) toc(depth int) {
	if depth <= 0 {
		return
	}
	size := b.font.Size - bodyFontDrop
	if size < 1 {
		size = 1
	}
	var items []bodyTOCItem
	for _, blk := range b.blocks {
		if blk.kind != bodyHeading || blk.level > depth {
			continue
		}
		runs := make([]bodyRun, len(blk.runs))
		for i, r := range blk.runs {
			r.font.Size = size
			runs[i] = r
		}
		items = append(items, bodyTOCItem{level: blk.level, runs: runs})
	}
	if len(items) > 0 {
		b.blocks = append([]bodyBlock{{kind: bodyTOC, toc: items}}, b.blocks...)
	}
}

// bodyLinks lists the targets of the explicit links [label](target) of markdown text, code spans and
// fenced blocks skipped.
func bodyLinks(text string) []string {
	var out []string
	inCode := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCode = !inCode
			continue
		}
		if inCode {
			continue
		}
		for i := 0; i < len(line); {
			if line[i] == '`' {
				if j := strings.IndexByte(line[i+1:], '`'); j >= 0 {
					i += j + 2
				} else {
					i++
				}
				continue
			}
			if line[i] != '[' || i > 0 && line[i-1] == '!' { // an image is not a section link
				i++
				continue
			}
			mid := strings.Index(line[i:], "](")
			if mid < 0 {
				i++
				continue
			}
			end := strings.IndexByte(line[i+mid+2:], ')')
			if end < 0 {
				i++
				continue
			}
			target := strings.TrimSpace(line[i+mid+2 : i+mid+2+end])
			if sp := strings.IndexAny(target, " \t"); sp >= 0 { // an optional title after the path
				target = target[:sp]
			}
			out = append(out, target)
			i += mid + 2 + end + 1
		}
	}
	return out
}

// checkBodyLinks reports a link [label](file#anchor) of the body whose file or section is not there.
// base is the markdown file the text came from, or the .tenn when it was written there; the link is
// resolved from it, as a reader of the markdown does. pos is the body command: the reader fixes the
// .tenn, not the markdown file.
func (p *prep) checkBodyLinks(pos Pos, base, text string) {
	var own map[string]bool
	var ownNames []string
	for _, target := range bodyLinks(text) {
		file, anchor, _ := strings.Cut(target, "#")
		anchor = strings.TrimSpace(anchor)
		if anchor == "" || strings.Contains(file, "://") {
			continue
		}
		if file == "" {
			if own == nil {
				// A fragment names a heading of the markdown file, not only of the section the body shows.
				src := text
				if base != pos.File {
					if s, err := p.t.read(p.t.abs(base), base); err == nil {
						src = s
					}
				}
				ownNames, own = headingsOf(src)
			}
			if !own[anchorOf(anchor)] {
				p.noSection(pos, target, anchor, base, ownNames)
			}
			continue
		}
		abs, rel, ok := p.t.resolveIn(base, file)
		if !ok {
			p.t.fail(pos, "link %q leaves the workspace root", target)
			continue
		}
		src, err := p.t.read(abs, rel)
		if err != nil {
			p.t.fail(pos, "link %q: no such file %s", target, rel)
			continue
		}
		if names, anchors := headingsOf(src); !anchors[anchorOf(anchor)] {
			p.noSection(pos, target, anchor, rel, names)
		}
	}
}

func (p *prep) noSection(pos Pos, target, anchor, rel string, names []string) {
	if len(names) > 0 {
		p.t.fail(pos, "link %q: no section %q in %s (headings: %s)", target, anchor, rel, strings.Join(names, ", "))
		return
	}
	p.t.fail(pos, "link %q: no section %q in %s (no headings)", target, anchor, rel)
}
