package app

import (
	"strings"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/model"
)

// State is what the window shows: the document, the element on the canvas and the viewport of each
// element visited. It holds no widgets, so reload and selection are testable without a window.
type State struct {
	Doc *Document
	// Current is the element on the canvas; nil with no document.
	Current *model.Element
	// Error is the text for the banner: the parse errors of the last read, or the last I/O error.
	Error string
	// cameras are the viewports of the elements left, by path; a reload keeps them.
	cameras map[string]gfx.Camera
}

// NewState starts with doc already loaded.
func NewState(doc *Document) *State {
	s := &State{cameras: map[string]gfx.Camera{}}
	s.Open(doc)
	return s
}

// InitialElement is what Swift shows on open: the root model itself, with no outline row selected.
func InitialElement(m *model.ElementModel) *model.Element { return &m.Element }

// Open replaces the document with a different file: selection and viewports start over.
func (s *State) Open(doc *Document) {
	s.cameras = map[string]gfx.Camera{}
	s.Doc, s.Current, s.Error = doc, nil, errorText(doc)
	if doc.Model != nil {
		s.Current = InitialElement(doc.Model)
	}
}

// Reload takes the next version of the same file. A version with parse errors keeps the last good
// model on screen and only sets Error; a good one keeps the current element (found again by path, else
// the root).
func (s *State) Reload(doc *Document) {
	if doc.Model == nil {
		s.Error = errorText(doc)
		return
	}
	var path []string
	if s.Current != nil {
		path = PathOf(s.Current)
	}
	s.Doc, s.Error = doc, ""
	if cur := Resolve(&doc.Model.Element, path); cur != nil {
		s.Current = cur
	} else {
		s.Current = InitialElement(doc.Model)
	}
}

// Camera is the remembered viewport of e.
func (s *State) Camera(e *model.Element) (gfx.Camera, bool) {
	c, ok := s.cameras[pathKey(e)]
	return c, ok
}

// Remember stores the viewport of e.
func (s *State) Remember(e *model.Element, c gfx.Camera) { s.cameras[pathKey(e)] = c }

func pathKey(e *model.Element) string { return strings.Join(PathOf(e), "\x00") }

func errorText(doc *Document) string {
	var b strings.Builder
	for i, e := range doc.Errors {
		if i == 5 {
			b.WriteString("...\n")
			break
		}
		b.WriteString(e.String())
		b.WriteByte('\n')
	}
	return strings.TrimSuffix(b.String(), "\n")
}
