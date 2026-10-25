package markdown

import (
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/haiodo/tenniarb/go/tenn"
)

// CalcTitleFontSize strips the leading "#", spaces and tabs and returns the text, font size and heading level.
func CalcTitleFontSize(text string, fontSize float64) (string, float64, int) {
	hlevel := 0
	calch := true
	var result strings.Builder
	for _, c := range tenn.Graphemes(text) {
		if calch {
			if c == "#" {
				hlevel++
				continue
			} else if c == " " || c == "\t" {
				continue
			}
			calch = false
		}
		result.WriteString(c)
	}
	return result.String(), fontSize + 5 - float64(hlevel*2), hlevel
}

var (
	infRe = regexp.MustCompile(`^(?i:inf|infinity)$`)
	nanRe = regexp.MustCompile(`^(?i:nan(\(\w*\))?)$`)
	decRe = regexp.MustCompile(`^(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)
	hexRe = regexp.MustCompile(`^0[xX]([0-9a-fA-F]*)\.?([0-9a-fA-F]*)(?:[pP]([+-]?\d+))?$`)
)

// parseDouble is Swift Double(String): strtod syntax, nothing else (no whitespace, no "_"). NaN payloads and sign are not kept.
func parseDouble(s string) (float64, bool) {
	sign := 1.0
	if strings.HasPrefix(s, "-") {
		sign = -1
	}
	body := s
	if strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") {
		body = s[1:]
	}
	if infRe.MatchString(body) {
		return sign * math.Inf(1), true
	}
	if nanRe.MatchString(body) {
		return math.NaN(), true
	}
	if decRe.MatchString(body) {
		v, _ := strconv.ParseFloat(body, 64) // range error still returns +-Inf or 0, like JS Number
		return sign * v, true
	}
	if m := hexRe.FindStringSubmatch(body); m != nil && m[1]+m[2] != "" {
		mant, _ := new(big.Int).SetString(m[1]+m[2], 16)
		f, _ := new(big.Float).SetInt(mant).Float64()
		exp := 0.0
		if m[3] != "" {
			exp, _ = strconv.ParseFloat(m[3], 64)
		}
		return sign * f * math.Pow(2, exp-float64(4*len(m[2]))), true
	}
	return 0, false
}

// swiftDouble is Swift "\(Double)": shortest round-trip digits, always with a fraction, exponent form outside 1e-4..<1e16.
func swiftDouble(n float64) string {
	if math.IsNaN(n) {
		return "nan"
	}
	if math.IsInf(n, 0) {
		if n < 0 {
			return "-inf"
		}
		return "inf"
	}
	if n == 0 {
		if math.Signbit(n) {
			return "-0.0"
		}
		return "0.0"
	}
	mant, expStr, _ := strings.Cut(strconv.FormatFloat(math.Abs(n), 'e', -1, 64), "e")
	e, _ := strconv.Atoi(expStr)
	sign := ""
	if n < 0 {
		sign = "-"
	}
	digits := strings.Replace(mant, ".", "", 1)
	if e < -4 || e >= 16 {
		esign := "+"
		if e < 0 {
			esign, e = "-", -e
		}
		return sign + mant + "e" + esign + fmt.Sprintf("%02d", e)
	}
	if e < 0 {
		return sign + "0." + strings.Repeat("0", -e-1) + digits
	}
	if len(digits) < e+1 {
		digits += strings.Repeat("0", e+1-len(digits))
	}
	frac := digits[e+1:]
	if frac == "" {
		frac = "0"
	}
	return sign + digits[:e+1] + "." + frac
}

// Swift String.hasPrefix/hasSuffix/contains compare Characters, and "\r\n" is a single Character.
func hasNewline(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' && (i == 0 || s[i-1] != '\r') {
			return true
		}
	}
	return false
}

func endsWithNewline(s string) bool {
	n := len(s)
	return n > 0 && s[n-1] == '\n' && (n == 1 || s[n-2] != '\r')
}

// escape does not touch a char followed by a combining mark: Foundation's replacingOccurrences
// does not match there (GCB Extend, not ZWJ).
func escape(s string) string {
	if !strings.ContainsAny(s, `&<>"'`) {
		return s
	}
	var sb strings.Builder
	for i, c := range s {
		var rep string
		switch c {
		case '&':
			rep = "&amp;"
		case '<':
			rep = "&lt;"
		case '>':
			rep = "&gt;"
		case '"':
			rep = "&quot;"
		case '\'':
			rep = "&#39;"
		}
		if rep != "" {
			if next, w := utf8.DecodeRuneInString(s[i+1:]); w == 0 || !isExtend(next) {
				sb.WriteString(rep)
				continue
			}
		}
		sb.WriteRune(c)
	}
	return sb.String()
}

func isExtend(r rune) bool {
	return unicode.In(r, unicode.Mn, unicode.Me, unicode.Other_Grapheme_Extend) || r >= 0x1f3fb && r <= 0x1f3ff
}

func isBlank(s string) bool { return trimmed(s) == "" }

