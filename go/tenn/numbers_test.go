package tenn

import (
	"math"
	"testing"
)

// Expected strings are real Swift String(Double) output.
func TestSwiftDouble(t *testing.T) {
	tests := []struct {
		v    float64
		want string
	}{
		{0, "0.0"}, {math.Copysign(0, -1), "-0.0"}, {1, "1.0"}, {-3, "-3.0"}, {100, "100.0"}, {0.5, "0.5"},
		{sumTenths(), "0.30000000000000004"}, {123456789.123, "123456789.123"},
		{1e15, "1000000000000000.0"}, {8e15, "8000000000000000.0"},
		{9007199254740991, "9007199254740991.0"}, {9007199254740992, "9007199254740992.0"},
		{9007199254740994, "9.007199254740994e+15"}, {9.5e15, "9.5e+15"}, {1e16, "1e+16"}, {1.5e16, "1.5e+16"},
		{1.2345678901234568e17, "1.2345678901234568e+17"}, {1e21, "1e+21"}, {1e100, "1e+100"},
		{1.7976931348623157e308, "1.7976931348623157e+308"},
		{1e-4, "0.0001"}, {0.00099, "0.00099"}, {0.00010000000000000002, "0.00010000000000000002"},
		{9.999999999999999e-5, "9.999999999999999e-05"}, {1e-5, "1e-05"}, {3e-5, "3e-05"}, {1.5e-7, "1.5e-07"},
		{-1.5e-7, "-1.5e-07"}, {5e-324, "5e-324"}, {-2.5e-10, "-2.5e-10"},
		{math.NaN(), "nan"}, {math.Inf(1), "inf"}, {math.Inf(-1), "-inf"},
	}
	for _, tc := range tests {
		if got := SwiftDouble(tc.v); got != tc.want {
			t.Errorf("SwiftDouble(%v) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

func TestNewFloatNodeUsesSwiftDouble(t *testing.T) {
	if got := NewFloatNode(1e-5).Token.Literal; got != "1e-05" {
		t.Errorf("got %q", got)
	}
}

func TestGetValueTrimsSwiftWhitespace(t *testing.T) {
	block := func(v string) *Node { return NewBlockExpr(NewCommand("k", NewStrNode(v))) }
	if got := block("\u00855").GetValueInt("k", 0); got != 5 {
		t.Errorf("NEL trim: %d", got)
	}
	if got := block("​5​").GetValueInt("k", 0); got != 5 {
		t.Errorf("ZWSP trim: %d", got)
	}
	if got := block("\xef\xbb\xbf5").GetValueInt("k", 0); got != 0 {
		t.Errorf("BOM must not be trimmed: %d", got)
	}
	if got := block("​ x 　").GetValueString("k", ""); got != "x" {
		t.Errorf("string trim: %q", got)
	}
	if got := block("\xef\xbb\xbfx").GetValueString("k", ""); got != "\xef\xbb\xbfx" {
		t.Errorf("BOM must not be trimmed: %q", got)
	}
}

// Go folds 0.1 + 0.2 exactly at compile time; the runtime float64 sum is what the TS case checks.
func sumTenths() float64 {
	a, b := 0.1, 0.2
	return a + b
}
