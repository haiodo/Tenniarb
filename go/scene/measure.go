package scene

import (
	"math"
	"strings"

	"github.com/haiodo/gowt/gfx"
)

// MeasureText is the size of a plain text drawn at fontSize: the widest of its lines across, the sum of
// their line boxes down, both rounded up. The classic evaluator asks for it (`utils.textSize`), and it is
// the measurement of this engine, not of the one that drew before.
func MeasureText(m gfx.TextMeasurer, text string, fontSize float64) (w, h float64) {
	font := gfx.Font{Size: fontSize}
	metrics := m.MeasureText("Hg", font)
	for _, line := range strings.Split(strings.ReplaceAll(text, `\n`, "\n"), "\n") {
		if line == "" {
			h += metrics.Ascent + metrics.Descent
			continue
		}
		w = math.Max(w, m.MeasureText(line, font).Width)
		h += metrics.Ascent + metrics.Descent
	}
	return math.Ceil(w), math.Ceil(h)
}
