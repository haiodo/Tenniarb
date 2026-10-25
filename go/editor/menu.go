package editor

import (
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
)

// Entry is a context menu entry (SceneDrawView.menu(for:)). A separator has only Sep set; an entry
// with Sub opens a submenu, any other runs Run.
type Entry struct {
	Label string
	// Icon is "plus" or "minus", the glyph of the Swift menu; empty for none.
	Icon string
	Run  func()
	Sub  []Entry
	Sep  bool
}

var separator = Entry{Sep: true}

// MenuHost are the actions of the context menu that only the window can do; a nil one drops its
// entry.
type MenuHost struct {
	// Layout runs "Test layout" (the window knows the view size and recentres after).
	Layout func()
	// AttachImage asks for a file and calls AttachImage.
	AttachImage func()
	// ExportHTML copies the text of the selected item as HTML.
	ExportHTML func()
}

// QuickEntries are the options of one quick style; markers are grouped as in the Swift marker menu.
func (s *Session) QuickEntries(q QuickStyle) []Entry {
	pick := func(o string) Entry {
		return Entry{Label: OptionLabel(o), Run: func() { s.SetQuickStyle(q.Prop, o) }}
	}
	var out []Entry
	if q.Prop == "marker" {
		for _, g := range Markers {
			e := Entry{Label: g.Label}
			for _, m := range g.Items {
				e.Sub = append(e.Sub, pick(m))
			}
			out = append(out, e)
		}
		return out
	}
	for _, o := range q.Options {
		out = append(out, pick(o))
	}
	return out
}

// StyleEntries are the "Style" and "Quick Style" entries for the selection (createStylesMenu,
// createQuickStyleMenu).
func (s *Session) StyleEntries() []Entry {
	names := s.StyleNames()
	var sub []Entry
	for _, n := range names {
		sub = append(sub, Entry{Label: n, Run: func() { s.ApplyStyle(n) }})
	}
	if len(names) > 0 {
		sub = append(sub, separator)
	}
	sub = append(sub, Entry{Label: "Define new style", Run: func() { s.DefineStyle() }})
	style := Entry{Label: "Style", Sub: sub}

	switch len(s.selection) {
	case 0:
		return []Entry{style, {Label: "Global Styles", Sub: []Entry{{Label: "Enable shadows", Run: s.EnableShadows}}}}
	case 1:
	default:
		return []Entry{style}
	}
	var quick []Entry
	for _, q := range QuickStylesFor(s.selection[0].Kind) {
		if q.Sep {
			quick = append(quick, separator)
		}
		quick = append(quick, Entry{Label: q.Label, Sub: s.QuickEntries(q)})
	}
	return []Entry{style, {Label: "Quick Style", Sub: quick}}
}

// CanvasEntries is Swift menu(for:): p is the right-click point (scene space). Call Pick(p) first.
func (s *Session) CanvasEntries(p render.Point, host MenuHost) []Entry {
	sel := s.selection
	under := s.scene.Find(p, true)
	// Overlapping items to pick from; not offered when the plain hit test finds the same single one.
	var pickEntries []Entry
	if len(under) != len(s.scene.Find(p, false)) || len(under) > 1 {
		pickEntries = []Entry{separator}
		sub := Entry{Label: "Select"}
		for _, i := range under {
			label := i.Name
			if label == "" {
				label = "Link"
			}
			sub.Sub = append(sub.Sub, Entry{Label: label, Run: func() { s.Select([]*model.DiagramItem{i}) }})
		}
		pickEntries = append(pickEntries, sub)
	}
	add := Entry{Label: "New item", Icon: "plus", Run: s.AddTopItem}
	if len(sel) == 0 {
		out := []Entry{add}
		if host.Layout != nil {
			out = append(out, Entry{Label: "Test layout", Run: host.Layout})
		}
		out = append(out, separator)
		out = append(out, s.StyleEntries()...)
		return append(out, pickEntries...)
	}

	out := []Entry{
		add, separator,
		{Label: "New linked item", Run: func() { s.AddNewItem(false) }},
		{Label: "Linked styled item", Run: func() { s.AddNewItem(true) }},
		separator,
	}
	out = append(out, s.StyleEntries()...)
	if len(sel) > 1 {
		align := func(label, edge string) Entry { return Entry{Label: label, Run: func() { s.Align(edge) }} }
		out = append(out, separator, Entry{Label: "Align", Sub: []Entry{
			align("Leading Edges", "leading"), align("Trailing Edges", "trailing"), align("Top Edges", "top"), align("Bottom Edges", "bottom"),
		}})
	}
	out = append(out, separator, Entry{Label: "Duplicate", Run: s.Duplicate})
	if len(sel) == 1 {
		out = append(out, separator)
		if host.AttachImage != nil {
			out = append(out, Entry{Label: "Attach image", Run: host.AttachImage})
		}
		out = append(out, Entry{Label: "Order", Sub: []Entry{
			{Label: "Move Forward", Run: func() { s.Order(true) }},
			{Label: "Move Backward", Run: func() { s.Order(false) }},
		}})
		if host.ExportHTML != nil {
			out = append(out, separator, Entry{Label: "Export text as html", Run: host.ExportHTML})
		}
	}
	out = append(out, pickEntries...)
	return append(out, separator, Entry{Label: "Delete", Icon: "minus", Run: s.DeleteSelection})
}

// QuickSegment is one button of the quick style panel over a single selected item.
type QuickSegment struct {
	Style QuickStyle
	// Label is the glyph of the button.
	Label string
}

// quickSegments is the Swift segment order; links get only the last three.
var quickSegments = []struct {
	prop, label string
	itemOnly    bool
}{
	{"marker", "✑", true},
	{"font-size", "Ƭ", true},
	{"display", "❑", false},
	{"color", "🔴", true},
	{"line-style", "⊞", false},
	{"line-width", "〰", false},
}

// QuickPanel are the buttons of the quick style panel (Swift SceneDrawView.showPopup) for an item
// or a link; each opens QuickEntries of its style.
func QuickPanel(kind model.ItemKind) []QuickSegment {
	styles := QuickStylesFor(kind)
	var out []QuickSegment
	for _, seg := range quickSegments {
		if seg.itemOnly && kind != model.ItemKindItem {
			continue
		}
		for _, q := range styles {
			if q.Prop == seg.prop {
				out = append(out, QuickSegment{q, seg.label})
			}
		}
	}
	return out
}
