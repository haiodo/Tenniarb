package scene

import (
	"math"
	"strings"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/markdown"
	"github.com/haiodo/tenniarb/go/tenn"
)

// The body of an item is the text under its title, as in go/render (spec 5d): markdown, 2 points
// smaller than the title, laid out in the item and grown to fit when no height is given.
const (
	bodyFontDrop    = 2
	bodyBlockGap    = 5.0
	bodyMonoFace    = "Menlo"
	bodyListIndent  = 16.0
	bodyQuoteIndent = 12.0
	bodyCodePad     = 8.0
	bodyBarW        = 3.0
	bodyHeadSize1   = 6.0 // above the body, h2 +4, h3 +2
	bodyHeadGap1    = 10.0
	bodyHeadGap2    = 6.0
)

// bodyRun is one span of a line in one look: a markdown token after its glyphs are decided.
type bodyRun struct {
	text string
	font gfx.Font
	code bool // drawn over the code background
}

// body blocks: a heading, a paragraph, a list of items, a quote, a fenced code block.
const (
	bodyHeading   = 'h'
	bodyParagraph = 'p'
	bodyList      = 'l'
	bodyQuote     = 'q'
	bodyCode      = 'c'
)

type bodyBlock struct {
	kind  byte
	level int         // heading level 1..3
	runs  []bodyRun   // heading, paragraph, quote
	items [][]bodyRun // list: one run list per bullet
	code  []string    // code: source lines, wrapped at the width
}

// bodyLine is one laid out line of the body.
type bodyLine struct {
	x, w  float64 // x: indent of the line, w: width of its runs
	asc   float64
	desc  float64
	h     float64 // line box height
	gap   float64 // extra space before the line, between blocks
	quote bool    // a bar on the left
	code  bool    // a background across the width
	runs  []bodyRun
}

// bodyText is the markdown body of an item, ready to be measured and drawn.
type bodyText struct {
	m       gfx.TextMeasurer
	font    gfx.Font // body text, already 2 points below the title
	mono    gfx.Font // code
	color   gfx.Color
	codeBg  gfx.Color
	bar     gfx.Color // the bar of a quote
	lineH   float64   // 0: the font's rounded line box
	gap     float64   // between blocks
	padX    float64
	padY    float64
	blocks  []bodyBlock
	metrics map[gfx.Font]gfx.TextMetrics
	widths  map[bodyWidthKey]float64
}

type bodyWidthKey struct {
	font gfx.Font
	text string
}

// bodyInfo reads the body command of rn and parses its markdown; nil when rn has no body or an
// empty one. The command has the forms of the classic renderer: body "text", body %{ ... } and
// body { text "..."; font-size 14; ... }.
func (p *prep) bodyInfo(rn *RNode, st style, inCtx bool) *bodyText {
	pr := p.prop(rn, "body")
	if pr == nil {
		return nil
	}
	size, lineH, gap := 0.0, 0.0, bodyBlockGap
	family, mono := "", bodyMonoFace
	color := st.text
	text := ""
	textArg := func(i int) string {
		switch {
		case pr.Val != nil:
			if i < len(pr.Val) {
				return pr.Val[i]
			}
		case i < len(pr.Args):
			return p.markdownArg(pr.Pos.File, pr.Args[i])
		}
		return ""
	}
	if pr.Block != nil {
		for _, c := range pr.Block.Children {
			name := cmdName(c)
			args, _ := split(c)
			switch name {
			case "text":
				if len(args) > 0 {
					text = p.markdownArg(pr.Pos.File, args[0])
				}
			case "font-size":
				if v, ok := argNum(args, 0); ok && v > 0 {
					size = v
				} else {
					p.t.warn(pr.Pos, "body font-size: not a number above 0, ignored")
				}
			case "font-family":
				family = argText(args, 0)
			case "mono":
				if s := argText(args, 0); s != "" {
					mono = s
				}
			case "text-color":
				if c, ok := parseColor(argText(args, 0)); ok {
					color = c
				} else {
					p.t.warn(pr.Pos, "body text-color: %q is not a color", argText(args, 0))
				}
			case "line-height":
				if v, ok := argNum(args, 0); ok && v > 0 {
					lineH = v
				} else {
					p.t.warn(pr.Pos, "body line-height: not a number above 0, ignored")
				}
			case "gap":
				if v, ok := argNum(args, 0); ok && v >= 0 {
					gap = v
				} else {
					p.t.warn(pr.Pos, "body gap: not a number from 0, ignored")
				}
			}
		}
	} else {
		text = textArg(0)
	}
	if strings.TrimSpace(text) == "" {
		return nil
	}
	text = strings.ReplaceAll(dedent(text), "\t", "    ")

	if size == 0 {
		size = p.titleSize(rn) - bodyFontDrop
	}
	if size <= 0 {
		size = 1
	}
	b := &bodyText{m: p.opts.Measurer, font: p.fontOf(rn, size), color: color, lineH: lineH, gap: gap,
		metrics: map[gfx.Font]gfx.TextMetrics{}, widths: map[bodyWidthKey]float64{}}
	if family != "" {
		b.font.Family, b.font.Name = family, ""
	}
	b.mono = gfx.Font{Size: size, Family: mono}
	b.codeBg, _ = parseColor("grey-200")
	b.bar, _ = parseColor("grey-400")
	if inCtx {
		b.padX, b.padY = 0, 0
	} else {
		b.padX, b.padY = textPadX, textPadY
	}
	b.parse(text)
	return b
}

