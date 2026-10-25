package editor

import (
	"strings"

	"github.com/haiodo/tenniarb/go/tenn"
)

// The text-only part of the properties panel: token colors and parse diagnostics (Swift
// TextPropertiesDelegate.highlight). Offsets are bytes of the text.

// HighlightClass is the color class of a token.
type HighlightClass string

const (
	HighlightSymbol     HighlightClass = "symbol"
	HighlightString     HighlightClass = "string"
	HighlightNumber     HighlightClass = "number"
	HighlightExpression HighlightClass = "expression"
)

// HighlightRange is a span of the text, From inclusive and To exclusive.
type HighlightRange struct {
	From, To int
	Class    HighlightClass
}

// Colors are the Swift TennColors as #rrggbb, for a light and a dark background.
var Colors = map[string]map[HighlightClass]string{
	"light": {HighlightSymbol: "#815f03", HighlightString: "#1c00cf", HighlightNumber: "#1c00cf", HighlightExpression: "#646485"},
	"dark":  {HighlightSymbol: "#75b492", HighlightString: "#fc6a5d", HighlightNumber: "#9686f5", HighlightExpression: "#c67c48"},
}

// HighlightRanges: the lexer positions cover the content between the delimiters; the ranges widen
// them as the Swift highlighter does.
func HighlightRanges(text string) []HighlightRange {
	var out []HighlightRange
	add := func(from, size int, class HighlightClass) {
		f := max(from, 0)
		to := min(from+size, len(text))
		if to > f {
			out = append(out, HighlightRange{f, to, class})
		}
	}
	lexer := tenn.NewLexer(text)
	for t := lexer.GetToken(); t != nil && t.Type != tenn.TokEOF; t = lexer.GetToken() {
		if t.Size == 0 {
			continue
		}
		switch t.Type {
		case tenn.TokSymbol:
			add(t.Pos, t.Size, HighlightSymbol)
		case tenn.TokStringLit:
			add(t.Pos-1, t.Size+2, HighlightString)
		case tenn.TokMarkdownLit:
			add(t.Pos-2, t.Size+3, HighlightString)
		case tenn.TokExpression, tenn.TokExpressionBlock:
			add(t.Pos-2, t.Size+3, HighlightExpression) // `$(` / `${` before, `)` / `}` after
		case tenn.TokIntLit, tenn.TokFloatLit:
			add(t.Pos, t.Size, HighlightNumber)
		}
	}
	return out
}

// TextDiagnostic is a parse error with the span of text to mark.
type TextDiagnostic struct {
	From, To int
	Message  string
}

// ParseDiagnostics: parse errors carry only line and column of the token end, so the whole
// (trimmed) line is marked.
func ParseDiagnostics(text string) []TextDiagnostic {
	_, parser := tenn.Parse(text)
	lines := strings.Split(text, "\n")
	var out []TextDiagnostic
	for _, e := range parser.Errors.Errors {
		line := min(e.Line, len(lines)-1)
		from := 0
		for _, l := range lines[:line] {
			from += len(l) + 1
		}
		raw := lines[line]
		from += len(raw) - len(strings.TrimLeft(raw, " \t\r\n"))
		to := min(max(from+len(strings.TrimSpace(raw)), from+1), len(text))
		out = append(out, TextDiagnostic{from, to, e.Message})
	}
	return out
}
