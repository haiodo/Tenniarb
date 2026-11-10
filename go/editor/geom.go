package editor

import (
	"github.com/haiodo/tenniarb/go/geom"
)

// geom is the geometry the session works with, given by whatever draws the diagram.
func (s *Session) geom() geom.Geom { return s.opts.Geom }

// Geom is the geometry a session is given: the canvas answers it, and go/scenegeom answers it when no
// window draws the diagram.
type Geom = geom.Geom
