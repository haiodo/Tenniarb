package render

import (
	"math"
	"strings"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/markdown"
	"github.com/haiodo/tenniarb/go/model"
)

// Layer orders drawables inside a container: Background first, Hover last.
type Layer int

const (
	LayerDefault Layer = iota
	LayerBackground
	LayerHover
)

// Drawable is a basic drawable element. Draw records it into rec in y-down coordinates; at is the
// y-up offset of the parent.
type Drawable interface {
	Visible() bool
	Selectable() bool
	Layer() Layer
	// Layout lays out the children; dirty is the area that has to be drawn.
	Layout(bounds, dirty Rect)
	SelectorBounds() Rect
	// Bounds is the bounds of the element, shadow and stroke included.
	Bounds() Rect
	Traverse(op func(Drawable) bool)
	Draw(rec *gfx.Recorder, at Point)
}

// itemBase is Swift's ItemDrawable.
type itemBase struct {
	Item    *model.DiagramItem
	visible bool
	layer   Layer
	bounds  Rect
}

// DrawableItem is the item a box or line draws, nil for plain containers and text.
func (b *itemBase) DrawableItem() *model.DiagramItem { return b.Item }

func (b *itemBase) Visible() bool    { return b.visible }
func (b *itemBase) Selectable() bool { return b.layer == LayerDefault }
func (b *itemBase) Layer() Layer     { return b.layer }

// DrawableContainer is a group of drawables; its bounds are those of the children.
type DrawableContainer struct {
	itemBase
	Children []Drawable
}

func NewDrawableContainer(children ...Drawable) *DrawableContainer {
	c := &DrawableContainer{itemBase: itemBase{visible: true}}
	c.Children = append(c.Children, children...)
	return c
}

func (c *DrawableContainer) Append(d Drawable) { c.Children = append(c.Children, d) }

func (c *DrawableContainer) Insert(d Drawable, at int) {
	c.Children = append(c.Children, nil)
	copy(c.Children[at+1:], c.Children[at:])
	c.Children[at] = d
}

func traverseWith(self Drawable, children []Drawable, op func(Drawable) bool) {
	if op(self) {
		for _, c := range children {
			if !op(c) {
				return
			}
		}
	}
}

func (c *DrawableContainer) Traverse(op func(Drawable) bool) { traverseWith(c, c.Children, op) }

// Ordered returns the visible children in draw order: background, normal, hover.
func (c *DrawableContainer) Ordered() []Drawable {
	var out []Drawable
	for _, pass := range []func(Layer) bool{
		func(l Layer) bool { return l == LayerBackground },
		func(l Layer) bool { return l != LayerBackground && l != LayerHover },
		func(l Layer) bool { return l == LayerHover },
	} {
		for _, ch := range c.Children {
			if pass(ch.Layer()) && ch.Visible() {
				out = append(out, ch)
			}
		}
	}
	return out
}

func (c *DrawableContainer) drawChildren(rec *gfx.Recorder, at Point) {
	for _, ch := range c.Ordered() {
		ch.Draw(rec, at)
	}
}

// drawChildrenShadowed draws the children with a CGContext shadow still set: text gets its shadow, the
// rest (images, nested boxes) has none, as gfx has no shadow for them.
func (c *DrawableContainer) drawChildrenShadowed(rec *gfx.Recorder, at Point, sh *gfx.Shadow) {
	for _, ch := range c.Ordered() {
		if tb, ok := ch.(*TextBox); ok && sh != nil {
			tb.drawShadowed(rec, at, sh)
		} else {
			ch.Draw(rec, at)
		}
	}
}

// itemShadow is the shadow of a style, nil without one.
func itemShadow(s *DrawableStyle) *gfx.Shadow {
	if s.Shadow == nil {
		return nil
	}
	sh := gfxShadow(*s.Shadow, s.ShadowBlur, s.ShadowColor)
	sh.Screen = s.ScreenShadow
	return &sh
}

func (c *DrawableContainer) Draw(rec *gfx.Recorder, at Point) { c.drawChildren(rec, at) }

