package scene

import (
	"strings"
	"unicode"
)

// NormalizeName is convertNameJS of web/packages/core/src/execution.ts: a rune that is not a letter,
// mark or number becomes "_". An empty name stays empty.
func NormalizeName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// splitPath splits a dotted path into normalized segments; ok is false for an empty segment.
func splitPath(path string) (segs []string, ok bool) {
	for _, s := range strings.Split(path, ".") {
		n := NormalizeName(s)
		if s == "" {
			return nil, false
		}
		segs = append(segs, n)
	}
	return segs, true
}
