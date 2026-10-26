package app

import (
	"strconv"
	"strings"

	"github.com/haiodo/tenniarb/go/model"
)

// OutlineSignature changes when the outline would look different: an element added, removed, renamed
// or moved, or an element got its first item or lost its last one (the row icon). Other item changes
// do not show in the outline.
func OutlineSignature(root *model.Element) string {
	var b strings.Builder
	var walk func(e *model.Element)
	walk = func(e *model.Element) {
		for _, c := range e.Elements {
			b.WriteString(strconv.Quote(c.Name))
			if len(c.Items) > 0 {
				b.WriteByte('+')
			}
			b.WriteByte('(')
			walk(c)
			b.WriteByte(')')
		}
	}
	walk(root)
	return b.String()
}