// titleSize is font-size of the item, the size the title is drawn with and the body is based on.
func (p *prep) titleSize(rn *RNode) float64 {
	if fs := p.prop(rn, "font-size"); fs != nil {
		if v, ok := fs.num(0); ok && v > 0 {
			return v
		}
	}
	return defaultFontSize
}

// markdownArg is the text of a string or markdown literal argument. "%{ ... %}" is what a reader
// writes: the literal closes at "}", so the percent of the closing marker lands in the text and is
// cut here (the same rule as template in load.go).
func (p *prep) markdownArg(file string, a *tenn.Node) string {
	if a == nil {
		return ""
	}
	text, _ := a.GetIdentText()
	if a.Token != nil && strings.HasSuffix(text, "%") {
		at := a.Token.Pos + len(text)
		if src := p.t.src[file]; at < len(src) && src[at] == '}' {
			text = strings.TrimSuffix(text, "%")
		}
	}
	return text
}

// argNum is the number of argument i of a command read outside a Prop.
func argNum(args []*tenn.Node, i int) (float64, bool) {
	if i >= len(args) {
		return 0, false
	}
	return (&Prop{Args: args}).num(i)
}

// parse splits markdown into blocks and their inline runs.
func (b *bodyText) parse(text string) {
	var blocks []bodyBlock
	var para, quote []string
	var list *bodyBlock
	var code []string
	inCode := false
	flushPara := func() {
		if len(para) > 0 {
			blocks = append(blocks, bodyBlock{kind: bodyParagraph, runs: b.inline(strings.Join(para, " "))})
			para = nil
		}
	}
	flushQuote := func() {
		if len(quote) > 0 {
			blocks = append(blocks, bodyBlock{kind: bodyQuote, runs: b.inline(strings.Join(quote, " "))})
			quote = nil
		}
	}
	flushList := func() {
		if list != nil {
			blocks = append(blocks, *list)
			list = nil
		}
	}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if inCode {
			if strings.HasPrefix(trimmed, "```") {
				blocks = append(blocks, bodyBlock{kind: bodyCode, code: code})
				code, inCode = nil, false
			} else {
				code = append(code, line)
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") {
			flushPara()
			flushQuote()
			flushList()
			inCode = true
			continue
		}
		if trimmed == "" {
			flushPara()
			flushQuote()
			flushList()
			continue
		}
		if rest, ok := quoteLine(trimmed); ok {
			flushPara()
			flushList()
			quote = append(quote, rest)
			continue
		}
		if level, rest, ok := headingLine(trimmed); ok {
			flushPara()
			flushQuote()
			flushList()
			runs := b.inline(rest)
			size := b.font.Size + bodyHeadSize1 - 2*float64(level-1)
			for i := range runs {
				runs[i].font = b.weighted(runs[i].font, size)
			}
			blocks = append(blocks, bodyBlock{kind: bodyHeading, level: level, runs: runs})
			continue
		}
		if item, ok := bulletLine(trimmed); ok {
			flushPara()
			flushQuote()
			if list == nil {
				list = &bodyBlock{kind: bodyList}
			}
			list.items = append(list.items, b.inline(item))
			continue
		}
		flushList()
		flushQuote()
		para = append(para, trimmed)
	}
	flushPara()
	flushQuote()
	flushList()
	if inCode && len(code) > 0 { // an unclosed fence is still code: the scene must not lose it
		blocks = append(blocks, bodyBlock{kind: bodyCode, code: code})
	}
	b.blocks = blocks
}

