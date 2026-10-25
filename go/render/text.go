package render

import (
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/markdown"
	"github.com/haiodo/tenniarb/go/tenn"
)

// Replaces NSAttributedString + CoreText (CTFramesetter) with runs laid out on top of a gfx text
// measurer. Swift's calculateSize never wraps (path width is greatestFiniteMagnitude); wrapping happens
// only when TextBox.draw runs the same layout at the frame width. Both follow that here.

// Line metrics of SF (NSFont.systemFont, CTLineGetTypographicBounds: 10pt -> 9.668/2.109, linear in
// size), not of the face gfx draws with: boxes get the Swift height whatever the font. Only widths come
// from the measurer.
const (
	fontAscentRatio  = 1980.0 / 2048
	fontDescentRatio = 432.0 / 2048
	fontCapRatio     = 1490.0 / 2048
)

type TextPosition string

const (
	Left   TextPosition = "Left"
	Right  TextPosition = "Right"
	Center TextPosition = "Center"
	Top    TextPosition = "Top"
	Fill   TextPosition = "Fill"
	Bottom TextPosition = "Bottom"
	Middle TextPosition = "Middle"
)

// FontSpec is NSFont: only what the renderer varies.
type FontSpec struct {
	Size         float64
	Bold, Italic bool
}

func systemFont(size float64) FontSpec { return FontSpec{Size: size} }

func (f FontSpec) ascent() float64    { return f.Size * fontAscentRatio }
func (f FontSpec) descent() float64   { return f.Size * fontDescentRatio }
func (f FontSpec) capHeight() float64 { return f.Size * fontCapRatio }

// ParagraphStyle is NSParagraphStyle.
type ParagraphStyle struct {
	Alignment              string // natural, left, center, right
	LineSpacing            float64
	HeadIndent             float64
	ParagraphSpacing       float64
	ParagraphSpacingBefore float64
}

func defaultParagraphStyle() ParagraphStyle { return ParagraphStyle{Alignment: "natural"} }

// Attachment is NSTextAttachment: an image plus its bounds relative to the baseline (y up, as Swift).
type Attachment struct {
	Image   gfx.Image
	Width   float64
	Height  float64
	OffsetY float64
}

// TextRun is one uniformly attributed span of an NSAttributedString.
type TextRun struct {
	Text          string
	Font          FontSpec
	Paragraph     ParagraphStyle
	Color         *Color
	Background    *Color
	Underline     bool
	Strikethrough bool
	Attachment    *Attachment
}

type AttributedString struct{ Runs []*TextRun }

// PlainText is text in one font, for callers without markdown (utils.textSize, tests).
func PlainText(text string, font FontSpec) *AttributedString {
	a := &AttributedString{}
	a.Append(text, font, defaultParagraphStyle(), nil)
	return a
}

func (a *AttributedString) Append(text string, font FontSpec, ps ParagraphStyle, color *Color) *TextRun {
	r := &TextRun{Text: text, Font: font, Paragraph: ps, Color: color}
	a.Runs = append(a.Runs, r)
	return r
}

func (a *AttributedString) AppendString(o *AttributedString) { a.Runs = append(a.Runs, o.Runs...) }

func (a *AttributedString) String() string {
	var b strings.Builder
	for _, r := range a.Runs {
		b.WriteString(r.Text)
	}
	return b.String()
}

// textEnv measures text for layout.
type textEnv struct {
	measurer gfx.TextMeasurer
	family   string
	widths   map[widthKey]float64
}

type widthKey struct {
	font FontSpec
	text string
}

func newTextEnv(m gfx.TextMeasurer, family string) *textEnv {
	if m == nil {
		m = gfx.DefaultMeasurer
	}
	return &textEnv{measurer: m, family: family, widths: map[widthKey]float64{}}
}

// gfxFont is the gfx face for a spec; Spec size must be finite and positive, else 18 (as the TS fontCss).
func (e *textEnv) gfxFont(f FontSpec) gfx.Font {
	size := f.Size
	if math.IsNaN(size) || math.IsInf(size, 0) || size <= 0 {
		size = 18
	}
	gf := gfx.Font{Family: e.family, Size: size, Italic: f.Italic}
	if f.Bold {
		gf.Weight = 700
	}
	return gf
}

