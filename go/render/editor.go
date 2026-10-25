package render

import (
	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/tenn"
)

// Editor parts of the Swift scene: find, updateLineTo, updateLayout, SelectorBox and SelectorLine.

// Everywhere is a dirty area that never culls a box moved away from its place.
var Everywhere = Rect{-1e9, -1e9, 2e9, 2e9}

// DrawableOf is the item a drawable draws, nil for text and containers.
func DrawableOf(d Drawable) *model.DiagramItem {
	if h, ok := d.(interface{ DrawableItem() *model.DiagramItem }); ok {
		return h.DrawableItem()
	}
	return nil
}

// Items lists the built items: boxes in item order, then links (the order of the TS Map).
// Read-only.
func (s *DrawableScene) Items() []*model.DiagramItem { return s.order }

// NewRecorder records in the coordinates of Picture, measuring text as the scene does.
func (s *DrawableScene) NewRecorder() *gfx.Recorder { return gfx.NewRecorderWith(s.env.measurer) }

// Find is DrawableScene.find: the items under p in the order of the scene's children. Boxes win
// over lines, lines are found only where no box is hit. Boxes on layers other than the default are
// hit with all.
func (s *DrawableScene) Find(p Point, all bool) []*model.DiagramItem {
	var boxes, lines []*model.DiagramItem
	for _, c := range s.elements.Children {
		item := DrawableOf(c)
		if item == nil || !(all || c.Selectable()) {
			continue
		}
		if l, ok := c.(*DrawableLine); ok {
			if l.Find(p) {
				lines = append(lines, item)
			}
		} else if containsPoint(c.SelectorBounds(), p) {
			boxes = append(boxes, item)
		}
	}
	if len(boxes) > 0 {
		return boxes
	}
	return lines
}

// Find reports whether p is on the line. Swift also probes the path of "quad" lines; here a quad is
// hit along its polyline only.
func (l *DrawableLine) Find(p Point) bool {
	pts := append(append([]Point{l.Source}, l.ExtraPoints...), l.Target)
	for i := 1; i < len(pts); i++ {
		if crossPointLine(pts[i-1], pts[i], p) {
			return true
		}
	}
	return false
}

// LabelBounds is DrawableLine.getLabelBounds: the box an inline editor of the line's label takes.
func (l *DrawableLine) LabelBounds() Rect {
	if l.Label != nil {
		pt := l.labelPosition(l.Label)
		if len(l.ExtraPoints) > 0 {
			ep := l.ExtraPoints[0]
			pt = Point{ep.X + 5, ep.Y - l.Label.Bounds().Height}
		}
		lb := l.Label.SelectorBounds()
		return Rect{pt.X, pt.Y, lb.Width, lb.Height}
	}
	b := l.Bounds()
	return Rect{midX(b) - 50, midY(b) - 15, 100, 30}
}

// UpdateLineTo is DrawableScene.updateLineTo: the preview line from source to p, or to the middle
// of target (the item under p, found by the caller) when it has a drawable.
func (s *DrawableScene) UpdateLineTo(source *model.DiagramItem, p Point, target *model.DiagramItem) {
	sd, ok := s.Drawables[source]
	if !ok {
		return
	}
	sb := sd.Bounds()
	end := p
	var tb *Rect
	if target != nil {
		if td, ok := s.Drawables[target]; ok {
			b := td.Bounds()
			tb = &b
			end = Point{midX(b), midY(b)}
		}
	}
	mid := Point{midX(sb), midY(sb)}
	if cp, ok := crossBox(mid, end, sb); ok {
		mid = cp
	}
	if tb != nil {
		if cp, ok := crossBox(mid, end, *tb); ok {
			end = cp
		}
	}
	s.LineTo = NewDrawableLineBetween(mid, end, s.lineToStyle())
}

func (s *DrawableScene) lineToStyle() *DrawableLineStyle {
	st := NewDrawableLineStyle(s.DarkMode)
	st.ScreenShadow = s.opts.ScreenShadows
	return st
}

// RemoveLineTo drops the preview line.
func (s *DrawableScene) RemoveLineTo() { s.LineTo = nil }

