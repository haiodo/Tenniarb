package editor

import (
	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
)

// Overlay records what the session adds over the diagram: the dashed outlines of the selection
// (green for the item being edited), the rubber band and the preview of a link being drawn. The
// picture uses the coordinates of render.DrawableScene.Picture, so a window puts it into the node
// of the realized scene (render.Rendered.Root) above the diagram and records it again after each
// redraw, without touching the diagram. A picture with Len() == 0 draws nothing.
func (s *Session) Overlay() *gfx.Picture {
	rec := s.scene.NewRecorder()
	// While a link is drawn the item under the pointer is the one outlined (Swift updateLineTo).
	selected := s.selection
	if s.mode == modeLine && s.lineTarget != nil {
		selected = []*model.DiagramItem{s.lineTarget}
	}
	// The glow is in screen points in Swift: with Screen shadows (render.Options.ScreenShadows, the
	// window) gfx keeps it so; without them the picture has none of the zoom, so it is scaled back.
	screen := s.opts.Render.ScreenShadows
	blur := 5.0
	if !screen {
		blur = 5 / s.zoom
	}
	for _, item := range selected {
		switch d := s.scene.Drawables[item].(type) {
		case nil:
		case *render.DrawableLine:
			l := render.NewSelectorLine(d)
			l.Blur, l.Screen = blur, screen
			l.Draw(rec, render.Point{})
		default:
			b := d.SelectorBounds()
			green := s.editing != nil && s.editing.item == item
			if green {
				b.Width, b.Height = s.editing.rect.Width, s.editing.rect.Height
			}
			box := render.NewSelectorBox(b, green)
			box.Blur, box.Screen = blur, screen
			box.Draw(rec, render.Point{})
		}
	}
	if s.band != nil {
		box := &render.SelectorBox{Pos: render.Point{X: s.band.X, Y: s.band.Y}, Size: render.Size{Width: s.band.Width, Height: s.band.Height}, Blur: blur, Screen: screen}
		box.Draw(rec, render.Point{})
	}
	if s.scene.LineTo != nil {
		s.scene.LineTo.Draw(rec, render.Point{})
	}
	return rec.Finish()
}