func (e *textEnv) width(f FontSpec, text string) float64 {
	k := widthKey{f, text}
	if w, ok := e.widths[k]; ok {
		return w
	}
	w := e.measurer.MeasureText(text, e.gfxFont(f)).Width
	e.widths[k] = w
	return w
}

type segment struct {
	run   *TextRun
	text  string
	width float64
}

// textLine is one laid-out line.
type textLine struct {
	segments []*segment
	width    float64
	// textAscent/textDescent are of the fonts only, what CTLineGetTypographicBounds reports.
	textAscent, textDescent float64
	// endAscent/endDescent are those with the font of the line's terminating "\n" (last line of a paragraph).
	endAscent, endDescent float64
	// maxImageHeight is the tallest inline image on the line.
	maxImageHeight float64
	paragraph      ParagraphStyle
	// index inside its paragraph: continuation lines use headIndent.
	index           int
	lastOfParagraph bool
}

type token struct {
	run   *TextRun
	text  string
	width float64
	space bool
}

func (l *textLine) add(tok token) {
	if n := len(l.segments); n > 0 && l.segments[n-1].run == tok.run && tok.run.Attachment == nil {
		l.segments[n-1].text += tok.text
		l.segments[n-1].width += tok.width
	} else {
		l.segments = append(l.segments, &segment{tok.run, tok.text, tok.width})
	}
	l.width += tok.width
	l.textAscent = math.Max(l.textAscent, tok.run.Font.ascent())
	l.textDescent = math.Max(l.textDescent, tok.run.Font.descent())
	if tok.run.Attachment != nil {
		l.maxImageHeight = math.Max(l.maxImageHeight, tok.run.Attachment.Height)
	}
}

// jsTrim trims what JS String.trim does.
func jsTrim(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return unicode.IsSpace(r) || r == 0xfeff })
}

func (e *textEnv) tokenize(run *TextRun, text string) []token {
	if run.Attachment != nil {
		return []token{{run, text, run.Attachment.Width, false}}
	}
	var result []token
	for len(text) > 0 {
		sp := text[0] == ' ' || text[0] == '\t'
		end := 1
		for end < len(text) && (text[end] == ' ' || text[end] == '\t') == sp {
			end++
		}
		part := text[:end]
		text = text[end:]
		result = append(result, token{run, part, e.width(run.Font, part), sp})
	}
	return result
}

// splitWord breaks a word wider than the line between characters, as NSLayoutManager does.
func (e *textEnv) splitWord(tok token, avail float64) []token {
	var parts []token
	cur := ""
	for _, ch := range tenn.Graphemes(tok.text) {
		if cur != "" && e.width(tok.run.Font, cur+ch) > avail {
			parts = append(parts, token{tok.run, cur, e.width(tok.run.Font, cur), false})
			cur = ""
		}
		cur += ch
	}
	return append(parts, token{tok.run, cur, e.width(tok.run.Font, cur), false})
}

type paragraph struct {
	tokens   []token
	style    ParagraphStyle
	emptyRun *TextRun
	endRun   *TextRun
}

