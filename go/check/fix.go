package check

import (
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/haiodo/tenn/go"
)

// edit is one replacement in the text: start is inclusive, end exclusive, both are UTF-8 bytes of the
// source, as a tenn token describes them.
type edit struct {
	start, end int
	repl       string
}

// applyEdits writes the edits into the text back to front, so a replacement does not move the range of
// the one before it.
func applyEdits(text string, edits []edit) string {
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	for _, e := range edits {
		if e.start < 0 || e.end > len(text) || e.start >= e.end {
			continue
		}
		text = text[:e.start] + e.repl + text[e.end:]
	}
	return text
}

// quoteTitles rewrites a title written with spaces and no quotes, `title Gap and padding`, as one quoted
// text, `title "Gap and padding"`: only the first word would be drawn, and the file says so with an error
// a reader has to guess the fix for. It answers the text and how many titles it changed.
func quoteTitles(text string, root *tenn.Node) (string, int) {
	edits := titleEdits(root)
	if len(edits) == 0 {
		return text, 0
	}
	return applyEdits(text, edits), len(edits)
}

// titleEdits is the ranges of every title that needs quoting: the arguments of the command, from the
// first word after its name to its last token.
func titleEdits(root *tenn.Node) []edit {
	var edits []edit
	var walk func(n *tenn.Node)
	walk = func(n *tenn.Node) {
		if n.Kind == tenn.KindCommand && n.Token != nil && strings.EqualFold(n.Token.Literal, "title") {
			if repl, ok := quotedTitle(n); ok {
				// Children[0] is the name of the command itself, so the arguments start at one.
				edits = append(edits, edit{n.Children[1].Token.Pos, n.End, repl})
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	return edits
}

// roundPos rewrites a pos value longer than two decimals as its rounding to two, `pos -666.55078125
// 142.05078125` as `pos -666.55 142.05`: the digits of a value converted from a `.pen` are noise a reader
// cannot use, and the layout only needs a point. It answers the text and how many values it changed.
func roundPos(text string, root *tenn.Node) (string, int) {
	edits := posEdits(root)
	if len(edits) == 0 {
		return text, 0
	}
	return applyEdits(text, edits), len(edits)
}

// posEdits is the range of every number of a pos command that has more than two decimals.
func posEdits(root *tenn.Node) []edit {
	var edits []edit
	var walk func(n *tenn.Node)
	walk = func(n *tenn.Node) {
		if n.Kind == tenn.KindCommand && n.Token != nil && n.Token.Literal == "pos" {
			for _, c := range n.Children[1:] {
				if repl, ok := roundedNumber(c); ok {
					edits = append(edits, edit{c.Token.Pos, c.Token.Pos + c.Token.Size, repl})
				}
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	return edits
}

// roundedNumber is a pos argument written after rounding it to two decimals, false when there is nothing
// to round: an expression, an integer, or a value that is already two decimals short. The number is
// printed the way the format prints one, so `12.50` reads `12.5` and `-183.0004` reads `-183.0`.
func roundedNumber(n *tenn.Node) (string, bool) {
	if n.Token == nil || n.Kind != tenn.KindFloatLit {
		return "", false
	}
	lit := n.Token.Literal
	dot := strings.IndexByte(lit, '.')
	if dot < 0 || len(lit)-dot-1 <= 2 {
		return "", false
	}
	v, err := strconv.ParseFloat(lit, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return "", false
	}
	out := tenn.SwiftDouble(math.Round(v*100) / 100)
	// A number the lexer would not read back as a number would break the file it was written into.
	if typ := tenn.DetectSymbolType(out); typ != tenn.TokFloatLit && typ != tenn.TokIntLit {
		return "", false
	}
	return out, true
}

// quotedTitle is the one quoted argument for a title, false when the title is not the broken shape: one
// value already, or an argument that is not a bare word (a body, a block, an expression of its own).
func quotedTitle(n *tenn.Node) (string, bool) {
	args := n.Children[1:]
	if len(args) < 2 {
		return "", false
	}
	words := make([]string, 0, len(args))
	for _, c := range args {
		if c.Kind != tenn.KindIdent || c.Token == nil {
			return "", false
		}
		words = append(words, c.Token.Literal)
	}
	return quoteText(strings.Join(words, " ")), true
}

// quoteText is text as a tenn string: in quotes, with the quote and the backslash of the text escaped.
func quoteText(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"', '\\':
			b.WriteByte('\\')
		}
		b.WriteByte(s[i])
	}
	b.WriteByte('"')
	return b.String()
}

// FixTitles is the text with every title of one unquoted value quoted and every pos value longer than
// two decimals rounded to two, and how many places it changed. A text that does not parse comes back as
// it was: the check says what is wrong with it, and a guess would only make it worse. The window calls it
// for the button that fixes what the banner reports.
func FixTitles(text string) (string, int) {
	// The parser is given the text without its byte order mark, so token positions are offsets of the
	// body and an edit lands where it should; the mark is put back in front of the fixed text.
	bom, body := "", text
	if strings.HasPrefix(body, "\xef\xbb\xbf") {
		bom, body = body[:3], body[3:]
	}
	root, parser := tenn.Parse(body)
	if parser.Errors.HasErrors() || root == nil {
		return text, 0
	}
	edits := append(titleEdits(root), posEdits(root)...)
	if len(edits) == 0 {
		return text, 0
	}
	return bom + applyEdits(body, edits), len(edits)
}

// Fixable reports whether a diagnostic of the check is one the fixer repairs: a title of one unquoted
// value. The window shows its Fix button for a message that names one.
func Fixable(diagnostic string) bool {
	return strings.Contains(diagnostic, "title takes one value")
}

// fixFile applies FixTitles to the file at abs, and answers how many places it rewrote. A file that does
// not read is left alone: the diagnostics of the check say what is wrong with it.
func fixFile(abs string) (int, error) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return 0, err
	}
	fixed, n := FixTitles(string(data))
	if n == 0 {
		return 0, nil
	}
	return n, os.WriteFile(abs, []byte(fixed), 0o644)
}
