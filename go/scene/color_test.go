package scene

import (
	"testing"

	"github.com/haiodo/gowt/gfx"
)

func TestParseColorTransparentHex(t *testing.T) {
	for _, s := range []string{"#00000000", "00000000", "transparent", "clear"} {
		c, ok := parseColor(s)
		if !ok || c.A != 0 {
			t.Errorf("parseColor(%q) = %v, %v; want transparent", s, c, ok)
		}
	}
}

func TestParseColorChannels(t *testing.T) {
	c, ok := parseColor("11, 234, 0")
	if !ok || c.R != 11.0/255 || c.G != 234.0/255 || c.B != 0 || c.A != 1 {
		t.Errorf("parseColor(11, 234, 0) = %v, %v; want rgb opaque", c, ok)
	}
	c, ok = parseColor("1,2,3,4")
	if !ok || c.A != 4.0/255 {
		t.Errorf("parseColor(1,2,3,4) = %v, %v; want alpha 4", c, ok)
	}
	for _, s := range []string{"1,2", "1,2,3,4,5", "1,x,3"} {
		if _, ok := parseColor(s); ok {
			t.Errorf("parseColor(%q) read a colour", s)
		}
	}
}

// TestParseColorRGBA: every #rrggbbaa reads the same, whatever the digits; the values an expression
// branch returns go through this parse, so a branch cannot fail on some colours and not on others.
func TestParseColorRGBA(t *testing.T) {
	for _, c := range []struct {
		hex string
		rgb uint32
		a   uint8
	}{
		{"#2e7cc726", 0x2e7cc7, 0x26},
		{"#2e7cc71f", 0x2e7cc7, 0x1f},
		{"#79797c26", 0x79797c, 0x26},
		{"#a5bdff0d", 0xa5bdff, 0x0d},
		{"#ffffff17", 0xffffff, 0x17},
	} {
		got, ok := parseColor(c.hex)
		want := gfx.Color{
			R: float64(c.rgb>>16&0xff) / 255,
			G: float64(c.rgb>>8&0xff) / 255,
			B: float64(c.rgb&0xff) / 255,
			A: float64(c.a) / 255,
		}
		if !ok || got != want {
			t.Errorf("parseColor(%q) = %v, %v; want %v", c.hex, got, ok, want)
		}
	}
}
