package scene

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"strconv"
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

// bodyRun is one span of a line in one look: a markdown token after its glyphs are decided, or an
// inline picture of a classic diagram.
type bodyRun struct {
	text  string
	font  gfx.Font
	color gfx.Color  // the colour of the markup at this point of the line
	code  bool       // drawn over the code background
	link  string     // the target of the link the run is the label of, "" for plain text
	img   *textImage // an inline picture: text is empty
}

// body blocks: a heading, a paragraph, a list of items, a quote, a fenced code block, the table of
// contents of the headings.
const (
	bodyHeading   = 'h'
	bodyParagraph = 'p'
	bodyList      = 'l'
	bodyQuote     = 'q'
	bodyCode      = 'c'
	bodyTOC       = 't'
)

type bodyBlock struct {
	kind   byte
	level  int           // heading level 1..3
	anchor string        // heading: the link target of the heading, see anchorOf
	runs   []bodyRun     // heading, paragraph, quote
	items  [][]bodyRun   // list: one run list per bullet
	code   []string      // code: source lines, wrapped at the width
	toc    []bodyTOCItem // the table of contents, built by toc
}

// bodyTOCItem is one line of the table of contents: a heading level and its text.
type bodyTOCItem struct {
	level int
	runs  []bodyRun
}

// bodyLine is one laid out line of the body.
type bodyLine struct {
	x, w   float64 // x: indent of the line, w: width of its runs
	asc    float64
	desc   float64
	h      float64 // line box height
	gap    float64 // extra space before the line, between blocks
	quote  bool    // a bar on the left
	code   bool    // a background across the width
	anchor string  // heading: the anchor of the heading, on its first line
	runs   []bodyRun
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
	pos     Pos    // the body command, for a warning about its height
	from    string // the markdown file the text was read from; "" for text written in the .tenn
	base    string // the markdown file the text is resolved from: from, or the .tenn that writes it
	p       *prep
	imgs    map[string]*itemImage // the classic inline pictures the body may name
	prov    *markdown.ImageProvider
}

type bodyWidthKey struct {
	font gfx.Font
	text string
}

// bodyInfo reads the body command of rn and parses its markdown; nil when rn has no body or an
// empty one. The command has the forms of the classic renderer: body "text", body %{ ... },
// body { text "..."; font-size 14; ... } and body --from-file "page.md", which reads the text from a
// markdown file so a page layout and the text it shows live apart.
func (p *prep) bodyInfo(rn *RNode, st style, inCtx bool) *bodyText {
	pr := p.prop(rn, "body")
	if pr == nil {
		if toc := p.prop(rn, "toc"); toc != nil {
			p.t.warn(toc.Pos, "toc needs a body: it lists the headings of the body of the same item")
		}
		return nil
	}
	size, lineH, gap := 0.0, 0.0, bodyBlockGap
	family, mono := "", bodyMonoFace
	color := st.text
	text, from := "", ""
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
	}
	if file, ok := bodyFileArg(pr.Args); ok {
		var read bool
		if text, read = p.bodyFileText(pr, file, true); !read {
			return nil
		}
		from = file
	} else if pr.Block == nil {
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
		metrics: map[gfx.Font]gfx.TextMetrics{}, widths: map[bodyWidthKey]float64{}, pos: pr.Pos, from: from, p: p}
	if p.classic {
		b.imgs = p.inlineImages(rn)
		b.prov = inlineProvider(b.imgs)
	}
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
	if toc := p.prop(rn, "toc"); toc != nil {
		b.toc(p.tocDepth(toc))
	}
	base := pr.Pos.File
	if from != "" {
		if f, _, _ := strings.Cut(from, "#"); f != "" {
			if _, rel, ok := p.t.resolveIn(pr.Pos.File, f); ok {
				base = rel
			}
		}
	}
	b.base = base
	p.checkBodyLinks(pr.Pos, base, text)
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

// bodyFileArg is the path after --from-file among the arguments of a body command; ok is false when the
// command has no such flag, and the path is "" when the flag has none.
func bodyFileArg(args []*tenn.Node) (string, bool) {
	for i, a := range args {
		if s, _ := a.GetIdentText(); s == FromFileFlag {
			return argText(args, i+1), true
		}
	}
	return "", false
}

