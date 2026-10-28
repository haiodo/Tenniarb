package app

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/comments"
)

// Pin is the marker of one comment on the canvas.
type Pin struct {
	N        int // the place of the comment in the list of the document, from 1: resolving another one keeps it
	ID       string
	At       gfx.Point // in the gfx scene, y down
	Detached bool      // the node of the anchor is not on the canvas any more: the pin sits at the element origin
	Resolved bool
}

// PlacePins puts a pin for each comment of the element on the canvas. A comment with a node sits at the
// top left of the node's box plus the point, or at its top right corner without a point; box gives the
// box in the gfx scene, ok false when the node is not drawn. A comment without a node sits at origin
// plus the point, where nodeless maps the point of the anchor into the gfx scene.
func PlacePins(cs []comments.Comment, element []string, showResolved bool, box func(node string) (gfx.Rect, bool), nodeless func(comments.Point) gfx.Point) []Pin {
	var out []Pin
	for i, c := range cs {
		if !slices.Equal(c.Anchor.Element, element) || (c.State == comments.Resolved && !showResolved) {
			continue
		}
		at, detached := pinPlace(c.Anchor, box, nodeless)
		out = append(out, Pin{N: i + 1, ID: c.ID, At: at, Detached: detached, Resolved: c.State == comments.Resolved})
	}
	return out
}

func pinPlace(a comments.Anchor, box func(string) (gfx.Rect, bool), nodeless func(comments.Point) gfx.Point) (at gfx.Point, detached bool) {
	if a.Node == "" {
		pt := comments.Point{}
		if a.Point != nil {
			pt = *a.Point
		}
		return nodeless(pt), false
	}
	b, ok := box(a.Node)
	switch {
	case !ok:
		return nodeless(comments.Point{}), true
	case a.Point == nil:
		return gfx.Point{X: b.Max.X, Y: b.Min.Y}, false
	}
	return gfx.Point{X: b.Min.X + a.Point.X, Y: b.Min.Y + a.Point.Y}, false
}

// AnchorAt makes the anchor of a click: node is the item under it ("" on empty space) with its box in the
// gfx scene, and at is the click in the gfx scene. The point is measured from the box, or through
// toElement from the scene without a node.
func AnchorAt(element []string, node string, box gfx.Rect, at gfx.Point, toElement func(gfx.Point) comments.Point) comments.Anchor {
	a := comments.Anchor{Element: slices.Clone(element), Node: node}
	if node == "" {
		pt := toElement(at)
		a.Point = &pt
		return a
	}
	a.Point = &comments.Point{X: at.X - box.Min.X, Y: at.Y - box.Min.Y}
	return a
}

// pinRadius is the size of a marker in view points; a click within it opens the thread.
const pinRadius = 11

// PinAtView is the pin whose marker lies under the view point p (the camera origin and zoom map the
// gfx scene to the view); the one drawn last, topmost, wins.
func PinAtView(pins []Pin, cam gfx.Camera, p gfx.Point) (Pin, bool) {
	for i := len(pins) - 1; i >= 0; i-- {
		c := pinView(pins[i], cam)
		if dx, dy := p.X-c.X, p.Y-c.Y; dx*dx+dy*dy <= pinRadius*pinRadius {
			return pins[i], true
		}
	}
	return Pin{}, false
}

// pinView is the centre of the marker in view points. The marker sits above-right of the anchor, so a pin
// on a corner does not hide the node.
func pinView(p Pin, cam gfx.Camera) gfx.Point {
	return gfx.Point{X: (p.At.X-cam.Origin.X)*cam.Zoom + pinRadius*0.6, Y: (p.At.Y-cam.Origin.Y)*cam.Zoom - pinRadius*0.6}
}

// commentWhen is the time of a comment for a thread: the time today, the date and time before.
func commentWhen(t, now time.Time) string {
	t, now = t.Local(), now.Local()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	return t.Format("2 Jan 15:04")
}

// commentLabel is one line for the list: the number, the author and the start of the text.
func commentLabel(n int, c comments.Comment) string {
	text := strings.Join(strings.Fields(c.Text), " ")
	if r := []rune(text); len(r) > 48 {
		text = string(r[:47]) + "..."
	}
	mark := ""
	if c.State == comments.Resolved {
		mark = " (resolved)"
	}
	return fmt.Sprintf("%d  %s: %s%s", n, c.Author, text, mark)
}

// numberOf is the pin number of the comment id in cs, 0 when it is not there.
func numberOf(cs []comments.Comment, id string) int {
	for i, c := range cs {
		if c.ID == id {
			return i + 1
		}
	}
	return 0
}