// layoutLines lays attr out in lines. With maxWidth nil only "\n" breaks lines. A trailing "\n" does not
// start another line (CoreText).
func (e *textEnv) layoutLines(attr *AttributedString, maxWidth *float64) []*textLine {
	// Split into paragraphs of tokens; an empty paragraph keeps the run holding its "\n" for the font.
	var paragraphs []paragraph
	var tokens []token
	var style *ParagraphStyle
	var emptyRun *TextRun
	for _, run := range attr.Runs {
		parts := []string{run.Text}
		if run.Attachment == nil {
			parts = strings.Split(run.Text, "\n")
		}
		for i, part := range parts {
			if i > 0 {
				ps := run.Paragraph
				if style != nil {
					ps = *style
				}
				er := run
				if emptyRun != nil {
					er = emptyRun
				}
				paragraphs = append(paragraphs, paragraph{tokens, ps, er, run})
				tokens, style, emptyRun = nil, nil, nil
			}
			if len(part) > 0 {
				if style == nil {
					ps := run.Paragraph
					style = &ps
				}
				if emptyRun == nil {
					emptyRun = run
				}
				tokens = append(tokens, e.tokenize(run, part)...)
			}
		}
	}
	if len(tokens) > 0 {
		paragraphs = append(paragraphs, paragraph{tokens, *style, emptyRun, nil})
	}

	var lines []*textLine
	for _, p := range paragraphs {
		line := &textLine{paragraph: p.style}
		if len(p.tokens) == 0 {
			line.add(token{run: p.emptyRun})
		}
		avail := func(l *textLine) float64 {
			a := math.Inf(1)
			if maxWidth != nil {
				a = *maxWidth
			}
			if l.index > 0 {
				a -= p.style.HeadIndent
			}
			return a
		}
		for _, tok := range p.tokens {
			hasWord := false
			for _, s := range line.segments {
				if s.run.Attachment != nil || jsTrim(s.text) != "" {
					hasWord = true
					break
				}
			}
			if !tok.space && hasWord && line.width+tok.width > avail(line) {
				// Spaces at the wrap point hang past the edge and do not count.
				for n := len(line.segments); n > 0; n = len(line.segments) {
					last := line.segments[n-1]
					if jsTrim(last.text) != "" || last.run.Attachment != nil {
						break
					}
					line.width -= last.width
					line.segments = line.segments[:n-1]
				}
				lines = append(lines, line)
				line = &textLine{paragraph: p.style, index: line.index + 1}
			}
			if !tok.space && tok.run.Attachment == nil && line.width == 0 && tok.width > avail(line) && avail(line) > 0 {
				pieces := e.splitWord(tok, avail(line))
				for _, piece := range pieces[:len(pieces)-1] {
					line.add(piece)
					lines = append(lines, line)
					line = &textLine{paragraph: p.style, index: line.index + 1}
				}
				tok = pieces[len(pieces)-1]
			}
			if tok.space && line.width == 0 && line.index > 0 {
				continue // leading spaces of a continuation line are dropped
			}
			line.add(tok)
		}
		// CTLine includes the "\n" glyph's font, NSAttributedString.draw does not: only calculateSize sees it.
		line.endAscent, line.endDescent = line.textAscent, line.textDescent
		if p.endRun != nil {
			line.endAscent = math.Max(line.endAscent, p.endRun.Font.ascent())
			line.endDescent = math.Max(line.endDescent, p.endRun.Font.descent())
		}
		line.lastOfParagraph = true
		lines = append(lines, line)
	}
	return lines
}

func (l *textLine) ascent() float64  { return math.Max(l.textAscent, l.endAscent) }
func (l *textLine) descent() float64 { return math.Max(l.textDescent, l.endDescent) }

// calculateSize is DrawableScene.calculateSize(attrStr:).
func (e *textEnv) calculateSize(attr *AttributedString) Size {
	lines := e.layoutLines(attr, nil)
	var size Size
	maxWidth := 0.0
	frameHeight := 0.0
	for _, l := range lines {
		maxHeight := math.Floor(l.ascent() + l.descent())
		if l.maxImageHeight > 0 && l.maxImageHeight > maxHeight {
			maxHeight = l.maxImageHeight
		}
		maxPsAdd := 0.0
		for _, s := range l.segments {
			ps := s.run.Paragraph
			maxPsAdd = math.Max(maxPsAdd, ps.ParagraphSpacingBefore+ps.ParagraphSpacing+ps.LineSpacing)
		}
		// Swift also weighs font.boundingRectForFont.height + descender, which never exceeds the line height for these fonts.
		maxWidth = math.Max(maxWidth, l.width)
		size.Height += maxHeight + maxPsAdd + 1

		frameHeight += l.ascent() + l.descent() + l.paragraph.LineSpacing
		if l.lastOfParagraph {
			frameHeight += l.paragraph.ParagraphSpacing
		}
	}
	size.Width = maxWidth
	frameHeight = math.Ceil(frameHeight)
	if size.Height < frameHeight {
		size.Height = frameHeight
	}
	return size
}