// bodyFileText is the text of "body --from-file <path>": the whole markdown file, or the section named
// after "#". The path is relative to the file that writes the body and may not leave the workspace
// root, as for element --from-file. Every problem is reported when report is set, and ok is false then.
func (p *prep) bodyFileText(pr *Prop, ref string, report bool) (string, bool) {
	file, section, hasSection := strings.Cut(ref, "#")
	if file == "" {
		if report {
			p.t.fail(pr.Pos, "body %s needs a file: body %s \"docs/plan.md\"", FromFileFlag, FromFileFlag)
		}
		return "", false
	}
	abs, rel, ok := p.t.resolveIn(pr.Pos.File, file)
	if !ok {
		if report {
			p.t.fail(pr.Pos, "body %s %q leaves the workspace root", FromFileFlag, file)
		}
		return "", false
	}
	// shortcut: the file is read on every load and build, no cache by modification time; add one if a
	// large page is rebuilt in a loop.
	text, err := p.t.read(abs, rel)
	if err != nil {
		// The OS words a missing file differently on every platform; the message is the interface here.
		if errors.Is(err, fs.ErrNotExist) {
			err = fmt.Errorf("no such file")
		}
		if report {
			p.t.fail(pr.Pos, "body %s %q: %v", FromFileFlag, file, err)
		}
		return "", false
	}
	if !hasSection {
		return text, true
	}
	if s, found := bodySection(text, section); found {
		return s, true
	}
	if report {
		if all := bodyHeadings(text); len(all) > 0 {
			p.t.fail(pr.Pos, "body %s %q: %s has no heading %q; headings: %s", FromFileFlag, ref, rel, section, strings.Join(all, ", "))
		} else {
			p.t.fail(pr.Pos, "body %s %q: %s has no heading %q and no headings at all", FromFileFlag, ref, rel, section)
		}
	}
	return "", false
}

// bodySection cuts the section under the heading name: the heading line and everything below it up to
// the next heading of the same or a higher level, as a reader of the markdown file sees it.
func bodySection(text, name string) (string, bool) {
	want := headingKey(name)
	lines := strings.Split(text, "\n")
	start, level := -1, 0
	for i, line := range lines {
		n, title, ok := headingAt(line)
		if !ok {
			continue
		}
		if start < 0 {
			if headingKey(title) == want {
				start, level = i, n
			}
			continue
		}
		if n <= level {
			return strings.Join(lines[start:i], "\n"), true
		}
	}
	if start < 0 {
		return "", false
	}
	return strings.Join(lines[start:], "\n"), true
}

// headingAt reads an ATX heading: one to six hashes and a space.
func headingAt(line string) (int, string, bool) {
	s := strings.TrimSpace(line)
	n := 0
	for n < len(s) && s[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || n == len(s) || s[n] != ' ' {
		return 0, "", false
	}
	return n, s[n+1:], true
}

// headingKey is a heading name as sections compare it: spaces collapsed, the closing hashes of an ATX
// heading ("## Цель ##") dropped.
func headingKey(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "#")
	return strings.Join(strings.Fields(s), " ")
}

// bodyHeadings lists the headings of a markdown file, for the message about a section that is not there.
func bodyHeadings(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if _, title, ok := headingAt(line); ok {
			out = append(out, headingKey(title))
		}
	}
	return out
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
	anchors := map[string]int{} // heading anchors seen, for the -2, -3 of a repeat
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
			anchor := uniqueAnchor(anchors, anchorOf(runsString(runs)))
			blocks = append(blocks, bodyBlock{kind: bodyHeading, level: level, anchor: anchor, runs: runs})
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

// inline turns one line of markdown into runs with go/markdown tokens, the runs of a link label carrying
// its target. Before the lexer runs, the forms it does not know are mapped to the ones it does: [label](url)
// becomes label, **bold** becomes *bold* and *italic* becomes _italic_. Code spans are copied as they are,
// so their asterisks and brackets stay literal. The colour and size markup carries from one piece to the
// next of a link label.
func (b *bodyText) inline(s string) []bodyRun {
	st := inlineState{font: b.font, color: b.color}
	var runs []bodyRun
	for _, piece := range splitLinks(s) {
		runs = append(runs, b.linkRuns(piece.text, piece.link, &st)...)
	}
	return runs
}

// inlineState is the colour and font in force inside a line: what !(...) and &(...) set until !() and &().
type inlineState struct {
	font  gfx.Font
	color gfx.Color
}

