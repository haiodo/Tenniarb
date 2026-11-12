package editor

import (
	"math"
	"strings"

	"github.com/haiodo/tenn/go"
)

// prepareBodyText is the text of a body as the editor shows it: escaped newlines expanded, tabs as blanks,
// the empty first and last line dropped, and the common indent of the lines taken away. go/render has the
// same function for drawing; the two become one when the old engine goes.
func prepareBodyText(textValue string) string {
	content := strings.ReplaceAll(strings.ReplaceAll(textValue, `\n`, "\n"), "\t", "    ")
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && isBlankLine(lines[0]) {
		lines = lines[1:]
	}
	if len(lines) > 0 && isBlankLine(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}
	least := math.MaxInt
	for _, l := range lines {
		if !isBlankLine(l) {
			least = min(least, leadingBlanks(l))
		}
	}
	out := make([]string, len(lines))
	for i, body := range lines {
		if isBlankLine(body) {
			out[i] = body + " "
			continue
		}
		r := []rune(body)
		out[i] = string(r[min(least, len(r)):])
	}
	return strings.Join(out, "\n")
}

// isBlankLine reports whether a line holds nothing but blanks.
func isBlankLine(s string) bool { return strings.TrimSpace(s) == "" }

// leadingBlanks is how many blanks a line starts with.
func leadingBlanks(s string) int {
	n := 0
	for _, c := range s {
		if c != ' ' {
			break
		}
		n++
	}
	return n
}

// nodeText is the text a node of a property holds: "" for no node, and the literal it carries.
func nodeText(n *tenn.Node) (string, bool) {
	if n == nil {
		return "", false
	}
	return n.GetIdentText()
}