func (c *DrawableContainer) layoutChildren(selfBounds, dirty Rect) {
	for _, ch := range c.Children {
		ch.Layout(selfBounds, dirty)
	}
}

func (c *DrawableContainer) Layout(_, dirty Rect) {
	c.layoutChildren(c.SelectorBounds(), dirty)
	c.visible = intersects(dirty, c.Bounds())
}

func (c *DrawableContainer) SelectorBounds() Rect { return c.Bounds() }

func (c *DrawableContainer) Bounds() Rect {
	var rect Rect
	for i, ch := range c.Children {
		if i == 0 {
			rect = ch.Bounds()
		} else {
			rect = union(rect, ch.Bounds())
		}
	}
	return rect
}

// gfxRect is a y-up rect as a y-down gfx rect.
func gfxRect(r Rect) gfx.Rect {
	return gfx.R(minX(r), -maxY(r), maxX(r)-minX(r), maxY(r)-minY(r))
}

// defaultShadowColor is the CGContext default shadow colour: black at 1/3 alpha.
var defaultShadowColor = Color{0, 0, 0, 1.0 / 3}

// gfxShadow is CGContext.setShadow(offset:blur:color:): the offset is y-up, gfx's is y-down.
func gfxShadow(offset Size, blur float64, color *Color) gfx.Shadow {
	c := defaultShadowColor
	if color != nil {
		c = *color
	}
	return gfx.Shadow{DX: offset.Width, DY: -offset.Height, Blur: blur, Color: c.gfxColor()}
}

func shadowRect(bounds Rect, style *DrawableStyle) Rect {
	result := bounds
	// In case we had shadow, we need to extend bounds a bit.
	if s := style.Shadow; s != nil {
		if s.Width < 0 {
			result.X += s.Width - style.ShadowBlur*2
			result.Width += -1*s.Width + style.ShadowBlur*4
		}
		if s.Width > 0 {
			result.Width += s.Width + style.ShadowBlur*2
		}
		if s.Height < 0 {
			result.Y += s.Height - style.ShadowBlur*2
			result.Height += -1*s.Height + style.ShadowBlur*4
		}
		if s.Height > 0 {
			result.Height += s.Height + style.ShadowBlur*2
		}
	}
	return result
}

// strokeStyle is updateLineStyle: dash by line-style and the line width.
func strokeStyle(s *DrawableStyle) gfx.StrokeStyle {
	st := gfx.StrokeStyle{Width: s.LineWidth, MiterLimit: 10}
	if s.LineStyle != nil {
		switch *s.LineStyle {
		case "dotted":
			st.Dash, st.DashPhase = []float64{1, 4}, 1
		case "dashed":
			// Canvas repeats an odd dash list, so [5] is 5 on, 5 off.
			st.Dash, st.DashPhase = []float64{5, 5}, 5
		}
	}
	return st
}

// RoundBox is the default item box: a rounded rectangle holding the text.
type RoundBox struct {
	DrawableContainer
	Style *DrawableItemStyle
	Fill  bool
	Stack int
	// StackStep is the offset between the stacked sheets.
	StackStep Point

	path   *path
	radius [4]float64
}

func NewRoundBox(bounds Rect, style *DrawableItemStyle, fill bool) *RoundBox {
	b := &RoundBox{DrawableContainer: DrawableContainer{itemBase: itemBase{visible: true}}, Style: style, Fill: fill, StackStep: Point{2, 2}}
	b.SetPath(bounds)
	return b
}

func (b *RoundBox) SetPath(rect Rect) {
	base := 8.0
	if len(b.Style.CornerRadius) > 0 {
		base = b.Style.CornerRadius[0]
	}
	radius := [4]float64{base, base, base, base}
	copy(radius[:], b.Style.CornerRadius)
	b.radius = radius

	b.bounds = rect
	p := newPath()
	p.move(Point{midX(rect), minY(rect)})
	p.arc(Point{maxX(rect), minY(rect)}, Point{maxX(rect), maxY(rect)}, radius[2])
	p.arc(Point{maxX(rect), maxY(rect)}, Point{minX(rect), maxY(rect)}, radius[1])
	p.arc(Point{minX(rect), maxY(rect)}, Point{minX(rect), minY(rect)}, radius[0])
	p.arc(Point{minX(rect), minY(rect)}, Point{maxX(rect), minY(rect)}, radius[3])
	p.close()
	b.path = p
}