// linkRuns is inline of text that is the label of a link, or plain text when link is "".
func (b *bodyText) linkRuns(s, link string, st *inlineState) []bodyRun {
	var runs []bodyRun
	add := func(text string, font gfx.Font, color gfx.Color, code bool) {
		if text == "" {
			return
		}
		if n := len(runs); n > 0 && runs[n-1].font == font && runs[n-1].color == color && runs[n-1].code == code && runs[n-1].link == link {
			runs[n-1].text += text
			return
		}
		runs = append(runs, bodyRun{text: text, font: font, color: color, code: code, link: link})
	}
	for _, t := range markdown.GetTokens(normalizeInline(s)) {
		switch t.Type {
		case markdown.TokBold:
			add(t.Literal, b.styled(st.font, 700, false), st.color, false)
		case markdown.TokItalic:
			add(t.Literal, b.styled(st.font, 0, true), st.color, false)
		case markdown.TokCode:
			add(t.Literal, b.mono, st.color, true)
		case markdown.TokImage:
			if img := resolveInline(b.prov, b.imgs, t.Literal); img != nil {
				runs = append(runs, bodyRun{img: img})
			} else {
				add("@("+t.Literal+")", st.font, st.color, false)
			}
		case markdown.TokColor:
			before, text, inline := strings.Cut(t.Literal, "|")
			if inline {
				c, ok := b.p.markupColor(b.pos, before)
				if !ok {
					c = b.color
				}
				add(text, st.font, c, false)
			} else if strings.TrimSpace(t.Literal) == "" {
				st.color = b.color
			} else if c, ok := b.p.markupColor(b.pos, t.Literal); ok {
				st.color = c
			} else {
				st.color = b.color
			}
		case markdown.TokFont:
			if size, text, inline := strings.Cut(t.Literal, "|"); inline {
				if v, ok := markupSize(size); ok {
					add(text, sizedFont(st.font, v), st.color, false)
				}
			} else if v, ok := markupSize(t.Literal); ok {
				st.font = sizedFont(b.font, v)
			} else {
				st.font = b.font
			}
		case markdown.TokUnderline:
			add("<"+t.Literal+">", st.font, st.color, false)
		case markdown.TokScratch:
			add("~"+t.Literal+"~", st.font, st.color, false)
		case markdown.TokEOF:
		default:
			add(t.Literal, st.font, st.color, false)
		}
	}
	return runs
}

// markupColor reads the colour of !(...): a name of the cmkdown table or hex digits, and false for a
// name it does not know, reported once per build.
func (p *prep) markupColor(pos Pos, name string) (gfx.Color, bool) {
	c, ok := parseColor(name)
	if !ok && p != nil {
		p.warnColor(pos, name)
	}
	return c, ok
}

// warnColor reports an unknown colour once per build: the same typo on many lines must not bury the rest.
func (p *prep) warnColor(pos Pos, name string) {
	if p.warned == nil {
		p.warned = map[string]bool{}
	}
	if p.warned[name] {
		return
	}
	p.warned[name] = true
	p.t.warn(pos, "!(%s): no such color, the default is used", name)
}

// markupSize reads the size of &(...); anything that is not a number resets the size, as go/render does.
func markupSize(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsInf(v, 0) || math.IsNaN(v) {
		return 0, false
	}
	return v, true
}

// sizedFont is f at another size, its family and weight kept.
func sizedFont(f gfx.Font, size float64) gfx.Font {
	f.Size = size
	return f
}

// inlinePiece is a piece of a line: plain text, or the label of a link and its target.
type inlinePiece struct {
	text string
	link string // the text inside ( ), as bodyLinks reads it; "" for plain text
}