// drawAttributedString is NSAttributedString.draw(in:) for a rect given y-up: it lays out at the rect
// width and hangs the first line from the top. rec takes y-down coordinates, so the rect's top edge is
// -(y+height). With sh set the glyphs are drawn over its shadow, as CGContext does with a shadow set.
func (e *textEnv) drawAttributedString(rec *gfx.Recorder, attr *AttributedString, rect Rect, sh *gfx.Shadow) {
	width := rect.Width
	lines := e.layoutLines(attr, &width)
	x0 := rect.X
	y := -(rect.Y + rect.Height)

	for _, l := range lines {
		ps := l.paragraph
		if l.index == 0 {
			y += ps.ParagraphSpacingBefore
		}
		ascent, descent := l.textAscent, l.textDescent
		for _, s := range l.segments {
			if a := s.run.Attachment; a != nil {
				ascent = math.Max(ascent, a.OffsetY+a.Height)
				descent = math.Max(descent, -a.OffsetY)
			}
		}
		baseline := y + ascent

		indent := 0.0
		if l.index > 0 {
			indent = ps.HeadIndent
		}
		avail := rect.Width - indent
		x := x0 + indent
		switch ps.Alignment {
		case "center":
			x += (avail - l.width) / 2
		case "right":
			x += avail - l.width
		}

		for _, s := range l.segments {
			run := s.run
			switch {
			case run.Attachment != nil:
				a := run.Attachment
				rec.DrawImage(a.Image, x, baseline-a.OffsetY-a.Height, a.Width, a.Height)
			case len(s.text) > 0:
				if run.Background != nil {
					rec.FillRect(gfx.R(x, y, s.width, ascent+descent), run.Background.gfxColor())
				}
				c := colorBlack
				if run.Color != nil {
					c = *run.Color
				}
				col := c.gfxColor()
				if sh != nil {
					rec.TextShadow(s.text, x, baseline, e.gfxFont(run.Font), col, *sh)
				} else {
					rec.Text(s.text, x, baseline, e.gfxFont(run.Font), col)
				}
				thickness := math.Max(1, run.Font.Size/14)
				if run.Underline {
					rec.FillRect(gfx.R(x, baseline+thickness, s.width, thickness), col)
				}
				if run.Strikethrough {
					rec.FillRect(gfx.R(x, baseline-run.Font.capHeight()/2, s.width, thickness), col)
				}
			}
			x += s.width
		}
		y += ascent + descent + ps.LineSpacing
		if l.lastOfParagraph {
			y += ps.ParagraphSpacing
		}
	}
}

// MARK: Markdown to attributed string (MarkDownAttributedPrinter + DrawableScene.toAttributedString)

func isBlank(s string) bool { return jsTrim(s) == "" }

func colorPtr(c Color) *Color { return &c }

func toAttributedStr(tokens []markdown.Token, originalFont FontSpec, ps ParagraphStyle, foreground Color, shift *Point, images *ElementImageProvider) *AttributedString {
	result := &AttributedString{}

	currentColor := foreground
	font := originalFont

	prevMultiCode := false
	lastLiteral := ""
	lastToken := markdown.TokEOF
	codeBackground := colorPtr(fromMarkdown(markdown.ParseColor("grey-200", 1)))
	black := colorPtr(colorBlack)
	add := func(text string, f FontSpec, p ParagraphStyle, c *Color) *TextRun {
		return result.Append(text, f, p, c)
	}

	for _, t := range tokens {
		literal := t.Literal
		if prevMultiCode {
			prevMultiCode = false
			literal = strings.TrimPrefix(literal, "\n")
			add("\n", font, ps, black).Background = codeBackground
		}
		if t.Type != markdown.TokEOF {
			lastToken = t.Type
		}
		cur := colorPtr(currentColor)
		switch t.Type {
		case markdown.TokText:
			add(literal, font, ps, cur)
		case markdown.TokBold:
			add(literal, FontSpec{Size: font.Size, Bold: true}, ps, cur)
		case markdown.TokBullet:
			p := ps
			p.HeadIndent = float64(5 * tenn.GraphemeCount(literal))
			if p.HeadIndent > shift.X {
				shift.X = p.HeadIndent
			}
			add("•", font, p, cur)
		case markdown.TokImage:
			if resolved, ok := images.ResolveImagePath(t.Literal); ok {
				if decoded, ok := images.decoded[resolved.Image]; ok {
					r := resolved.Rect
					// We need to add at least one space to be able to see image.
					add("￼", font, ps, nil).Attachment = &Attachment{Image: decoded.Image, Width: r.Width, Height: r.Height, OffsetY: font.capHeight()/2 - r.Height/2}
					add(" ", font, ps, black)
				}
			}
		case markdown.TokItalic:
			f := font
			f.Italic = true
			add(literal, f, ps, cur)
		case markdown.TokUnderline:
			add(literal, font, ps, cur).Underline = true
		case markdown.TokScratch:
			add(literal, font, ps, cur).Strikethrough = true
		case markdown.TokTitle:
			title, titleSize, _ := markdown.CalcTitleFontSize(literal, font.Size)
			p := ps
			p.ParagraphSpacing = 5
			add(title, systemFont(titleSize), p, cur)
		case markdown.TokColor:
			if pos := strings.Index(literal, "|"); pos >= 0 {
				add(literal[pos+1:], font, ps, colorPtr(fromMarkdown(markdown.ParseColor(literal[:pos], 1))))
			} else if isBlank(literal) {
				currentColor = foreground
			} else {
				currentColor = fromMarkdown(markdown.ParseColor(literal, 1))
			}
		case markdown.TokFont:
			if pos := strings.Index(literal, "|"); pos >= 0 {
				fs := literal[:pos]
				if floatRe.MatchString(fs) {
					add(literal[pos+1:], systemFont(parseFloat(fs)), ps, cur)
				}
			} else if isBlank(literal) || !floatRe.MatchString(literal) {
				font = originalFont
			} else {
				font = systemFont(parseFloat(literal))
			}
		case markdown.TokCode:
			if strings.Contains(literal, "\n") {
				if !strings.HasPrefix(literal, "\n") && !strings.HasSuffix(lastLiteral, "\n") {
					add("\n", font, ps, black)
				}
				if !strings.HasSuffix(literal, "\n") {
					prevMultiCode = true
				}
			}
			add(literal, font, ps, black).Background = codeBackground
		}
		lastLiteral = literal
	}

	if lastToken == markdown.TokImage {
		add("\n", font, ps, black)
	}
	return result
}

