//go:build darwin || linux || windows

package app

import (
	"slices"
	"testing"
)

// TestSharedSectionRows pins the captions of the sections a classic item and a scene node both show.
// The rows are data and take the kind from nodeCfg, so this needs no window.
func TestSharedSectionRows(t *testing.T) {
	classic := nodeCfg{prefix: "insp"}
	scene := nodeCfg{prefix: "scene", words: true}
	sections := []struct {
		title string
		rows  func(nodeCfg) []nodeRow
		want  []string
	}{
		{"Layout", layoutRows, []string{"Position", "Size", "W", "H", "Padding"}},
		{"Fill", fillRows, []string{"Color"}},
		{"Text", textRows, []string{"Color", "Size"}},
		{"Border", borderRows, []string{"Color", "Width", "Style", "Radius"}},
		{"Shape", shapeRows, []string{"Display", "Layer"}},
	}
	for _, s := range sections {
		got, other := rowCaptions(s.rows(classic)), rowCaptions(s.rows(scene))
		if !slices.Equal(got, other) {
			t.Errorf("%s: classic shows %v, scene shows %v", s.title, got, other)
		}
		if !slices.Equal(got, s.want) {
			t.Errorf("%s: got %v, want %v", s.title, got, s.want)
		}
	}
}

func rowCaptions(rows []nodeRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.caption
	}
	return out
}
