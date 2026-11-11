//go:build darwin || linux || windows

package app

import (
	"runtime"
	"slices"

	"github.com/haiodo/gowt/look"
)

// toolbar is the unified window toolbar: element, item pane and sidebar buttons, zoom, new item, inspector, and the
// Goto Item search.
func (v *Viewer) toolbar() {
	// macOS puts the inspector items in the window bar, where the Goto Item field has room; Windows and
	// Linux put them at the top of the panel, so there the field is a popover opened by this button.
	searchItem := look.ToolbarItem{Title: "Goto Item", ID: "goto", Symbol: "magnifyingglass", Inspector: true, Small: true, OnClick: v.showSearch}
	if runtime.GOOS == "darwin" {
		searchItem = look.ToolbarItem{Title: "Goto Item", ID: "goto-field", Inspector: true, Small: true, OnSearch: v.searchTo}
	}
	items := []look.ToolbarItem{
		{Title: "Element", Sidebar: true, Small: true, Group: []look.ToolbarItem{
			{Title: "New Element", Symbol: "plus", OnClick: func() { v.addElement(v.state.Current) }},
			{Title: "Delete Element", Symbol: "minus", OnClick: v.removeElement},
		}},
		look.ToolbarItem{Title: "Items", Symbol: "list.bullet", Sidebar: true, Small: true, OnClick: v.toggleItems},
		look.ToolbarItem{Title: "Hide Sidebar", Symbol: "sidebar.left", Sidebar: true, Small: true, OnClick: v.toggleSidebar},
		look.ToolbarItem{Space: true},
		look.ToolbarItem{Title: "Zoom", Small: true, Group: []look.ToolbarItem{
			{Title: "Zoom Out", Symbol: "minus.magnifyingglass", OnClick: func() { v.zoomBy(1 / ZoomStep) }},
			{Title: "Zoom In", Symbol: "plus.magnifyingglass", OnClick: func() { v.zoomBy(ZoomStep) }},
			{Title: "Fit to Window", Symbol: "arrow.up.left.and.arrow.down.right", OnClick: v.fitCommand},
		}},
		look.ToolbarItem{Title: "Reset Zoom", Text: v.zoomText, Small: true, OnClick: v.resetZoom},
		look.ToolbarItem{Title: "Comment Mode (C)", Symbol: "text.bubble", Small: true, ID: "comment-mode", OnClick: v.toggleCommentMode},
		look.ToolbarItem{Space: true, Inspector: true},
		searchItem,
		look.ToolbarItem{Title: "Hide Code", Symbol: "rectangle.bottomthird.inset.filled", Inspector: true, Small: true, OnClick: v.toggleCode},
		look.ToolbarItem{Title: "Hide Inspector", Symbol: "sidebar.right", Inspector: true, Small: true, OnClick: v.toggleInspector},
	}
	// The agent items sit next to the title, before the zoom controls.
	agents := v.agentItems()
	items = slices.Insert(items, 3, agents...)
	// A pages export long enough to show progress gets the button after the zoom controls.
	if v.exporting.active {
		items = slices.Insert(items, 6+len(agents), look.ToolbarItem{
			Title: "Export pages in progress; click for what is left", Text: v.exporting.text, Small: true,
			ID: "export-progress", OnClick: v.showExportProgress,
		})
	}
	look.SetToolbar(v.win, items...)
}

func (v *Viewer) toggleSidebar() { look.SetSidebarVisible(v.win, !look.SidebarVisible(v.win)) }

func (v *Viewer) toggleInspector() { look.SetInspectorVisible(v.win, !look.InspectorVisible(v.win)) }

func (v *Viewer) toggleCode() { look.SetBottomPanelVisible(v.win, !look.BottomPanelVisible(v.win)) }

func (v *Viewer) fitCommand() {
	if _, _, ok := v.contentSize(); ok {
		v.fitView()
	}
}

// contentSize is the size of what the canvas shows, padding included.
func (v *Viewer) contentSize() (w, h float64, ok bool) {
	if v.sv == nil {
		return 0, 0, false
	}
	return v.sv.w, v.sv.h, true
}
