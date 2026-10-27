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
