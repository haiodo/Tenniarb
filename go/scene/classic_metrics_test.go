package scene_test

import "math"

// classicLine is the height of one line of text in a classic diagram: go/render stacks a line by the SF
// ascent and descent of render/text.go (1980 and 432 of 2048) and rounds it up to the next point.
func classicLine(size float64) float64 { return math.Floor(size*(1980.0+432.0)/2048) + 1 }

// classicBox is the height of a text leaf of a classic diagram whose text has n lines at size, with the
// 4 point inset go/render gives the text on every side.
func classicBox(size, n float64) float64 { return n*classicLine(size) + 2*4 }
