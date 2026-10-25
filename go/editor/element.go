package editor

import (
	"slices"
	"strconv"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/tenn"
)

// The commands of the outline (ViewController handleAddElement, duplicate, copy, paste, delete,
// drop).

// AddElement adds a child element to parent (the document when nil); readonly: nil.
func (s *Session) AddElement(parent *model.Element) *model.Element {
	defer s.EndEvent()
	if s.opts.Readonly {
		return nil
	}
	if parent == nil {
		parent = &s.root.Element
	}
	el := model.NewElement("Unnamed element: " + strconv.Itoa(s.elementCount))
	s.elementCount++
	s.fail(s.store.Add(parent, el, s.um, s.refresh, nil))
	return el
}

// DuplicateElement adds a copy with its items next to the original (Swift duplicateItem of the
// outline).
func (s *Session) DuplicateElement(element *model.Element) *model.Element {
	defer s.EndEvent()
	if s.opts.Readonly || element.Parent == nil {
		return nil
	}
	cp := element.Clone(true, true)
	s.fail(s.store.Add(element.Parent, cp, s.um, s.refresh, nil))
	return cp
}

// CopyElement is the outline copy: the element with its items and sub-elements as .tenn text.
func (s *Session) CopyElement(element *model.Element) string {
	return model.ToTennStr(element, true, true)
}

// CutElement copies the element, then deletes it.
func (s *Session) CutElement(element *model.Element) string {
	text := s.CopyElement(element)
	s.RemoveElement(element)
	return text
}

// PasteElements adds the elements of a .tenn text into parent as one undo step. False: readonly,
// parse errors or no elements.
func (s *Session) PasteElements(parent *model.Element, text string) bool {
	defer s.EndEvent()
	if s.opts.Readonly {
		return false
	}
	m := model.ReadTenn(text)
	if m == nil || len(m.Elements) == 0 {
		return false
	}
	if parent == nil {
		parent = &s.root.Element
	}
	s.fail(s.store.AddElements(parent, slices.Clone(m.Elements), s.um, s.refresh))
	return true
}

// RenameElement renames an element, one undo step.
func (s *Session) RenameElement(element *model.Element, name string) {
	defer s.EndEvent()
	if s.opts.Readonly || name == element.Name {
		return
	}
	s.fail(s.store.UpdateElementName(element, name, s.um, s.refresh))
}

// RemoveElement deletes an element. When it is the one on the canvas, its nearest ancestor takes
// its place. False: readonly, or the element is the document.
func (s *Session) RemoveElement(element *model.Element) bool {
	defer s.EndEvent()
	if s.opts.Readonly || element.Parent == nil {
		return false
	}
	s.fail(s.store.Remove(element.Parent, element, s.um, s.refresh))
	return true
}

// MoveElement puts element into parent at index (default: last, use len(parent.Elements)). Into
// itself or its own subtree a copy of the diagram is added instead (Swift acceptDrop). False:
// readonly.
func (s *Session) MoveElement(element, parent *model.Element, index int) bool {
	defer s.EndEvent()
	if s.opts.Readonly || element.Parent == nil {
		return false
	}
	for c := parent; c != nil; c = c.Parent {
		if c == element {
			s.fail(s.store.Add(parent, element.Clone(true, false), s.um, s.refresh, model.IntPtr(index)))
			return true
		}
	}
	// The element leaves its old slot first, so a move down inside one parent lands one place earlier.
	at := index
	if element.Parent == parent && slices.Index(parent.Elements, element) < index {
		at--
	}
	s.fail(s.store.Move(element, parent, s.um, s.refresh, at))
	return true
}

// Inherit is ViewController.inheritItem: a copy of the current element inside it, whose items only
// `inherit` the originals.
func (s *Session) Inherit() {
	defer s.EndEvent()
	if s.opts.Readonly {
		return
	}
	cp := s.element.Clone(true, true)
	refs := model.PrepareItemRefs(cp.Items)
	for _, i := range cp.Items {
		cmd := tenn.NewCommand("inherit", tenn.NewStrNode("../"+i.Name))
		if ref, ok := refs[i]; ok {
			cmd.Add(tenn.NewIntNode(ref))
		}
		i.Properties = model.NewModelProperties()
		i.Properties.Append(cmd)
	}
	s.fail(s.store.Add(s.element, cp, s.um, s.refresh, nil))
}