func (b *RoundBox) Path() *path { return b.path }

func (b *RoundBox) Traverse(op func(Drawable) bool) { traverseWith(b, b.Children, op) }

// uniformRadius reports the corner radius when all four are equal, so a gfx shadow box can draw it.
func (b *RoundBox) uniformRadius() (float64, bool) {
	r := b.radius
	return r[0], r[0] == r[1] && r[1] == r[2] && r[2] == r[3]
}

// fillBox fills the box path at offset (dx, dy); with sh set, under that shadow.
func (b *RoundBox) fillBox(rec *gfx.Recorder, dx, dy float64, sh *gfx.Shadow) {
	fill := b.Style.Color.gfxColor()
	if sh == nil {
		rec.FillPath(b.path.toGfx(dx, dy), fill)
		return
	}
	// The shadow box is the cheap path, but gfx rounds it with one radius capped at half the side.
	r, uniform := b.uniformRadius()
	rc := gfxRect(b.bounds).Translate(gfx.Point{X: dx, Y: -dy})
	if uniform && r <= math.Min(rc.W(), rc.H())/2 {
		rec.FillShadowRRect(rc.Min.X, rc.Min.Y, rc.W(), rc.H(), r, fill, *sh)
		return
	}
	rec.FillShadowPath(b.path.toGfx(dx, dy), fill, *sh)
}

func (b *RoundBox) doDraw(rec *gfx.Recorder, at Point, sh *gfx.Shadow) {
	for i := 1; i <= b.Stack; i++ {
		stackSh := gfxShadow(Size{1, 1}, 5, nil)
		stackSh.Screen = b.Style.ScreenShadow
		b.fillBox(rec, at.X+b.StackStep.X*float64(b.Stack-i), at.Y+b.StackStep.Y*float64(b.Stack-i), &stackSh)
	}
	if b.Fill {
		b.fillBox(rec, at.X, at.Y, sh)
	}
	p, st, col := b.path.toGfx(at.X, at.Y), strokeStyle(&b.Style.DrawableStyle), b.Style.BorderColor.gfxColor()
	if sh != nil && !b.Fill {
		rec.StrokeShadowPath(p, st, col, *sh)
	} else {
		rec.StrokePath(p, st, col)
	}
}

func (b *RoundBox) Draw(rec *gfx.Recorder, at Point) {
	rec.Save()
	b.doDraw(rec, at, itemShadow(&b.Style.DrawableStyle))

	// The width is one short, as in Swift.
	rec.ClipRect(gfxRect(Rect{b.bounds.X + at.X, b.bounds.Y + at.Y, b.bounds.Width - 1, b.bounds.Height}))
	b.drawChildren(rec, Point{minX(b.bounds) + at.X, minY(b.bounds) + at.Y})
	rec.Restore()
}

func (b *RoundBox) Layout(_, dirty Rect) {
	b.layoutChildren(b.bounds, dirty)
	b.visible = intersects(dirty, b.Bounds())
}

func (b *RoundBox) SelectorBounds() Rect { return b.bounds }

func (b *RoundBox) Bounds() Rect {
	lw := b.Style.LineWidth
	return insetBy(shadowRect(b.bounds, &b.Style.DrawableStyle), -lw, -lw)
}

// EmptyBox holds text without a visible box (display text). Its shadow falls on the text.
type EmptyBox struct {
	DrawableContainer
	Style *DrawableStyle
}

func NewEmptyBox(bounds Rect, style *DrawableStyle) *EmptyBox {
	b := &EmptyBox{DrawableContainer: DrawableContainer{itemBase: itemBase{visible: true, bounds: bounds}}, Style: style}
	return b
}

