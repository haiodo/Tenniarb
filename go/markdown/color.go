package markdown

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// RGBA is CGColor components, 0...1.
type RGBA struct {
	R, G, B, A float64
}

func HexStringToUIColor(hexString string, alpha float64) RGBA {
	hexString = trimmed(hexString)

	if startsWithChar(hexString, "#") {
		hexString = hexString[1:]
	}
	// Scanner.scanHexInt64: skips leading whitespace, accepts an 0x prefix, stops at the first non-hex
	// char, saturates on overflow, leaves 0 when there are no digits.
	hexString = strings.TrimLeftFunc(hexString, isSpace)
	if len(hexString) >= 2 && hexString[0] == '0' && (hexString[1] == 'x' || hexString[1] == 'X') {
		hexString = hexString[2:]
	}
	end := 0
	for end < len(hexString) && isHexDigit(hexString[end]) {
		end++
	}
	color, _ := strconv.ParseUint(hexString[:end], 16, 64) // range error leaves MaxUint64
	return RGBA{
		R: float64((color>>16)&0xff) / 255.0,
		G: float64((color>>8)&0xff) / 255.0,
		B: float64(color&0xff) / 255.0,
		A: alpha,
	}
}

func isHexDigit(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}

func ParseColor(color string, alpha float64) RGBA {
	if startsWithChar(color, "#") {
		return HexStringToUIColor(color, alpha)
	}
	if hex, ok := ColorNames[color]; ok {
		return HexStringToUIColor(hex, alpha)
	}
	return RGBA{A: alpha}
}

// ColorToHex: Swift iterates the dictionary (random order) when several names share a hex; here the first one in table order wins.
func ColorToHex(color RGBA) string {
	hex := func(v float64) string {
		return fmt.Sprintf("%02X", int(math.Floor(float64(float32(v*255))+0.5)))
	}
	hexString := "#" + hex(color.R) + hex(color.G) + hex(color.B)

	for _, e := range colorNameTable {
		if e.hex == hexString {
			return e.name
		}
	}
	return hexString
}
