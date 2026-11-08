package scene

import "testing"

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
