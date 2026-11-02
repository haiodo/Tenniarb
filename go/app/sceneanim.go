package app

import (
	"math"
	"os"
	"slices"
	"time"

	"github.com/haiodo/gowt/gfx"
	gqa "github.com/haiodo/gowt/qa"

	"github.com/haiodo/tenniarb/go/scene"
)

// The canvas plays the animations of the shown scene the way the SVG export does: a timer sets the
// transform and the opacity of each animated node for the moment and the scene repaints. Only what the
// canvas shows without a repaint of the picture is played - rotate, scale and opacity; colours and pos
// stay at their first frame.

const animFrame = 16 * time.Millisecond

// animStart starts the clock of the animations of sc, a new screen, and stops the timer of the one
// before: a screen without animations steps the timer.
func (v *Viewer) animStart(sc *scene.Screen) {
	if sc == v.animScreen {
		return
	}
	v.stopAnim()
	v.animScreen = nil
	if sc == nil || len(sc.AnimatedNodes()) == 0 {
		return
	}
	v.animFrom, v.animScreen = time.Now(), sc
	if v.app != nil && os.Getenv(gqa.EnvSocket) == "" {
		// Every, not After: a short timer counts as pending and keeps the QA harness from settling. The QA
		// driver freezes the animation: its settle waits for two equal frames, and a spinning scene never
		// gives one.
		v.animCancel = v.app.Every(animFrame, v.animTick)
	}
}

// stopAnim ends the frame timer, if one runs.
func (v *Viewer) stopAnim() {
	if v.animCancel != nil {
		v.animCancel()
		v.animCancel = nil
	}
}

// animTick plays one frame.
func (v *Viewer) animTick() {
	if v.sh != nil && !slices.Contains(v.sh.viewers, v) { // the window is closed
		v.stopAnim()
		return
	}
	sc := v.animScreen
	if sc == nil || v.sv == nil || v.sv.screen != sc || v.scene == nil {
		v.stopAnim()
		return
	}
	now := time.Since(v.animFrom)
	v.animTicks++
	v.scene.Update(func(tx *gfx.Tx) {
		for _, a := range sc.AnimatedNodes() {
			rot, scale, opacity := animValues(a.Anim, now)
			cx, cy := a.Box.X+a.Box.W/2, a.Box.Y+a.Box.H/2
			m := gfx.TranslateBy(cx, cy)
			if rot != 0 {
				m = m.Mul(gfx.RotateBy(rot * math.Pi / 180))
			}
			if scale != 1 {
				m = m.Mul(gfx.ScaleBy(scale, scale))
			}
			m = m.Mul(gfx.TranslateBy(-cx, -cy)).Mul(gfx.TranslateBy(a.Box.X, a.Box.Y))
			tx.SetTransform(a.Node, m)
			tx.SetOpacity(a.Node, a.Opacity*opacity)
		}
	})
}

// animValues are the rotate in degrees, the scale and the opacity of the animation a at now, from its
// frames.
func animValues(a *scene.Animation, now time.Duration) (rot, scale, opacity float64) {
	if len(a.Frames) == 0 || a.Duration <= 0 {
		return 0, 1, 1
	}
	total := a.Duration
	switch {
	case a.Repeat < 0: // forever
		total = 0
	case a.Repeat > 1:
		total *= time.Duration(a.Repeat)
	}
	if total > 0 && now >= total {
		now = total - 1 // the last frame stays
	}
	at := float64(now%a.Duration) / float64(a.Duration)
	lo, hi := a.Frames[0], a.Frames[len(a.Frames)-1]
	for i, f := range a.Frames {
		if f.Offset > at {
			break
		}
		lo = f
		if i+1 < len(a.Frames) {
			hi = a.Frames[i+1]
		}
	}
	k := 1.0
	if span := hi.Offset - lo.Offset; span > 0 {
		k = math.Min(1, math.Max(0, (at-lo.Offset)/span))
	}
	k = ease(a.Easing, k)
	r0, s0, o0 := frameValues(lo)
	r1, s1, o1 := frameValues(hi)
	return r0 + (r1-r0)*k, s0 + (s1-s0)*k, o0 + (o1-o0)*k
}

// frameValues are the rotate, the scale and the opacity a frame sets, and their defaults.
func frameValues(f scene.Frame) (rot, scale, opacity float64) {
	scale, opacity = 1, 1
	for _, p := range f.Props {
		v, ok := p.Num(0)
		if !ok {
			continue
		}
		switch p.Name {
		case "rotate":
			rot = v
		case "scale":
			scale = v
		case "opacity":
			opacity = v
		}
	}
	return rot, scale, opacity
}

// ease maps the progress inside a segment by the CSS easing of the animation.
func ease(name string, t float64) float64 {
	switch name {
	case "linear":
		return t
	case "ease-in":
		return bezier(t, 0.42, 0, 1, 1)
	case "ease-out":
		return bezier(t, 0, 0, 0.58, 1)
	case "ease-in-out":
		return bezier(t, 0.42, 0, 0.58, 1)
	default: // ease
		return bezier(t, 0.25, 0.1, 0.25, 1)
	}
}

// bezier solves a cubic-bezier timing function: the y of the curve at x t.
func bezier(t, x1, y1, x2, y2 float64) float64 {
	if t <= 0 || t >= 1 {
		return t
	}
	curve := func(s, p1, p2 float64) float64 {
		return 3*(1-s)*(1-s)*s*p1 + 3*(1-s)*s*s*p2 + s*s*s
	}
	lo, hi := 0.0, 1.0
	for i := 0; i < 30; i++ {
		if mid := (lo + hi) / 2; curve(mid, x1, x2) < t {
			lo = mid
		} else {
			hi = mid
		}
	}
	return curve((lo+hi)/2, y1, y2)
}