// ToHTML renders tokens like the Swift HTMLPrinter. textColor is not used, as in the TS port.
func ToHTML(tokens []Token, originalFont float64, textColor string, images ImagePathResolver) string {
	// The wrapper div no longer has a default font-size style.
	var result strings.Builder
	result.WriteString("<div>")

	// State variables for the *current* effective style.
	currentColor := ""
	currentFontSize := originalFont

	// State variables to track the *last* style applied to an element,
	// to minimize redundant style attributes.
	lastAppliedColor := ""
	lastAppliedFontSize := originalFont

	prevMultiCode := false
	lastLiteral := ""
	lastToken := TokEOF
	inList := false

	codeStyle := `style="background-color: ` + ColorToHex(ParseColor("grey-200", 1.0)) + `; color: black;"`

	for _, t := range tokens {
		// Escape HTML special characters in text
		literal := escape(t.Literal)

		if prevMultiCode {
			prevMultiCode = false
			literal = strings.TrimPrefix(literal, "\n")
			result.WriteString("<pre " + codeStyle + ">\n</pre>")
		}

		if t.Type != TokEOF {
			lastToken = t.Type
		}

		switch t.Type {
		case TokText:
			var styleAttributes []string

			// Only include font-size if it changed from the last applied size
			if currentFontSize != lastAppliedFontSize {
				styleAttributes = append(styleAttributes, "font-size: "+swiftDouble(currentFontSize)+"px")
				lastAppliedFontSize = currentFontSize
			}

			// Only include color if it changed from the last applied color
			if currentColor != lastAppliedColor {
				styleAttributes = append(styleAttributes, currentColor)
				lastAppliedColor = currentColor
			}

			styleAttr := ""
			if len(styleAttributes) > 0 {
				styleAttr = ` style="` + strings.Join(styleAttributes, "; ") + `"`
			}
			result.WriteString("<span" + styleAttr + ">" + literal + "</span>")
		case TokBold:
			result.WriteString("<strong>" + literal + "</strong>")
		case TokBullet:
			if !inList {
				result.WriteString(`<ul style="margin-left: ` + strconv.Itoa(5*len(tenn.Graphemes(literal))) + "px;\">\n")
				inList = true
			}
			result.WriteString("<li>" + literal + "</li>\n")
		case TokImage:
			if resolved, ok := images.ResolveImagePath(t.Literal); ok {
				result.WriteString(`<img src="` + t.Literal + `" width="` + swiftDouble(resolved.Rect.Width) +
					`" height="` + swiftDouble(resolved.Rect.Height) + `" alt="` + t.Literal + `" />`)
			}
		case TokItalic:
			result.WriteString("<em>" + literal + "</em>")
		case TokUnderline:
			result.WriteString("<u>" + literal + "</u>")
		case TokScratch:
			result.WriteString("<del>" + literal + "</del>")
		case TokTitle:
			title, titleSize, hlevel := CalcTitleFontSize(literal, originalFont)
			headerTag := "h" + strconv.Itoa(min(max(hlevel, 1), 6)) // Ensure h1-h6
			result.WriteString("<" + headerTag + ` style="font-size: ` + swiftDouble(titleSize) + "px;\">" + title + "</" + headerTag + ">\n")
		case TokColor:
			if splitPos := firstIndexOf(literal, "|"); splitPos >= 0 {
				colorHex := ColorToHex(ParseColor(literal[:splitPos], 1.0))
				result.WriteString(`<span style="color: ` + colorHex + ";\">" + literal[splitPos+1:] + "</span>")
			} else if isBlank(literal) {
				// Reset color to default (empty)
				currentColor = ""
			} else {
				// Set new global color
				currentColor = "color: " + ColorToHex(ParseColor(literal, 1.0)) + ";"
			}
		case TokFont:
			if splitPos := firstIndexOf(literal, "|"); splitPos >= 0 {
				if fontSize, ok := parseDouble(literal[:splitPos]); ok {
					// Inline font size, always apply the style
					result.WriteString(`<span style="font-size: ` + swiftDouble(fontSize) + "px;" + currentColor + `">` + literal[splitPos+1:] + "</span>")
				}
			} else if isBlank(literal) {
				// Reset font size to default
				currentFontSize = originalFont
			} else if fontSize, ok := parseDouble(literal); ok {
				// Set new global font size
				currentFontSize = fontSize
			}
		case TokCode:
			if hasNewline(literal) {
				if !strings.HasPrefix(literal, "\n") && !endsWithNewline(lastLiteral) {
					result.WriteString("<br>")
				}
				if !endsWithNewline(literal) {
					prevMultiCode = true
				}
				result.WriteString("<pre " + codeStyle + ">" + literal + "</pre>")
			} else {
				result.WriteString("<code " + codeStyle + ">" + literal + "</code>")
			}
		}
		lastLiteral = literal
	}

	if inList {
		result.WriteString("</ul>\n")
	}

	if lastToken == TokImage {
		result.WriteString("<br>")
	}

	result.WriteString("</div>")

	return result.String()
}