// headingLine reads "# ", "## " or "### "; more hashes are an ordinary paragraph.
func headingLine(s string) (int, string, bool) {
	n := 0
	for n < len(s) && s[n] == '#' {
		n++
	}
	if n == 0 || n > 3 || n == len(s) || s[n] != ' ' {
		return 0, "", false
	}
	return n, strings.TrimSpace(s[n:]), true
}

// bulletLine reads "- item", "* item" or "+ item".
func bulletLine(s string) (string, bool) {
	if len(s) > 2 && (s[0] == '-' || s[0] == '*' || s[0] == '+') && s[1] == ' ' {
		return strings.TrimSpace(s[2:]), true
	}
	return "", false
}

// quoteLine reads "> text": one level of quote, the bar is drawn by the layout.
func quoteLine(s string) (string, bool) {
	if s == ">" {
		return "", true
	}
	if len(s) > 1 && s[0] == '>' && s[1] == ' ' {
		return strings.TrimSpace(s[2:]), true
	}
	return "", false
}

// inline turns one line of markdown into runs with go/markdown tokens. Before the lexer runs, the
// forms it does not know are mapped to the ones it does: [label](url) becomes label (a link is
// text in a scene), **bold** becomes *bold* and *italic* becomes _italic_. Code spans are copied
// as they are, so their asterisks and brackets stay literal.
func (b *bodyText) inline(s string) []bodyRun {
	var runs []bodyRun
	add := func(text string, font gfx.Font, code bool) {
		if text == "" {
			return
		}
		if n := len(runs); n > 0 && runs[n-1].font == font && runs[n-1].code == code {
			runs[n-1].text += text
			return
		}
		runs = append(runs, bodyRun{text: text, font: font, code: code})
	}
	for _, t := range markdown.GetTokens(normalizeInline(s)) {
		switch t.Type {
		case markdown.TokBold:
			add(t.Literal, b.styled(b.font, 700, false), false)
		case markdown.TokItalic:
			add(t.Literal, b.styled(b.font, 0, true), false)
		case markdown.TokCode:
			add(t.Literal, b.mono, true)
		case markdown.TokImage:
			add("@("+t.Literal+")", b.font, false)
		case markdown.TokColor:
			add("!("+t.Literal+")", b.font, false)
		case markdown.TokFont:
			add("&("+t.Literal+")", b.font, false)
		case markdown.TokUnderline:
			add("<"+t.Literal+">", b.font, false)
		case markdown.TokScratch:
			add("~"+t.Literal+"~", b.font, false)
		case markdown.TokEOF:
		default:
			add(t.Literal, b.font, false)
		}
	}
	return runs
}

// styled is a run font with a weight or italic. A face Name leaves both to the face (gfx and the
// backends do), so the family from Options.FontName takes over; otherwise **bold** would stay thin.
func (b *bodyText) styled(f gfx.Font, weight int, italic bool) gfx.Font {
	if f.Name != "" {
		fam, _, _ := strings.Cut(f.Name, ",")
		f.Family, f.Name = strings.TrimSpace(fam), ""
	}
	if weight != 0 {
		f.Weight = weight
	}
	if italic {
		f.Italic = true
	}
	return f
}