func (b *EmptyBox) SetPath(rect Rect) { b.bounds = rect }

func (b *EmptyBox) Traverse(op func(Drawable) bool) { traverseWith(b, b.Children, op) }

func (b *EmptyBox) Draw(rec *gfx.Recorder, at Point) {
	rec.Save()
	rec.ClipRect(gfxRect(Rect{b.bounds.X + at.X, b.bounds.Y + at.Y, b.bounds.Width, b.bounds.Height}))
	b.drawChildrenShadowed(rec, Point{minX(b.bounds) + at.X, minY(b.bounds) + at.Y}, itemShadow(b.Style))
	rec.Restore()
}

func (b *EmptyBox) Layout(_, dirty Rect) {
	b.layoutChildren(b.bounds, dirty)
	b.visible = intersects(dirty, b.Bounds())
}

func (b *EmptyBox) SelectorBounds() Rect { return b.bounds }
func (b *EmptyBox) Bounds() Rect         { return b.bounds }

// InvisibleBox is display none: it takes room and gives bounds but is never drawn.
type InvisibleBox struct{ EmptyBox }

func NewInvisibleBox(bounds Rect, style *DrawableStyle) *InvisibleBox {
	return &InvisibleBox{*NewEmptyBox(bounds, style)}
}

func (b *InvisibleBox) Visible() bool { return false }

func (b *InvisibleBox) Traverse(op func(Drawable) bool) { traverseWith(b, b.Children, op) }

// CircleBox is display circle: an ellipse in the item bounds.
type CircleBox struct {
	DrawableContainer
	Style *DrawableStyle
	Fill  bool
}

func NewCircleBox(bounds Rect, style *DrawableStyle, fill bool) *CircleBox {
	return &CircleBox{DrawableContainer: DrawableContainer{itemBase: itemBase{visible: true, bounds: bounds}}, Style: style, Fill: fill}
}

func (b *CircleBox) SetPath(rect Rect) { b.bounds = rect }

func (b *CircleBox) Traverse(op func(Drawable) bool) { traverseWith(b, b.Children, op) }

func ellipsePath(r Rect) *gfx.Path {
	return gfx.EllipsePath(gfxRect(Rect{midX(r) - math.Abs(r.Width)/2, midY(r) - math.Abs(r.Height)/2, math.Abs(r.Width), math.Abs(r.Height)}))
}

func (b *CircleBox) Draw(rec *gfx.Recorder, at Point) {
	rec.Save()
	r := Rect{b.bounds.X + at.X, b.bounds.Y + at.Y, b.bounds.Width, b.bounds.Height}
	fill := b.Style.Color.gfxColor()
	sh := itemShadow(b.Style)
	if b.Fill {
		switch {
		case sh == nil:
			rec.FillPath(ellipsePath(r), fill)
		case math.Abs(b.bounds.Width-b.bounds.Height) < 1e-6:
			// A square box is an exact circle, which gfx draws from the cheap shadow box.
			g := gfxRect(r)
			rec.FillShadowRRect(g.Min.X, g.Min.Y, g.W(), g.H(), g.W()/2, fill, *sh)
		default:
			rec.FillShadowPath(ellipsePath(r), fill, *sh)
		}
	}
	if sh != nil && !b.Fill {
		rec.StrokeShadowPath(ellipsePath(r), strokeStyle(b.Style), b.Style.BorderColor.gfxColor(), *sh)
	} else {
		rec.StrokePath(ellipsePath(r), strokeStyle(b.Style), b.Style.BorderColor.gfxColor())
	}

	rec.ClipPath(ellipsePath(Rect{r.X + 2, r.Y + 2, r.Width - 4, r.Height - 4}))
	b.drawChildren(rec, Point{minX(b.bounds) + at.X, minY(b.bounds) + at.Y})
	rec.Restore()
}

func (b *CircleBox) Layout(_, dirty Rect) {
	b.layoutChildren(b.bounds, dirty)
	b.visible = intersects(dirty, b.Bounds())
}

