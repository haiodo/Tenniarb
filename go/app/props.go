//go:build darwin || linux || windows

package app

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/tenn"
)

// propsDelay is how long the properties text rests before it is applied (Swift: 300 ms).
const propsDelay = 300 * time.Millisecond

// Swift draws the value of an expression at least this far from the left edge of the text.
const valueMinX = 250

// props is the properties panel: the text of the selected item, or of the element when nothing is
// selected, edited in place.
type props struct {
	text    *g.StyledTextView
	target  editor.Target // what the text on screen belongs to
	changes int           // counts edits; a timer applies only the last one
	applied int           // the edit the last timer applied: the user is typing while changes differs
	loading bool          // the text is being set by code
	// values are the evaluated expressions by 0-based line, drawn at the line ends.
	values map[int]string
	// sceneKey is the scene node the text belongs to (sceneCodeKey), when a scene element is shown.
	sceneKey string
}

// codePane is the text of the properties under the canvas, as Swift shows it.
func (v *Viewer) codePane(ref **g.Panel) g.Option {
	return g.VBox(g.Ref(ref), g.Grow(), g.Padding(0), g.Gap(0),
		g.StyledText(g.NoBackground(), g.NoBorder(), g.Scrollbars(), g.Grow(), g.ID("props"), g.Init(v.initProps)))
}

func (v *Viewer) initProps(t *g.StyledTextView) {
	v.props.text = t
	t.SetTabs(4)
	t.Unwrap().SetMargins(8, 4, 8, 4)
	t.OnChange(v.onPropsChange)
	t.Unwrap().AddPaintListener(&valuePainter{v})
	t.Unwrap().AddMouseListener(swt.MouseListenerMouseUpAdapter(v.onPropsClick))
}

func (v *Viewer) propsFocused() bool { return v.props.text.Unwrap().IsFocusControl() }

// onPropsChange applies the last edit once the text rested for propsDelay. The timer keeps the
// text and target of its edit, so a selection that changes meanwhile does not lose it.
func (v *Viewer) onPropsChange(text string) {
	if v.props.loading || v.session == nil {
		return
	}
	v.props.changes++
	n, target, key, scene := v.props.changes, v.props.target, v.props.sceneKey, v.sceneMode()
	v.app.After(propsDelay, func() {
		if n != v.props.changes {
			return
		}
		v.props.applied = n
		if scene {
			v.applySceneCode(key, text)
		} else {
			v.applyProps(target, text)
		}
	})
}

// applyProps marks the text, evaluates it and, when it parses, hands it to the session.
func (v *Viewer) applyProps(target editor.Target, text string) {
	if v.session == nil {
		return
	}
	if v.props.text.Text() == text {
		v.styleProps(text)
		v.props.values = v.session.PropsValues(target, text)
		v.props.text.Unwrap().Redraw()
	}
	if len(editor.ParseDiagnostics(text)) == 0 {
		v.session.ApplyProps(target, text)
	}
}

// refreshProps shows the current target in the panel. The text is replaced only when it means
// something else than what the panel holds, so typing is not interrupted by its own echo.
func (v *Viewer) refreshProps() {
	if v.insp != nil {
		v.insp.schedule()
	}
	if v.props.text == nil {
		return
	}
	if v.session == nil {
		v.setPropsText("", editor.Target{})
		return
	}
	if v.sceneMode() {
		v.refreshSceneCode()
		return
	}
	v.props.sceneKey = ""
	target := v.session.PropsTarget()
	want := v.session.PropsText(target)
	cur := v.props.text.Text()
	if target == v.props.target && cur != "" {
		if node, p := tenn.Parse(cur); p.Errors.HasErrors() {
			// The user is typing something that does not parse yet.
			if v.propsFocused() {
				return
			}
		} else if tenn.ToStr(node, 0, false) == want {
			v.props.values = v.session.PropsValues(target, cur)
			v.props.text.Unwrap().Redraw()
			return
		}
	}
	v.setPropsText(want, target)
}

func (v *Viewer) setPropsText(text string, target editor.Target) {
	t := v.props.text
	v.props.loading = true
	t.SetText(text)
	v.props.loading = false
	v.props.target = target
	t.SetCaret(0)
	v.styleProps(text)
	v.props.values = nil
	if v.session != nil && !v.sceneMode() {
		v.props.values = v.session.PropsValues(target, text)
	}
	t.Unwrap().Redraw()
}

// restyleProps paints the code and the expression fields again after a light/dark switch.
func (v *Viewer) restyleProps() {
	if v.props.text == nil {
		return
	}
	v.styleProps(v.props.text.Text())
	v.props.text.Unwrap().Redraw()
	v.insp.refresh()
}