// weighted is a heading run: the body weight of bold and a size of its own.
func (b *bodyText) weighted(f gfx.Font, size float64) gfx.Font {
	f = b.styled(f, 700, false)
	f.Size = size
	return f
}

// normalizeInline maps **bold**, *italic* and [label](url) to the forms the markdown lexer reads.
// A run of asterisks that is not at the start of a word is left alone, as the lexer would.
func normalizeInline(s string) string {
	boundary := func(i int) bool { return i == 0 || s[i-1] == ' ' || s[i-1] == '\t' }
	var out strings.Builder
	for i := 0; i < len(s); {
		switch {
		case s[i] == '`':
			if j := strings.IndexByte(s[i+1:], '`'); j >= 0 {
				out.WriteString(s[i : i+j+2])
				i += j + 2
			} else {
				out.WriteByte(s[i])
				i++
			}
		case strings.HasPrefix(s[i:], "**") && boundary(i):
			if j := strings.Index(s[i+2:], "**"); j > 0 {
				out.WriteString("*" + s[i+2:i+2+j] + "*")
				i += j + 4
			} else {
				out.WriteByte(s[i])
				i++
			}
		case s[i] == '*' && boundary(i):
			if j := strings.IndexByte(s[i+1:], '*'); j > 0 {
				out.WriteString("_" + s[i+1:i+1+j] + "_")
				i += j + 2
			} else {
				out.WriteByte(s[i])
				i++
			}
		case s[i] == '[':
			if mid := strings.Index(s[i:], "]("); mid >= 0 {
				if end := strings.IndexByte(s[i+mid+2:], ')'); end >= 0 {
					out.WriteString(s[i+1 : i+mid])
					i += mid + 2 + end + 1
					continue
				}
			}
			out.WriteByte(s[i])
			i++
		default:
			out.WriteByte(s[i])
			i++
		}
	}
	return out.String()
}

func (b *bodyText) metric(f gfx.Font) gfx.TextMetrics {
	if m, ok := b.metrics[f]; ok {
		return m
	}
	m := b.m.MeasureText("Hg", f)
	b.metrics[f] = m
	return m
}

func (b *bodyText) widthOf(s string, f gfx.Font) float64 {
	k := bodyWidthKey{f, s}
	if w, ok := b.widths[k]; ok {
		return w
	}
	w := b.m.MeasureText(s, f).Width
	b.widths[k] = w
	return w
}

// lines lays the body out at width. A width of 0 or less does not wrap.
func (b *bodyText) lines(width float64) []bodyLine {
	var out []bodyLine
	for _, blk := range b.blocks {
		var lines []bodyLine
		switch blk.kind {
		case bodyHeading, bodyParagraph:
			lines = b.wrap(blk.runs, width, 0, 0)
		case bodyQuote:
			lines = b.wrap(blk.runs, width, bodyQuoteIndent, bodyQuoteIndent)
			for i := range lines {
				lines[i].quote = true
			}
		case bodyList:
			bulletW := b.widthOf("• ", b.font)
			for j, item := range blk.items {
				runs := append([]bodyRun{{text: "• ", font: b.font}}, item...)
				ls := b.wrap(runs, width, bodyListIndent, bodyListIndent+bulletW)
				if j == 0 && len(out) > 0 && len(ls) > 0 {
					ls[0].gap = b.gap
				}
				lines = append(lines, ls...)
			}
			if len(lines) == 0 {
				continue
			}
		case bodyCode:
			inner := width - 2*bodyCodePad
			for _, src := range blk.code {
				for _, part := range b.codeLines(src, inner) {
					line := bodyLine{x: bodyCodePad, runs: []bodyRun{{text: part, font: b.mono, code: true}}, code: true}
					b.finishLine(&line)
					lines = append(lines, line)
				}
			}
			if len(lines) == 0 {
				continue
			}
		}
		if len(out) > 0 && blk.kind != bodyList && len(lines) > 0 {
			lines[0].gap = b.gap
			if blk.kind == bodyHeading {
				switch blk.level {
				case 1:
					lines[0].gap += bodyHeadGap1
				case 2:
					lines[0].gap += bodyHeadGap2
				}
			}
		}
		out = append(out, lines...)
	}
	return out
}