// parseFloat is Number(s) for a string floatRe has accepted.
func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// toAttributedString is DrawableScene.toAttributedString.
func toAttributedString(tokens []markdown.Token, font FontSpec, color Color, shift *Point, images *ElementImageProvider, layout []TextPosition, lineSpacing float64) *AttributedString {
	ps := defaultParagraphStyle()
	ps.LineSpacing = lineSpacing
	for _, pos := range layout {
		switch pos {
		case Center:
			ps.Alignment = "center"
		case Left:
			ps.Alignment = "left"
		case Right:
			ps.Alignment = "right"
		}
	}
	return toAttributedStr(tokens, font, ps, color, shift, images)
}

// MARK: Body text

func spaceCount(s string) int {
	n := 0
	for _, c := range s {
		if c != ' ' {
			break
		}
		n++
	}
	return n
}

func prepareBodyText(textValue string) string {
	content := strings.ReplaceAll(strings.ReplaceAll(textValue, `\n`, "\n"), "\t", "    ")
	lines := strings.Split(content, "\n")

	// Remove first empty line
	if len(lines) > 0 && isBlank(lines[0]) {
		lines = lines[1:]
	}
	// remove last empty line.
	if len(lines) > 0 && isBlank(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}

	minCount := math.MaxInt
	for _, l := range lines {
		if !isBlank(l) {
			minCount = min(minCount, spaceCount(l))
		}
	}

	out := make([]string, len(lines))
	for i, body := range lines {
		if isBlank(body) {
			out[i] = body + " "
			continue
		}
		r := []rune(body)
		out[i] = string(r[min(minCount, len(r)):])
	}
	return strings.Join(out, "\n")
}

// TextBox draws attributed text into a frame (y-up, relative to its parent).
type TextBox struct {
	frame Rect
	attr  *AttributedString
	env   *textEnv
	// HTML is the markdown source of the text as HTML, built lazily (only "Export text as html" needs it).
	HTML func() string
}

func (t *TextBox) Text() *AttributedString { return t.attr }

func (t *TextBox) SetFrame(r Rect) { t.frame = r }

func (t *TextBox) Visible() bool        { return true }
func (t *TextBox) Selectable() bool     { return true }
func (t *TextBox) Layer() Layer         { return LayerDefault }
func (t *TextBox) Layout(_, _ Rect)     {}
func (t *TextBox) Bounds() Rect         { return t.frame }
func (t *TextBox) SelectorBounds() Rect { return t.frame }

func (t *TextBox) Traverse(op func(Drawable) bool) { op(t) }

func (t *TextBox) Draw(rec *gfx.Recorder, at Point) { t.drawShadowed(rec, at, nil) }

func (t *TextBox) drawShadowed(rec *gfx.Recorder, at Point, sh *gfx.Shadow) {
	t.env.drawAttributedString(rec, t.attr, Rect{at.X + t.frame.X, at.Y + t.frame.Y, t.frame.Width, t.frame.Height}, sh)
}
