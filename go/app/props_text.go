package app

import (
	"slices"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
)

// PropSpan is a colored span of the properties text. Start and Length count runes, as gowt StyledText
// does; the editor package counts bytes.
type PropSpan struct {
	Start, Length int
	Class         editor.HighlightClass
	// Error marks a parse error instead of a token class.
	Error bool
}

// runeIndex maps byte offsets of text to rune offsets.
func runeIndex(text string) func(byteOff int) int {
	at := make([]int, 0, len(text)+1) // at[b] is the rune count before byte b
	n := 0
	for i := range text {
		for len(at) < i {
			at = append(at, n) // continuation bytes of the previous rune
		}
		at = append(at, n)
		n++
	}
	for len(at) <= len(text) {
		at = append(at, n)
	}
	return func(b int) int { return at[max(0, min(b, len(text)))] }
}

// PropSpans are the token colors of text and its parse errors, in rune offsets, ordered by start.
// Error spans come after the token spans, so a style applied in order lets the error win.
func PropSpans(text string) []PropSpan {
	idx := runeIndex(text)
	var out []PropSpan
	for _, r := range editor.HighlightRanges(text) {
		out = append(out, PropSpan{Start: idx(r.From), Length: idx(r.To) - idx(r.From), Class: r.Class})
	}
	slices.SortStableFunc(out, func(a, b PropSpan) int { return a.Start - b.Start })
	for _, d := range editor.ParseDiagnostics(text) {
		out = append(out, PropSpan{Start: idx(d.From), Length: idx(d.To) - idx(d.From), Error: true})
	}
	return out
}

// DropZone is the part of an outline row a drop lands on.
type DropZone int

const (
	DropBefore DropZone = iota
	DropInto
	DropAfter
)

// ZoneAt is the zone of a row that starts at top and is height tall: the outer quarters put the
// element next to the row, the middle puts it inside.
func ZoneAt(y, top, height int) DropZone {
	if height <= 0 {
		return DropInto
	}
	switch rel := (y - top) * 4 / height; {
	case rel <= 0:
		return DropBefore
	case rel >= 3:
		return DropAfter
	}
	return DropInto
}

// DropPlace is the parent and index for MoveElement; a nil target is the space below the rows.
// ok is false for a drop that changes nothing.
func DropPlace(root, dragged, target *model.Element, zone DropZone) (parent *model.Element, index int, ok bool) {
	if target == nil {
		return root, len(root.Elements), true
	}
	if target == dragged {
		return nil, 0, false
	}
	if zone == DropInto {
		return target, len(target.Elements), true
	}
	parent = target.Parent
	if parent == nil {
		return root, len(root.Elements), true
	}
	index = slices.Index(parent.Elements, target)
	if zone == DropAfter {
		index++
	}
	if own := slices.Index(parent.Elements, dragged); parent == dragged.Parent && (index == own || index == own+1) {
		return nil, 0, false
	}
	return parent, index, true
}