// wrap breaks runs into lines greedily at spaces; text wider than the width stays on its line.
// first is the indent of the first line, cont the indent of the continuation lines.
func (b *bodyText) wrap(runs []bodyRun, width, first, cont float64) []bodyLine {
	var lines []bodyLine
	line := bodyLine{x: first}
	started := false
	flush := func() {
		b.finishLine(&line)
		lines = append(lines, line)
		line = bodyLine{x: cont}
		started = false
	}
	for _, run := range runs {
		for _, tok := range splitSpaces(run.text) {
			space := strings.TrimSpace(tok) == ""
			w := b.widthOf(tok, run.font)
			if space && !started {
				continue // leading spaces of a line are not drawn
			}
			if !space && started && width > 0 && line.x+line.w+w > width+1e-9 {
				flush()
			}
			if n := len(line.runs); n > 0 && line.runs[n-1].font == run.font && line.runs[n-1].code == run.code {
				line.runs[n-1].text += tok
			} else {
				line.runs = append(line.runs, bodyRun{text: tok, font: run.font, code: run.code})
			}
			line.w += w
			started = true
		}
	}
	if started || len(lines) == 0 {
		flush()
	}
	return lines
}

// codeLines breaks a code line at the width by characters: code has no spaces to break at.
// A width of 0 or less keeps the line whole.
func (b *bodyText) codeLines(src string, width float64) []string {
	if width <= 0 || b.widthOf(src, b.mono) <= width+1e-9 {
		return []string{src}
	}
	var out []string
	line := ""
	for _, g := range tenn.Graphemes(src) {
		if line != "" && b.widthOf(line+g, b.mono) > width+1e-9 {
			out = append(out, line)
			line = g
			continue
		}
		line += g
	}
	return append(out, line)
}

// splitSpaces splits text into runs of spaces and runs of other characters, as the renderer wraps.
func splitSpaces(s string) []string {
	var out []string
	for len(s) > 0 {
		sp := s[0] == ' ' || s[0] == '\t'
		end := 1
		for end < len(s) && (s[end] == ' ' || s[end] == '\t') == sp {
			end++
		}
		out = append(out, s[:end])
		s = s[end:]
	}
	return out
}

func (b *bodyText) finishLine(l *bodyLine) {
	for _, run := range l.runs {
		m := b.metric(run.font)
		l.asc = math.Max(l.asc, m.Ascent)
		l.desc = math.Max(l.desc, m.Descent)
	}
	l.h = math.Round(l.asc + l.desc)
	if b.lineH > 0 {
		l.h = b.lineH
	}
}

// natural is the width of the widest line when nothing wraps.
func (b *bodyText) natural() float64 {
	w := 0.0
	for _, l := range b.lines(math.Inf(1)) {
		w = math.Max(w, l.x+l.w)
	}
	return w
}

// height is the height of the body at width, its block gaps included.
func (b *bodyText) height(width float64) float64 {
	h := 0.0
	for _, l := range b.lines(width) {
		h += l.gap + l.h
	}
	return h
}

// draw draws the body with its top left corner at x, y.
func (b *bodyText) draw(r *gfx.Recorder, x, y, width float64) {
	yy := y
	for _, l := range b.lines(width) {
		yy += l.gap
		if l.code && width > 0 {
			r.FillRect(gfx.R(x, yy, width, l.h), b.codeBg)
		}
		if l.quote {
			r.FillRect(gfx.R(x, yy, bodyBarW, l.h), b.bar)
		}
		xx := x + l.x
		base := yy + (l.h-(l.asc+l.desc))/2 + l.asc
		for _, run := range l.runs {
			w := b.widthOf(run.text, run.font)
			if run.code && !l.code && w > 0 {
				r.FillRect(gfx.R(xx, yy, w, l.h), b.codeBg)
			}
			if run.text != "" {
				r.Text(run.text, xx, base, run.font, b.color)
			}
			xx += w
		}
		yy += l.h
	}
}
