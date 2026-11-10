package editor

import (
	"math"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/render"
)

// Port of SceneDrawView.drawRulers: dots at the view edges for items that are outside it, a dot
// grows with the number of items it stands for.

const (
	yCount = 20
	xCount = 30
)

// Dot is one indicator in the pixels of the view: y up, the origin at the lower-left corner of the view,
// so a dot keeps its size on screen whatever the zoom. X and Y are the lower-left corner of the square
// the dot is drawn in.
type Dot struct {
	X, Y, Size float64
	Count      int
}

// IndicatorDots counts the bounds that lie outside visible (the part of the scene on screen, y up)
// and places the dots.
func IndicatorDots(bounds []render.Rect, visible render.Rect) []Dot {
	w, h := visible.Width, visible.Height
	ox, oy := -visible.X, -visible.Y
	ystep, xstep := h/yCount, w/xCount
	var left, right [yCount + 1]int
	var top, bottom [xCount + 1]int
	clampY := func(v float64) int { return int(math.Min(math.Max(math.Trunc(v/ystep), 0), yCount)) }

	for _, db := range bounds {
		x, y := db.X+ox, db.Y+oy
		if x+db.Width < 0 {
			left[clampY(y+db.Height/2)]++
		}
		if x > w {
			right[clampY(y+db.Height/2)]++
		}
		// Items above or below fall into the x-column; those beyond the corners count towards the side
		// boxes.
		xpos := int(math.Trunc((x + db.Width/2) / xstep))
		if y+db.Height < 0 {
			switch {
			case xpos >= xCount:
				right[0]++
			case xpos <= 0:
				left[0]++
			default:
				bottom[xpos]++
			}
		}
		if y > h {
			switch {
			case xpos >= xCount:
				right[yCount]++
			case xpos <= 0:
				left[yCount]++
			default:
				top[xpos]++
			}
		}
	}

	var dots []Dot
	dot := func(x, y float64, count int) {
		dots = append(dots, Dot{x, y, 5 + math.Min(float64(count), 50)*0.2, count})
	}
	capOf := func(n int) float64 { return math.Min(float64(n), 50) * 0.2 }
	for i := 0; i <= yCount; i++ {
		y := ystep*float64(i) + 2
		if i == yCount {
			y -= capOf(left[i]) + 7
		}
		if left[i] > 0 {
			dot(2, y, left[i])
		}
		if right[i] > 0 {
			ry := y
			if i == yCount {
				ry = h - capOf(right[i]) - 7
			}
			dot(w-7-capOf(right[i]), ry, right[i])
		}
	}
	for i := 0; i <= xCount; i++ {
		x := xstep*float64(i) + 2
		shift := 0.0
		if i == xCount {
			shift = capOf(bottom[i]) + 7
		}
		if bottom[i] > 0 {
			dot(x-shift, 2, bottom[i])
		}
		if i == xCount {
			shift = capOf(top[i]) + 7
		}
		if top[i] > 0 {
			dot(x-shift, h-7-capOf(top[i]), top[i])
		}
	}
	return dots
}

// Indicators records the dots for the elements outside visible, the scene rectangle (y up) that the
// view of k pixels per unit shows. The picture is in view pixels, y down, from the top-left corner
// of the view.
func (s *Session) Indicators(visible render.Rect, k float64, dark bool) *gfx.Picture {
	items := s.geom().Items()
	bounds := make([]render.Rect, 0, len(items))
	for _, i := range items {
		b, ok := s.geom().PaintBox(i)
		if !ok {
			continue
		}
		bounds = append(bounds, render.Rect{
			X:      (b.X - visible.X) * k,
			Y:      (b.Y - visible.Y) * k,
			Width:  b.Width * k,
			Height: b.Height * k,
		})
	}
	px := render.Rect{Width: visible.Width * k, Height: visible.Height * k}
	return IndicatorsPicture(IndicatorDots(bounds, px), px.Height, dark)
}

// IndicatorsPicture draws dots on a view of the given pixel height. The sizes and the offsets of the dots
// are pixels of the view, so they do not follow the zoom.
func IndicatorsPicture(dots []Dot, height float64, dark bool) *gfx.Picture {
	rec := gfx.NewRecorder()
	// Swift colours: black, or orange on a dark diagram, with a soft shadow down and right.
	color := gfx.RGB(0, 0, 0)
	if dark {
		color = gfx.RGB(227, 157, 68)
	}
	sh := gfx.Shadow{DX: 3, DY: 3, Blur: 5, Color: gfx.RGBA(0, 0, 0, 0.33)}
	for _, d := range dots {
		r := d.Size / 2
		p := gfx.EllipsePath(gfx.R(d.X+d.Size/2-r, height-(d.Y+d.Size/2)-r, 2*r, 2*r))
		rec.FillShadowPath(p, color, sh)
		rec.StrokeShadowPath(p, gfx.StrokeStyle{Width: 1, MiterLimit: 10}, color, sh)
	}
	return rec.Finish()
}
