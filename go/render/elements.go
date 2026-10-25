package render

import (
	"strings"

	"github.com/haiodo/tenniarb/go/model"
)

// AllElements returns every descendant of e, parents before children.
func AllElements(e *model.Element) []*model.Element {
	var out []*model.Element
	for _, c := range e.Elements {
		out = append(out, c)
		out = append(out, AllElements(c)...)
	}
	return out
}

// Names may contain "/", so a path is matched name by name rather than split.
func byPath(parent *model.Element, rest string) *model.Element {
	for _, c := range parent.Elements {
		if c.Name == rest {
			return c
		}
		if strings.HasPrefix(rest, c.Name+"/") {
			if hit := byPath(c, rest[len(c.Name)+1:]); hit != nil {
				return hit
			}
		}
	}
	return nil
}

// FindElement finds the "A/B" path from root, else the first element named spec anywhere.
func FindElement(root *model.Element, spec string) *model.Element {
	if e := byPath(root, spec); e != nil {
		return e
	}
	for _, c := range AllElements(root) {
		if c.Name == spec {
			return c
		}
	}
	return nil
}