func (b *CircleBox) SelectorBounds() Rect { return b.bounds }
func (b *CircleBox) Bounds() Rect         { return shadowRect(b.bounds, b.Style) }

// ImageBox is a bare image at pos.
type ImageBox struct {
	Pos  Point
	Size Size
	Img  gfx.Image
}

func (b *ImageBox) Visible() bool                   { return true }
func (b *ImageBox) Selectable() bool                { return true }
func (b *ImageBox) Layer() Layer                    { return LayerDefault }
func (b *ImageBox) Layout(_, _ Rect)                {}
func (b *ImageBox) Traverse(op func(Drawable) bool) { op(b) }
func (b *ImageBox) SelectorBounds() Rect            { return b.Bounds() }
func (b *ImageBox) Bounds() Rect                    { return Rect{b.Pos.X, b.Pos.Y, b.Size.Width, b.Size.Height} }

func (b *ImageBox) Draw(rec *gfx.Recorder, at Point) {
	g := gfxRect(Rect{b.Pos.X + at.X, b.Pos.Y + at.Y, b.Size.Width, b.Size.Height})
	rec.DrawImage(b.Img, g.Min.X, g.Min.Y, g.W(), g.H())
}

// arrow is the outline of an arrow from -> to, nil when shorter than 5.
func arrow(from, to Point, tailWidth, headWidth, headLength float64) *path {
	length := math.Hypot(to.X-from.X, to.Y-from.Y)
	if length < 5 {
		return nil
	}
	tailLength := length - headLength
	points := []Point{
		{0, tailWidth / 2.0},
		{tailLength, tailWidth / 2.0},
		{tailLength, headWidth / 2.0},
		{length, 0},
		{tailLength, -headWidth / 2.0},
		{tailLength, -tailWidth / 2.0},
		{0, -tailWidth / 2.0},
	}
	cosine := (to.X - from.X) / length
	sine := (to.Y - from.Y) / length
	p := newPath()
	for i, pt := range points {
		q := Point{cosine*pt.X - sine*pt.Y + from.X, sine*pt.X + cosine*pt.Y + from.Y}
		if i == 0 {
			p.move(q)
		} else {
			p.line(q)
		}
	}
	p.close()
	return p
}

// DrawableLine is a link between two boxes, or two exact points.
type DrawableLine struct {
	itemBase
	Source, Target         Point
	ExtraPoints            []Point
	SourceRect, TargetRect Rect
	Style                  *DrawableLineStyle
	Control                Point
	Label                  *TextBox
}

// NewDrawableLine is a line between two boxes, laid out right away.
func NewDrawableLine(source, target Rect, style *DrawableLineStyle, control Point) *DrawableLine {
	l := &DrawableLine{itemBase: itemBase{visible: true}, Style: style, Control: control, SourceRect: source, TargetRect: target}
	l.UpdateLayout(source, target)
	return l
}

// NewDrawableLineBetween is a line between two exact points.
func NewDrawableLineBetween(source, target Point, style *DrawableLineStyle) *DrawableLine {
	return &DrawableLine{itemBase: itemBase{visible: true}, Style: style, Source: source, Target: target}
}

func (l *DrawableLine) Traverse(op func(Drawable) bool) { op(l) }

func (l *DrawableLine) AddLabel(label string, images *ElementImageProvider, env *textEnv) {
	shift := Point{}
	attr := toAttributedString(markdown.GetTokens(strings.ReplaceAll(label, `\n`, "\n")), FontSpec{Size: l.Style.FontSize}, l.Style.BorderColor, &shift, images, []TextPosition{Center, Middle}, 0)
	size := env.calculateSize(attr)
	l.Label = &TextBox{frame: Rect{0, 0, size.Width, size.Height}, attr: attr, env: env}
}

