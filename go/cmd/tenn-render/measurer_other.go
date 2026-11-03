//go:build !darwin

package main

import "github.com/haiodo/gowt/gfx"

// textMeasurer is nil elsewhere: the layout estimates text, as gfx does without a backend.
func textMeasurer() gfx.TextMeasurer { return nil }
