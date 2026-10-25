package app

import (
	"strconv"
	"strings"

	"github.com/haiodo/tenniarb/go/model"
)

// OutlineSignature changes when the outline would look different: an element added, removed, renamed
// or moved. Items do not show in the outline.
func OutlineSignature(root *model.Element) string {
	var b strings.Builder
	var walk func(e *model.Element)
	walk = func(e *model.Element) {
		for _, c := range e.Elements {
			b.WriteString(strconv.Quote(c.Name))
			b.WriteByte('(')
			walk(c)
			b.WriteByte(')')
		}
	}
	walk(root)
	return b.String()
}