// UpdateLayout recomputes the end points and waypoints for boxes sr and tr.
func (l *DrawableLine) UpdateLayout(sr, tr Rect) {
	l.ExtraPoints = nil

	if l.Style.Layout != nil && strings.HasPrefix(*l.Style.Layout, "middle") {
		x1, x2, w1, w2 := sr.X, tr.X, sr.Width, tr.Width
		y1, y2, h1, h2 := sr.Y, tr.Y, sr.Height, tr.Height

		cx := !(x1+w1 < x2 || x1 > x2+w2)
		cy := !(y1+h1 < y2 || y1 > y2+h2)

		switch {
		case cx && !cy:
			// Variant A
			if y1+h1 < y2 {
				// Below
				l.Source = Point{x1 + w1, y1 + h1/2.0}
				l.Target = Point{x2 + w2, y2 + h2/2.0}
				offset := math.Max(x1+w1, x2+w2) + 20
				l.ExtraPoints = append(l.ExtraPoints, Point{offset, l.Source.Y}, Point{offset, l.Target.Y})
			} else {
				// Under
				l.Source = Point{x1, y1 + h1/2.0}
				l.Target = Point{x2, y2 + h2/2.0}
				offset := math.Min(x1, x2) - 20
				l.ExtraPoints = append(l.ExtraPoints, Point{offset, l.Source.Y}, Point{offset, l.Target.Y})
			}
		case !cx && cy:
			// Variant B
			if x1+w1 < x2 {
				// Left
				l.Source = Point{x1 + w1/2, y1}
				l.Target = Point{x2 + w2/2, y2}
				offset := math.Min(y1, y2) - 20
				l.ExtraPoints = append(l.ExtraPoints, Point{l.Source.X, offset}, Point{l.Target.X, offset})
			} else {
				// Right
				l.Source = Point{x1 + w1/2, y1 + h1}
				l.Target = Point{x2 + w2/2, y2 + h2}
				offset := math.Max(y1+h1, y2+h2) + 20
				l.ExtraPoints = append(l.ExtraPoints, Point{l.Source.X, offset}, Point{l.Target.X, offset})
			}
		case x1+w1 < x2:
			// variant C, Left
			if y1+h1 < y2 {
				// Up
				l.Source = Point{x1 + w1, y1 + h1/2}
				l.Target = Point{x2 + w2/2, y2}
				l.ExtraPoints = append(l.ExtraPoints, Point{l.Target.X, l.Source.Y})
			} else {
				// Down
				l.Source = Point{x1 + w1/2, y1}
				l.Target = Point{x2, y2 + h2/2}
				l.ExtraPoints = append(l.ExtraPoints, Point{l.Source.X, l.Target.Y})
			}
		case y1+h1 < y2:
			// Right and Up
			l.Source = Point{x1 + w1/2, y1 + h1}
			l.Target = Point{x2 + w2, y2 + h2/2}
			l.ExtraPoints = append(l.ExtraPoints, Point{l.Source.X, l.Source.Y})
		default:
			// h2 where h1 would be expected: a quirk of the Swift original.
			l.Source = Point{x1, y1 + h2/2}
			l.Target = Point{x2 + w2/2, y2 + h2}
			l.ExtraPoints = append(l.ExtraPoints, Point{l.Target.X, l.Source.Y})
		}
		return
	}

	p1 := Point{midX(sr), midY(sr)}
	p2 := Point{midX(tr), midY(tr)}
	hasControl := l.Control.X != 0 || l.Control.Y != 0

	fromToLen := math.Sqrt(math.Pow(p1.X-p2.X, 2) + math.Pow(p1.Y-p2.Y, 2))
	if fromToLen == 0 {
		fromToLen = 1
	}
	fTo := Point{(p2.X - p1.X) / fromToLen, (p2.Y - p1.Y) / fromToLen}
	ctrl := Point{p1.X + l.Control.X + fTo.X*fromToLen/2, p1.Y + fTo.Y*fromToLen/2 + l.Control.Y}

	srcTo, tgtFrom := p2, p1
	if hasControl {
		l.ExtraPoints = append(l.ExtraPoints, ctrl)
		srcTo, tgtFrom = ctrl, ctrl
	}
	l.Source, l.Target = p1, p2
	if p, ok := crossBox(p1, srcTo, sr); ok {
		l.Source = p
	}
	if p, ok := crossBox(tgtFrom, p2, tr); ok {
		l.Target = p
	}
}

