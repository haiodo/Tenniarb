//go:build darwin

package main

import (
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/cg"
)

// textMeasurer measures with the face the viewer draws: the estimated widths of gfx put every run
// boundary of a markdown line in the wrong place (bold and code spans drift apart).
func textMeasurer() gfx.TextMeasurer { return cg.NewRenderer(cg.Options{}) }