// splitLinks cuts a line at the explicit links [label](target), code spans left whole. An image is not a
// link, as in bodyLinks.
func splitLinks(s string) []inlinePiece {
	var out []inlinePiece
	plain := 0
	flush := func(end int) {
		if end > plain {
			out = append(out, inlinePiece{text: s[plain:end]})
		}
	}
	for i := 0; i < len(s); {
		if s[i] == '`' {
			if j := strings.IndexByte(s[i+1:], '`'); j >= 0 {
				i += j + 2
			} else {
				i++
			}
			continue
		}
		if s[i] != '[' || i > 0 && s[i-1] == '!' {
			i++
			continue
		}
		mid := strings.Index(s[i:], "](")
		if mid < 0 {
			i++
			continue
		}
		end := strings.IndexByte(s[i+mid+2:], ')')
		if end < 0 {
			i++
			continue
		}
		target := strings.TrimSpace(s[i+mid+2 : i+mid+2+end])
		if sp := strings.IndexAny(target, " \t"); sp >= 0 { // an optional title after the path
			target = target[:sp]
		}
		flush(i)
		out = append(out, inlinePiece{text: s[i+1 : i+mid], link: target})
		i += mid + 2 + end + 1
		plain = i
	}
	flush(len(s))
	return out
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
				runs := append([]bodyRun{{text: "• ", font: b.font, color: b.color}}, item...)
				ls := b.wrap(runs, width, bodyListIndent, bodyListIndent+bulletW)
				if j == 0 && len(out) > 0 && len(ls) > 0 {
					ls[0].gap = b.gap
				}
				lines = append(lines, ls...)
			}
			if len(lines) == 0 {
				continue
			}
		case bodyTOC:
			for _, item := range blk.toc {
				x := float64(item.level-1) * bodyListIndent
				lines = append(lines, b.wrap(item.runs, width, x, x)...)
			}
			if len(lines) == 0 {
				continue
			}
		case bodyCode:
			inner := width - 2*bodyCodePad
			for _, src := range blk.code {
				for _, part := range b.codeLines(src, inner) {
					line := bodyLine{x: bodyCodePad, runs: []bodyRun{{text: part, font: b.mono, color: b.color, code: true}}, code: true}
					b.finishLine(&line)
					lines = append(lines, line)
				}
			}
			if len(lines) == 0 {
				continue
			}
		}
		if len(lines) > 0 && blk.kind == bodyHeading {
			lines[0].anchor = blk.anchor
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
		if run.img != nil {
			if started && width > 0 && line.x+line.w+run.img.w > width+1e-9 {
				flush()
			}
			line.runs = append(line.runs, run)
			line.w += run.img.w
			started = true
			continue
		}
		for _, tok := range splitSpaces(run.text) {
			space := strings.TrimSpace(tok) == ""
			w := b.widthOf(tok, run.font)
			if space && !started {
				continue // leading spaces of a line are not drawn
			}
			if !space && started && width > 0 && line.x+line.w+w > width+1e-9 {
				flush()
			}
			if n := len(line.runs); n > 0 && line.runs[n-1].font == run.font && line.runs[n-1].color == run.color && line.runs[n-1].code == run.code && line.runs[n-1].link == run.link && line.runs[n-1].img == nil {
				line.runs[n-1].text += tok
			} else {
				line.runs = append(line.runs, bodyRun{text: tok, font: run.font, color: run.color, code: run.code, link: run.link})
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
	for _, run := range l.runs {
		if run.img != nil {
			l.h = math.Max(l.h, run.img.h)
		}
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

// warnBodyFit warns when a body read from a file needs more room than the item gives it: the page in
// the .tenn has a fixed height, and the file is what grows on the next edit.
func (p *prep) warnBodyFit(rn *RNode) {
	if body := p.bodies[rn]; body != nil && body.from != "" {
		if ln := p.frames[rn]; ln != nil {
			width, top, have := ln.RW-2*body.padX, body.padY, ln.RH
			if bn := p.bodyAt[rn]; bn != nil {
				width, top = bn.RW, 0
			} else if ti := p.texts[rn]; ti != nil {
				top = ti.padY + ti.height(ln.RW-2*ti.padX)
			}
			room := have - top - body.padY
			if need := body.height(width); need > room+0.5 {
				p.t.warn(body.pos, "body %s %q needs %g points of height, the item leaves %g: raise height or shorten the text",
					FromFileFlag, body.from, need, room)
			}
		}
	}
	for _, k := range rn.Kids {
		if k.Kind == RItem {
			p.warnBodyFit(k)
		}
	}
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
			if run.img != nil {
				if img := run.img.raster(b.p); img != nil {
					r.DrawImage(img, xx, yy+(l.h-run.img.h)/2, run.img.w, run.img.h)
				}
				xx += run.img.w
				continue
			}
			w := b.widthOf(run.text, run.font)
			if run.code && !l.code && w > 0 {
				r.FillRect(gfx.R(xx, yy, w, l.h), b.codeBg)
			}
			if run.text != "" {
				r.Text(run.text, xx, base, run.font, run.color)
			}
			xx += w
		}
		yy += l.h
	}
}