func (l *DrawableLine) labelPosition(lbl *TextBox) Point {
	lb := lbl.Bounds()
	nx := (l.Source.X + l.Target.X) / 2
	if math.Abs(l.Source.Y-l.Target.Y) < 10 {
		nx -= lb.Width / 2
	}
	ny := (l.Source.Y + l.Target.Y) / 2
	if math.Abs(l.Source.X-l.Target.X) < 10 {
		ny -= lb.Height / 2
	}
	return Point{nx + 5, ny}
}

type builtPath struct {
	path       *path
	labelPoint *Point
	drawArrow  bool
	from, to   Point // where the line really starts and ends once shortened for arrows
}

func (l *DrawableLine) display() string {
	if l.Style.Display == nil {
		return ""
	}
	return *l.Style.Display
}

// buildPath is the line path, shifted by point.
func (l *DrawableLine) buildPath(point Point) builtPath {
	p := newPath()
	var labelPoint *Point
	display := l.display()
	arrowTarget := display == "arrow" || display == "arrows"
	arrowSource := display == "arrow-source" || display == "arrows"

	fromPt := Point{l.Source.X + point.X, l.Source.Y + point.Y}
	fromPtLast := fromPt
	toPt := Point{l.Target.X + point.X, l.Target.Y + point.Y}

	p.move(fromPt)

	quad := l.Style.Layout != nil && *l.Style.Layout == "quad"
	quadCp := fromPt
	for _, ep := range l.ExtraPoints {
		// Move from pt to new location
		fromPtLast = Point{ep.X + point.X, ep.Y + point.Y}

		if arrowSource {
			// We need to move source point a bit less
			px, py := fromPtLast.X-fromPt.X, fromPtLast.Y-fromPt.Y
			plen := math.Sqrt(px*px + py*py)
			if plen > 10 {
				fromPt = Point{fromPt.X + px/plen*5, fromPt.Y + py/plen*5}
				p.move(fromPt)
			}
		}
		if quad {
			quadCp = fromPtLast
		} else {
			p.line(fromPtLast)
		}
		if l.Label != nil {
			labelPoint = &Point{ep.X + 5, ep.Y - l.Label.Bounds().Height}
		}
	}

	if len(l.ExtraPoints) == 0 && l.Label != nil {
		lp := l.labelPosition(l.Label)
		labelPoint = &lp
	}

	switch {
	case arrowTarget:
		// We need to draw a bit less
		px, py := toPt.X-fromPtLast.X, toPt.Y-fromPtLast.Y
		plen := math.Sqrt(px*px + py*py)
		if plen > 10 {
			toPt = Point{toPt.X - px/plen*5, toPt.Y - py/plen*5}
		}
		if quad {
			p.quad(toPt, quadCp)
		} else {
			p.line(toPt)
		}
	case quad:
		p.quad(toPt, quadCp)
	default:
		p.line(toPt)
	}
	return builtPath{p, labelPoint, arrowSource || arrowTarget, fromPt, toPt}
}

