package render

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/tenn"
)

// Evaluated holds the ExecutionContext values keyed by token (JSValue in Swift).
type Evaluated map[*tenn.Token]any

var floatRe = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)

// jsToString is JSValue.toString() for the values an expression engine returns.
func jsToString(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return jsNumber(x)
	case float32:
		return jsNumber(float64(x))
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			if _, undef := e.(exec.UndefinedType); e != nil && !undef {
				parts[i] = jsToString(e)
			}
		}
		return strings.Join(parts, ",")
	}
	// undefined, Error, Function, objects: what the engine exports.
	return exec.ToString(v)
}

func jsNumber(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == 0:
		return "0"
	}
	if a := math.Abs(f); a >= 1e21 || a < 1e-6 {
		s := strconv.FormatFloat(f, 'e', -1, 64)
		// Go writes e+21 and e-07, JS e+21 and e-7.
		mant, exp, _ := strings.Cut(s, "e")
		sign := exp[:1]
		exp = strings.TrimLeft(exp[1:], "0")
		return mant + "e" + sign + exp
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// jsToDouble is JSValue.toDouble() / Number(v).
func jsToDouble(v any) float64 {
	switch x := v.(type) {
	case nil:
		return 0
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case bool:
		if x {
			return 1
		}
		return 0
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil && !math.IsInf(f, 0) {
			return math.NaN()
		}
		return f
	}
	// arrays, undefined, objects.
	return exec.ToFloat(v)
}

func (e Evaluated) lookup(n *tenn.Node) (any, bool) {
	if n == nil || n.Token == nil || e == nil {
		return nil, false
	}
	v, ok := e[n.Token]
	return v, ok
}

// getString: the evaluated value of child when it has one, else its text.
func getString(child *tenn.Node, ev Evaluated) (string, bool) {
	if child == nil {
		return "", false
	}
	if v, ok := ev.lookup(child); ok {
		return jsToString(v), true
	}
	return child.GetIdentText()
}

// DrawableStyle is the common part of item, line and scene styles. Nil pointers are Swift's nil.
type DrawableStyle struct {
	Color          Color
	TextColorValue *Color
	BorderColor    Color
	FontSize       float64
	Width          *float64
	Height         *float64
	DarkMode       bool

	LineStyle *string

	Shadow      *Size
	ShadowBlur  float64
	ShadowColor *Color
	// ScreenShadow keeps the shadow constant in screen points while the view zooms (Options.ScreenShadows).
	ScreenShadow bool

	LineWidth float64

	// Display: default/not specified - item box, text - just a text box, etc.
	Display *string
	// Layout is a child layout specification.
	Layout *string
	// Layer is default, background (drawn first, not selected by default) or hover (drawn last).
	Layer *string

	// Inherit is a pattern to include all properties during display from named parent element.
	Inherit      *string
	InheritIndex int
}

func newDrawableStyle(darkMode bool) *DrawableStyle {
	s := &DrawableStyle{DarkMode: darkMode}
	s.Reset()
	return s
}

func (s *DrawableStyle) TextColor() Color {
	if s.TextColorValue != nil {
		return *s.TextColorValue
	}
	return textColorBasedOn(s.Color)
}

func (s *DrawableStyle) copyBase() DrawableStyle {
	r := *s
	// Inherit is not copied by Swift/TS copy().
	r.Inherit, r.InheritIndex = nil, 0
	return r
}

// Reset restores the defaults.
func (s *DrawableStyle) Reset() {
	if s.DarkMode {
		s.Color = Color{0.2, 0.2, 0.2, 1}
		s.BorderColor = colorWhite
	} else {
		s.Color = Color{1, 1, 1, 1}
		s.BorderColor = colorBlack
	}
	s.TextColorValue = nil
	s.FontSize = 18
	s.Width, s.Height = nil, nil
	s.Display, s.Layout, s.LineStyle = nil, nil, nil
	s.Shadow, s.ShadowColor = nil, nil
	s.ShadowBlur = 5
	s.LineWidth = 0.3
}

func ptr[T any](v T) *T { return &v }

func (s *DrawableStyle) getFloat(child *tenn.Node, ev Evaluated) (float64, bool) {
	if child == nil {
		return 0, false
	}
	if v, ok := ev.lookup(child); ok {
		return jsToDouble(v), true
	}
	val, ok := child.GetIdentText()
	if ok && floatRe.MatchString(val) {
		f, _ := strconv.ParseFloat(val, 32)
		return f, true
	}
	return 0, false
}

// Swift tells Int/Double/Float apart from everything else (NSNumber vs String...).
func componentValue(v any) float64 {
	switch x := v.(type) {
	case float64, float32, int, int64:
		return jsToDouble(x)
	}
	return 255
}

func (s *DrawableStyle) getColor(child *tenn.Node, ev Evaluated, alpha float64) (Color, bool) {
	if child == nil {
		return Color{}, false
	}
	if v, ok := ev.lookup(child); ok {
		if arr, isArr := v.([]any); isArr {
			var r, g, b float64
			al := alpha
			if len(arr) >= 3 {
				r, g, b = componentValue(arr[0]), componentValue(arr[1]), componentValue(arr[2])
			}
			if len(arr) == 4 {
				al = componentValue(arr[3]) / 255.0
			}
			return Color{r / 255.0, g / 255.0, b / 255.0, al}, true
		}
		return parseColor(strings.ToLower(jsToString(v)), alpha)
	}
	if text, ok := child.GetIdentText(); ok {
		return parseColor(strings.ToLower(text), alpha)
	}
	return Color{}, false
}

func styleName(k model.PersistenceStyleKind) string { return model.PersistenceStyleName(k) }

// parseLine handles the commands every style knows.
func (s *DrawableStyle) parseLine(cmdName string, child *tenn.Node, ev Evaluated) {
	switch cmdName {
	case styleName(model.StyleColor):
		if c, ok := s.getColor(child.GetChild(1), ev, 1); ok {
			s.Color = c
		}
	case styleName(model.StyleTextColor):
		if c, ok := s.getColor(child.GetChild(1), ev, 1); ok {
			s.TextColorValue = &c
		}
	case styleName(model.StyleFontSize):
		if v, ok := s.getFloat(child.GetChild(1), ev); ok {
			s.FontSize = v
			if s.FontSize > 37 {
				s.FontSize = 36
			} else if s.FontSize < 4 {
				s.FontSize = 4
			}
		}
	case styleName(model.StyleDisplay):
		if v, ok := child.GetIdent(1); ok {
			s.Display = &v
		}
	case styleName(model.StyleLayer):
		if v, ok := child.GetIdent(1); ok {
			s.Layer = &v
		}
	case styleName(model.StyleLayout):
		layout := ""
		for i := 1; i < child.Count(); i++ {
			if v, ok := child.GetIdent(i); ok {
				if len(layout) > 0 {
					layout += ", "
				}
				layout += v
			}
		}
		s.Layout = &layout
	case styleName(model.StyleWidth):
		if v, ok := s.getFloat(child.GetChild(1), ev); ok {
			s.Width = ptr(min(v, 10000))
		}
	case styleName(model.StyleHeight):
		if v, ok := s.getFloat(child.GetChild(1), ev); ok {
			s.Height = ptr(min(v, 10000))
		}
	case styleName(model.StyleBorderColor):
		if c, ok := s.getColor(child.GetChild(1), ev, 1); ok {
			s.BorderColor = c
		}
	case styleName(model.StyleLineStyle):
		if v, ok := child.GetIdent(1); ok {
			s.LineStyle = &v
		}
	case styleName(model.StyleLineWidth):
		if v, ok := s.getFloat(child.GetChild(1), ev); ok {
			s.LineWidth = v
		}
	case styleName(model.StyleInherit):
		// Reads the command name itself, as the Swift code does.
		if v, ok := child.GetIdent(0); ok {
			s.Inherit = &v
			if idx, ok := child.GetInt(1); ok {
				s.InheritIndex = idx
			}
		}
	case styleName(model.StyleShadow):
		x, okx := child.GetFloat(1)
		y, oky := child.GetFloat(2)
		if okx && oky {
			s.Shadow = &Size{x, y}
		}
		if blur, ok := child.GetFloat(3); ok {
			s.ShadowBlur = blur
		}
		if c, ok := s.getColor(child.GetChild(4), ev, 1); ok {
			s.ShadowColor = &c
		}
	}
}

// parseCommands feeds every named command of a block (or statements) node to parse.
func parseCommands(node *tenn.Node, parse func(cmdName string, child *tenn.Node)) {
	if node == nil {
		return
	}
	for _, child := range node.Children {
		if cmdName, ok := child.GetIdent(0); child.Kind == tenn.KindCommand && child.Count() > 0 && ok {
			parse(cmdName, child)
		}
	}
}

// ParseStyle applies the commands of node (a block of properties) to a plain style.
func (s *DrawableStyle) ParseStyle(node *tenn.Node, ev Evaluated) {
	parseCommands(node, func(name string, child *tenn.Node) { s.parseLine(name, child, ev) })
}

// DrawableLineStyle is the style of links.
type DrawableLineStyle struct{ DrawableStyle }

func NewDrawableLineStyle(darkMode bool) *DrawableLineStyle {
	s := &DrawableLineStyle{DrawableStyle{DarkMode: darkMode}}
	s.Reset()
	return s
}

func (s *DrawableLineStyle) Reset() {
	s.DrawableStyle.Reset()
	if s.DarkMode {
		s.Color = colorWhite
		s.BorderColor = colorWhite
	} else {
		s.Color = Color{0.2, 0.2, 0.2, 1}
		s.BorderColor = colorBlack
	}
	s.LineWidth = 1
}

func (s *DrawableLineStyle) Copy() *DrawableLineStyle {
	return &DrawableLineStyle{s.copyBase()}
}

func (s *DrawableLineStyle) ParseStyle(node *tenn.Node, ev Evaluated) {
	s.DrawableStyle.ParseStyle(node, ev)
}

// DrawableItemStyle is the style of items.
type DrawableItemStyle struct {
	DrawableStyle
	Title        *string
	Marker       *string
	CornerRadius []float64
	LineSpacing  float64
}

func NewDrawableItemStyle(darkMode bool) *DrawableItemStyle {
	s := &DrawableItemStyle{DrawableStyle: DrawableStyle{DarkMode: darkMode}}
	s.Reset()
	return s
}

func (s *DrawableItemStyle) Copy() *DrawableItemStyle {
	r := &DrawableItemStyle{DrawableStyle: s.copyBase(), Title: s.Title, Marker: s.Marker, LineSpacing: s.LineSpacing}
	r.CornerRadius = append([]float64(nil), s.CornerRadius...)
	return r
}

func (s *DrawableItemStyle) Reset() { s.DrawableStyle.Reset() }

func evaluatedOr(ev Evaluated, child *tenn.Node, def *string) *string {
	if child != nil {
		if v, ok := ev.lookup(child); ok {
			return ptr(jsToString(v))
		}
	}
	return def
}

func (s *DrawableItemStyle) parseLine(cmdName string, child *tenn.Node, ev Evaluated) {
	switch cmdName {
	case styleName(model.StyleTitle):
		s.Title = nil
		if v, ok := child.GetIdent(1); ok {
			s.Title = &v
		}
		s.Title = evaluatedOr(ev, child.GetChild(1), s.Title)
	case styleName(model.StyleMarker):
		s.Marker = nil
		if v, ok := child.GetIdent(1); ok {
			s.Marker = &v
		}
		s.Marker = evaluatedOr(ev, child.GetChild(1), s.Marker)
	case styleName(model.StyleCornerRadius):
		s.CornerRadius = nil
		for ind := 1; ind < child.Count(); ind++ {
			if v, ok := child.GetFloat(ind); ok {
				s.CornerRadius = append(s.CornerRadius, math.Min(v, 15))
			}
		}
	case styleName(model.StyleLineSpacing):
		if v, ok := child.GetFloat(1); ok {
			s.LineSpacing = v
		}
	default:
		s.DrawableStyle.parseLine(cmdName, child, ev)
	}
}

func (s *DrawableItemStyle) ParseStyle(node *tenn.Node, ev Evaluated) {
	parseCommands(node, func(name string, child *tenn.Node) { s.parseLine(name, child, ev) })
}

// SceneStyle is the style of an element: its own settings and the defaults for items and links.
type SceneStyle struct {
	DrawableStyle
	GridSpan         Point
	DefaultItemStyle *DrawableItemStyle
	DefaultLineStyle *DrawableLineStyle
}

func NewSceneStyle(darkMode bool) *SceneStyle {
	s := &SceneStyle{DrawableStyle: *newDrawableStyle(darkMode), GridSpan: Point{5, 5}}
	s.DefaultItemStyle = NewDrawableItemStyle(darkMode)
	s.DefaultLineStyle = NewDrawableLineStyle(darkMode)
	s.DefaultLineStyle.FontSize = 12
	return s
}

func (s *SceneStyle) parseLine(cmdName string, child *tenn.Node, ev Evaluated) {
	switch cmdName {
	case styleName(model.StyleStyles):
		// Default styles for entire diagram
		block := child.GetChild(1)
		if block == nil || block.Kind != tenn.KindBlockExpr || block.Children == nil {
			return
		}
		for _, styleChild := range block.Children {
			name, ok := styleChild.GetIdent(0)
			if styleChild.Kind != tenn.KindCommand || styleChild.Count() == 0 || !ok {
				continue
			}
			props := styleChild.GetChild(1)
			if props == nil || props.Kind != tenn.KindBlockExpr || props.Children == nil {
				continue
			}
			switch name {
			case "item":
				s.DefaultItemStyle.Reset()
				s.DefaultItemStyle.ParseStyle(props, ev)
			case "line":
				s.DefaultLineStyle.Reset()
				s.DefaultLineStyle.ParseStyle(props, ev)
			}
		}
	case styleName(model.StyleGrid):
		x, okx := child.GetFloat(1)
		y, oky := child.GetFloat(2)
		if okx && oky {
			s.GridSpan = Point{x, y}
		}
	default:
		s.DrawableStyle.parseLine(cmdName, child, ev)
	}
}

func (s *SceneStyle) ParseStyle(node *tenn.Node, ev Evaluated) {
	parseCommands(node, func(name string, child *tenn.Node) { s.parseLine(name, child, ev) })
}