// UpdateLayout is DrawableScene.updateLayout: positions are item positions (pos, y-up), a link
// takes its position as the control point. Returns the drawables whose picture changed, in scene
// order.
func (s *DrawableScene) UpdateLayout(positions map[*model.DiagramItem]Point) []Drawable {
	moved := map[Drawable]bool{}
	for item, pos := range positions {
		d := s.Drawables[item]
		switch b := d.(type) {
		case interface {
			Drawable
			SetPath(Rect)
		}:
			sb := b.SelectorBounds()
			b.SetPath(Rect{pos.X, pos.Y - sb.Height, sb.Width, sb.Height})
		case *DrawableLine:
			b.Control = pos
			b.UpdateLayout(b.SourceRect, b.TargetRect)
		default:
			continue
		}
		moved[d] = true
		for _, l := range s.ItemToLink[item] {
			line, ok := s.Drawables[l].(*DrawableLine)
			if !ok || l.Source == nil || l.Target == nil {
				continue
			}
			sd, td := s.Drawables[l.Source], s.Drawables[l.Target]
			if sd != nil && td != nil {
				line.UpdateLayout(sd.SelectorBounds(), td.SelectorBounds())
				moved[line] = true
			}
		}
	}
	s.Layout(s.Bounds(), Everywhere)
	var out []Drawable
	for _, c := range s.elements.Children {
		if moved[c] {
			out = append(out, c)
		}
	}
	return out
}

// GetString is getString without evaluated values: the text of a node as written.
func GetString(n *tenn.Node) (string, bool) { return getString(n, nil) }

// PrepareBodyText is the text of a body as an editor shows it: escaped newlines expanded, tabs as
// blanks, the empty first and last line dropped.
func PrepareBodyText(s string) string { return prepareBodyText(s) }

// ValueString is how an evaluated value reads as text (JSValue.toString()).
func ValueString(v any) string { return jsToString(v) }

var (
	selectorBlue  = Color{0, 0, 1, 1}
	selectorGreen = Color{0, 1, 0, 1}
)

// SelectorBox is the dashed rounded outline around a selected item (Swift SelectorBox); Green marks
// the item being edited. Pos and Size are in the y-up space.
type SelectorBox struct {
	Pos    Point
	Size   Size
	Radius float64
	Green  bool
	// Blur is the glow, 5 as in Swift.
	Blur float64
	// Screen keeps the glow constant in screen points while the view zooms.
	Screen bool
}

// NewSelectorBox is the outline of bounds, grown by 5 on each side as Swift does for an item.
func NewSelectorBox(bounds Rect, green bool) *SelectorBox {
	return &SelectorBox{Pos: Point{bounds.X - 5, bounds.Y - 5}, Size: Size{bounds.Width + 10, bounds.Height + 10}, Radius: 9, Green: green, Blur: 5}
}

func (b *SelectorBox) color() Color {
	if b.Green {
		return selectorGreen
	}
	return selectorBlue
}

// dashedGlow is the stroke every selector uses: dashes 5 on 5 off with a glow of its own colour.
func dashedGlow(rec *gfx.Recorder, p *path, at Point, width, blur float64, c Color, screen bool) {
	col := c.gfxColor()
	st := gfx.StrokeStyle{Width: width, MiterLimit: 10, Dash: []float64{5, 5}, DashPhase: 5}
	rec.StrokeShadowPath(p.toGfx(at.X, at.Y), st, col, gfx.Shadow{Blur: blur, Color: col, Screen: screen})
}

func (b *SelectorBox) Draw(rec *gfx.Recorder, at Point) {
	r := Rect{b.Pos.X, b.Pos.Y, b.Size.Width, b.Size.Height}
	p := newPath()
	p.move(Point{midX(r), minY(r)})
	p.arc(Point{maxX(r), minY(r)}, Point{maxX(r), maxY(r)}, b.Radius)
	p.arc(Point{maxX(r), maxY(r)}, Point{minX(r), maxY(r)}, b.Radius)
	p.arc(Point{minX(r), maxY(r)}, Point{minX(r), minY(r)}, b.Radius)
	p.arc(Point{minX(r), minY(r)}, Point{maxX(r), minY(r)}, b.Radius)
	p.close()
	rec.Save()
	dashedGlow(rec, p, at, 1, b.Blur, b.color(), b.Screen)
	rec.Restore()
}

// SelectorLine is the dashed outline of a selected link (Swift SelectorLine).
type SelectorLine struct {
	Source, Target Point
	Extra          []Point
	Blur           float64
	Screen         bool
}

// NewSelectorLine follows the points of line.
func NewSelectorLine(line *DrawableLine) *SelectorLine {
	return &SelectorLine{Source: line.Source, Target: line.Target, Extra: line.ExtraPoints, Blur: 5}
}

func (l *SelectorLine) Draw(rec *gfx.Recorder, at Point) {
	p := newPath()
	p.move(l.Source)
	for _, ep := range l.Extra {
		p.line(ep)
	}
	p.line(l.Target)
	rec.Save()
	dashedGlow(rec, p, at, 1.5, l.Blur, selectorBlue, l.Screen)
	rec.Restore()
}