// styleProps colors the text as Swift highlight() does: tokens by class, parse errors in red.
func (v *Viewer) styleProps(text string) {
	t := v.props.text
	t.ClearStyles()
	mode := "light"
	if v.app.Dark() {
		mode = "dark"
	}
	red := g.RGB{R: 220, G: 40, B: 40}
	for _, s := range PropSpans(text) {
		style := g.TextStyle{Foreground: &red, Underline: true}
		if !s.Error {
			c, ok := parseHex(editor.Colors[mode][s.Class])
			if !ok {
				continue
			}
			style = g.TextStyle{Foreground: &c}
		}
		t.SetStyle(s.Start, s.Length, style)
	}
	if v.sceneMode() {
		v.styleStubs(text)
	}
}

// Link colour of the kid lines of the scene Code section, for a light and a dark background.
var (
	stubLight = g.RGB{R: 0, G: 102, B: 204}
	stubDark  = g.RGB{R: 94, G: 168, B: 255}
)

// styleStubs draws each kid line of the scene Code section as a link; the underline of the link kind
// is also what gives the hand cursor.
func (v *Viewer) styleStubs(text string) {
	n, _ := v.selectedScene()
	if n == nil {
		return
	}
	stubs, _ := findStubs(n, text)
	c := stubLight
	if v.app.Dark() {
		c = stubDark
	}
	t := v.props.text
	for _, s := range stubs {
		from, to := utf8.RuneCountInString(text[:s.Start]), utf8.RuneCountInString(text[:s.End])
		t.SetStyle(from, to-from, g.TextStyle{Foreground: &c, Underline: true})
		start := len(utf16.Encode([]rune(text[:s.Start])))
		if r := t.Unwrap().GetStyleRangeAtOffset(int32(start)); r != nil {
			r.Start, r.Length = int32(start), int32(len(utf16.Encode([]rune(text[s.Start:s.End]))))
			r.UnderlineStyle = swt.UNDERLINE_LINK
			t.Unwrap().SetStyleRange(r)
		}
	}
}

// onPropsClick selects the kid whose line was clicked in the scene Code section.
func (v *Viewer) onPropsClick(e *swt.MouseEvent) {
	t := v.props.text
	if _, sel := t.Selection(); e.Button != 1 || !v.sceneMode() || sel > 0 {
		return
	}
	unit := int(t.Unwrap().GetOffsetAtPoint(swt.NewPoint(e.X, e.Y)))
	if unit < 0 {
		return
	}
	text := t.Text()
	off := len(text)
	for i, r := range text { // the widget counts UTF-16 units
		if unit <= 0 {
			off = i
			break
		}
		unit -= utf16.RuneLen(r)
	}
	v.app.Async(func() { v.selectStubAt(text, off) })
}

// selectStubAt selects the kid whose stub holds the byte off of text, as a click in the item tree does.
func (v *Viewer) selectStubAt(text string, off int) {
	n, _ := v.selectedScene()
	if n == nil || v.props.text.Text() != text {
		return
	}
	stubs, _ := findStubs(n, text)
	for _, s := range stubs {
		if off < s.Start || off >= s.End {
			continue
		}
		p := NodePath(v.sv.node, s.Kid)
		if p == "" {
			return
		}
		if v.props.changes != v.props.applied { // an edit still waits for its timer: the selection would drop it
			v.props.changes++
			v.props.applied = v.props.changes
			v.applySceneCode(v.props.sceneKey, text)
		}
		v.selectItemPath(p)
		return
	}
}

func parseHex(s string) (g.RGB, bool) {
	if len(s) != 7 || s[0] != '#' {
		return g.RGB{}, false
	}
	n, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return g.RGB{}, false
	}
	return g.RGB{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n)}, true
}

// valuePainter draws the evaluated value of an expression after the end of its line, with a dotted
// leader, over the text the widget has just drawn.
type valuePainter struct{ v *Viewer }

func (p *valuePainter) PaintControl(e *swt.PaintEvent) {
	st := p.v.props.text.Unwrap()
	values := p.v.props.values
	if len(values) == 0 {
		return
	}
	gc := e.Gc
	gc.SetForeground(swt.NewColorRedGreenBlue(128, 128, 128))
	lines := int(st.GetLineCount())
	for line, val := range values {
		if line < 0 || line >= lines {
			continue
		}
		end := st.GetOffsetAtLine(int32(line)) + int32(len(utf16.Encode([]rune(st.GetLine(int32(line))))))
		at := st.GetLocationAtOffset(end)
		x := max(at.X+10, valueMinX)
		h := st.GetLineHeight()
		gc.SetLineStyle(swt.LINE_DOT)
		gc.DrawLine(at.X+5, at.Y+h/2, x-5, at.Y+h/2)
		gc.SetLineStyle(swt.LINE_SOLID)
		gc.DrawTextStringXYIsTransparent(strings.ReplaceAll(val, "\n", `\n`), x, at.Y, true)
	}
}
