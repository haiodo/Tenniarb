//go:build darwin || linux || windows

package app

import (
	"slices"
	"testing"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
)

// TestRowEntries: a component row inserts, a group row has no menu, and an item row keeps its own commands.
func TestRowEntries(t *testing.T) {
	labels := func(entries []editor.Entry) []string {
		out := make([]string, 0, len(entries))
		for _, e := range entries {
			if e.Sep {
				out = append(out, "-")
				continue
			}
			out = append(out, e.Label)
		}
		return out
	}
	noop := func() {}
	for _, c := range []struct {
		name    string
		row     *ItemRow
		handled bool
		want    []string
	}{
		{"item", &ItemRow{Item: &model.DiagramItem{}}, true, []string{"Duplicate", "Delete", "-", "Copy Link"}},
		{"component", &ItemRow{Component: true}, true, []string{"Insert"}},
		{"group", &ItemRow{Kind: "group"}, true, nil},
		{"node", &ItemRow{Path: "Frame"}, false, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			entries, handled := rowEntries(c.row, noop, noop, noop, noop)
			if handled != c.handled {
				t.Fatalf("handled = %v, want %v", handled, c.handled)
			}
			if got := labels(entries); !slices.Equal(got, c.want) {
				t.Fatalf("menu = %q, want %q", got, c.want)
			}
		})
	}
}