func (l *DrawableLine) Draw(rec *gfx.Recorder, at Point) {
	rec.Save()
	col := l.Style.Color.gfxColor()
	st := strokeStyle(&l.Style.DrawableStyle)

	sh := itemShadow(&l.Style.DrawableStyle)
	bp := l.buildPath(at)
	if sh != nil {
		rec.StrokeShadowPath(bp.path.toGfx(0, 0), st, col, *sh)
	} else {
		rec.StrokePath(bp.path.toGfx(0, 0), st, col)
	}

	if bp.drawArrow {
		spt, ept := bp.to, bp.from
		if n := len(l.ExtraPoints); n > 0 {
			spt = Point{l.ExtraPoints[0].X + at.X, l.ExtraPoints[0].Y + at.Y}
			ept = Point{l.ExtraPoints[n-1].X + at.X, l.ExtraPoints[n-1].Y + at.Y}
		}
		toPt, fromPt := bp.to, bp.from
		heads := newPath()
		display := l.display()
		if display == "arrow" || display == "arrows" {
			px, py := ept.X-toPt.X, ept.Y-toPt.Y
			if plen := math.Sqrt(px*px + py*py); plen > 10 {
				ept = Point{toPt.X + px/plen*10, toPt.Y + py/plen*10}
			}
			appendPath(heads, arrow(ept, toPt, 0, 10, 10))
		}
		if display == "arrow-source" || display == "arrows" {
			px, py := spt.X-fromPt.X, spt.Y-fromPt.Y
			if plen := math.Sqrt(px*px + py*py); plen > 10 {
				spt = Point{fromPt.X + px/plen*10, fromPt.Y + py/plen*10}
			}
			appendPath(heads, arrow(spt, fromPt, 0, 10, 10))
		}
		if len(heads.ops) > 0 {
			g := heads.toGfx(0, 0)
			if sh != nil {
				rec.FillShadowPath(g, col, *sh)
			} else {
				rec.FillPath(g, col)
			}
			rec.StrokePath(g, st, col)
		}
	}

	if l.Label != nil && bp.labelPoint != nil {
		l.Label.drawShadowed(rec, Point{at.X + bp.labelPoint.X, at.Y + bp.labelPoint.Y}, sh)
	}
	rec.Restore()
}

func appendPath(dst, src *path) {
	if src == nil {
		return
	}
	dst.ops = append(dst.ops, src.ops...)
}

func (l *DrawableLine) Layout(_, _ Rect) {}

func (l *DrawableLine) Visible() bool { return true }

// extent grows the box of the end points and waypoints.
func (l *DrawableLine) extent() (x0, y0, x1, y1 float64) {
	x0, x1 = math.Min(l.Source.X, l.Target.X), math.Max(l.Source.X, l.Target.X)
	y0, y1 = math.Min(l.Source.Y, l.Target.Y), math.Max(l.Source.Y, l.Target.Y)
	return
}

func (l *DrawableLine) Bounds() Rect {
	x0, y0, x1, y1 := l.extent()
	for _, ep := range l.ExtraPoints {
		x0, x1 = math.Min(ep.X, x0), math.Max(ep.X, x1)
		y0, y1 = math.Min(ep.Y, y0), math.Max(ep.Y, y1)
	}
	bp := l.buildPath(Point{})
	box := bp.path.boundingBox()
	x0, x1 = math.Min(minX(box), x0), math.Max(maxX(box), x1)
	y0, y1 = math.Min(minY(box), y0), math.Max(maxY(box), y1)
	if bp.drawArrow {
		x0, y0, x1, y1 = x0-10, y0-10, x1+10, y1+10
	}
	lw := l.Style.LineWidth
	return insetBy(Rect{x0, y0, math.Abs(x1 - x0), math.Max(math.Abs(y1-y0), 5)}, -lw, -lw)
}

func (l *DrawableLine) SelectorBounds() Rect {
	x0, y0, x1, y1 := l.extent()
	for _, ep := range l.ExtraPoints {
		x0, x1 = math.Min(ep.X, x0), math.Max(ep.X, x1)
		y0, y1 = math.Min(ep.Y, y0), math.Max(ep.Y, y1)
		if l.Label != nil {
			pt := Point{ep.X + 5, ep.Y - l.Label.Bounds().Height}
			lb := l.Label.SelectorBounds()
			x1 = math.Max(x1, pt.X+lb.Width+5)
			y0 = math.Min(y0, pt.Y-5)
		}
	}
	if len(l.ExtraPoints) == 0 && l.Label != nil {
		pt := l.labelPosition(l.Label)
		lb := l.Label.SelectorBounds()
		x1 = math.Max(x1, pt.X+lb.Width+5)
		y0 = math.Min(y0, pt.Y-5)
	}
	lw := l.Style.LineWidth
	return insetBy(Rect{x0, y0, math.Abs(x1 - x0), math.Max(math.Abs(y1-y0), 5)}, -lw, -lw)
}
