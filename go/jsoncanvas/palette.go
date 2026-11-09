package jsoncanvas

import (
	"strconv"
	"strings"

	"github.com/haiodo/tenniarb/go/markdown"
)

// preset is one of the six colours of JSON Canvas. The format names the colours by number only, so
// the hex is the Obsidian palette and the name is the nearest shade of the .tenn palette.
type preset struct {
	key, name, hex string
}

var presets = [...]preset{
	{"1", "red-500", "#fb464c"},
	{"2", "orange-500", "#e9973f"},
	{"3", "yellow-500", "#e0de71"},
	{"4", "green-500", "#44cf6e"},
	{"5", "cyan-500", "#53dfdd"},
	{"6", "purple-500", "#a882ff"},
}

func presetName(key string) (string, bool) {
	for _, p := range presets {
		if p.key == key {
			return p.name, true
		}
	}
	return "", false
}

// presetKey is the canvas colour nearest to a .tenn colour name or hex value.
func presetKey(color string) (string, bool) {
	r, g, b, ok := parseRGB(color)
	if !ok {
		return "", false
	}
	best, bestDist := "", -1.0
	for _, p := range presets {
		pr, pg, pb, _ := parseRGB(p.hex)
		d := (r-pr)*(r-pr) + (g-pg)*(g-pg) + (b-pb)*(b-pb)
		if bestDist < 0 || d < bestDist {
			best, bestDist = p.key, d
		}
	}
	return best, true
}

func parseRGB(color string) (r, g, b float64, ok bool) {
	s := strings.ToLower(strings.TrimSpace(color))
	if hex, found := markdown.ColorNames[s]; found {
		s = hex
	}
	s = strings.TrimPrefix(s, "#")
	if len(s) == 8 { // RGBA: JSON Canvas has no alpha
		s = s[:6]
	}
	if len(s) != 6 {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return float64(v >> 16 & 0xff), float64(v >> 8 & 0xff), float64(v & 0xff), true
}
