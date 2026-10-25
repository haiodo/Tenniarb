package markdown

import (
	"math"
	"reflect"
	"testing"
)

// HTMLPrinterTests.swift: a provider that always resolves to 100x50.
type mockImageProvider struct{}

func (mockImageProvider) ResolveImagePath(string) (ResolvedImage, bool) {
	return ResolvedImage{Rect: Rect{Width: 100, Height: 50}}, true
}

func generateHTML(markdown string) string {
	return ToHTML(GetTokens(markdown), 16.0, "black", mockImageProvider{})
}

func checkHTML(t *testing.T, markdown, want string) {
	t.Helper()
	if got := generateHTML(markdown); got != want {
		t.Errorf("%q:\n got  %s\n want %s", markdown, got, want)
	}
}

func TestPlainText(t *testing.T) {
	checkHTML(t, "Hello, world!", "<div><span>Hello, world!</span></div>")
}

// Expectations below were produced by the Swift HTMLPrinter.

func TestInlineStyles(t *testing.T) {
	checkHTML(t, "*b* _i_ ~s~ <u>", "<div><strong>b</strong><span> </span><em>i</em><span> </span><del>s</del><span> </span><u>u</u></div>")
	checkHTML(t, "!(red|w) &(20|v)", `<div><span style="color: #FF0000;">w</span><span> </span><span style="font-size: 20.0px;">v</span></div>`)
	checkHTML(t, "&(20) a &() b", `<div><span style="font-size: 20.0px"> a </span><span style="font-size: 16.0px"> b</span></div>`)
}

func TestTitlesAndBullets(t *testing.T) {
	checkHTML(t, "## T\n* a\n* b\n", `<div><h2 style="font-size: 17.0px;">T</h2>`+"\n"+`<ul style="margin-left: 5px;">`+"\n"+
		"<li>*</li>\n<span> a</span><span>\n</span><li>*</li>\n<span> b</span><span>\n</span></ul>\n</div>")
	text, size, level := CalcTitleFontSize("### \tx", 16)
	if text != "x" || size != 15 || level != 3 {
		t.Errorf("CalcTitleFontSize: %q %v %d", text, size, level)
	}
}

func TestEscaping(t *testing.T) {
	checkHTML(t, `a<b&"'`, "<div><span>a&lt;b&amp;&quot;&#39;</span></div>")
	// Foundation does not replace a char followed by a combining mark.
	checkHTML(t, "&\u0301", "<div><span>&\u0301</span></div>")
}

func TestCodeAndImages(t *testing.T) {
	checkHTML(t, "`x`", `<div><code style="background-color: gray-200; color: black;">x</code></div>`)
	checkHTML(t, "@(a)", `<div><img src="a" width="100.0" height="50.0" alt="a" /><br></div>`)
}

func TestMultilineCode(t *testing.T) {
	const style = `style="background-color: gray-200; color: black;"`
	checkHTML(t, "`a\nb`", "<div><br><pre "+style+">a\nb</pre><pre "+style+">\n</pre></div>")
	checkHTML(t, "`a\nb\n`", "<div><br><pre "+style+">a\nb\n</pre></div>")
}

func TestColors(t *testing.T) {
	if got, want := ParseColor("#ff8000", 0.5), (RGBA{1, 128.0 / 255, 0, 0.5}); got != want {
		t.Errorf("hex: %v, want %v", got, want)
	}
	if got, want := ParseColor("nosuch", 1), (RGBA{0, 0, 0, 1}); got != want {
		t.Errorf("unknown: %v, want %v", got, want)
	}
	if got := ColorToHex(ParseColor("red", 1)); got != "#FF0000" {
		t.Errorf("red: %s", got)
	}
	if got := ColorToHex(ParseColor("blue-500", 1)); got != "blue-500" {
		t.Errorf("blue-500: %s", got)
	}
	// Unknown names and black share #000000: the first table row with that hex wins.
	if got := ColorToHex(RGBA{}); got != "black" {
		t.Errorf("black: %s", got)
	}
	// Scanner.scanHexInt64 quirks: 0x prefix, stop at the first non-hex char, saturation.
	for in, want := range map[string]uint64{"#0xff": 0xff, "# ff": 0xff, "#12zz": 0x12, "#": 0, "#ffffffffffffffffffff": math.MaxUint64} {
		c := ParseColor(in, 1)
		got := uint64(math.Round(c.R*255))<<16 | uint64(math.Round(c.G*255))<<8 | uint64(math.Round(c.B*255))
		if got != want&0xffffff {
			t.Errorf("%q: %x, want %x", in, got, want&0xffffff)
		}
	}
}

func TestParseDouble(t *testing.T) {
	valid := map[string]float64{"14": 14, "20.5": 20.5, "1e5": 1e5, "0x10": 16, "+5": 5, ".5": 0.5, "5.": 5, "-3": -3, "007": 7, "0x1p3": 8, "1e400": math.Inf(1), "-inf": math.Inf(-1)}
	for in, want := range valid {
		if got, ok := parseDouble(in); !ok || got != want {
			t.Errorf("%q: %v %v, want %v", in, got, ok, want)
		}
	}
	if got, ok := parseDouble("nan"); !ok || !math.IsNaN(got) {
		t.Errorf("nan: %v %v", got, ok)
	}
	for _, in := range []string{"", "abc", "1_0", " 5", "1e", "0x", "0x.", "--1"} {
		if _, ok := parseDouble(in); ok {
			t.Errorf("%q parsed", in)
		}
	}
}

func TestSwiftDouble(t *testing.T) {
	negZero := math.Copysign(0, -1)
	cases := map[float64]string{
		0: "0.0", negZero: "-0.0", 16: "16.0", 20.5: "20.5", 0.0001: "0.0001", 0.00001: "1e-05",
		1e15: "1000000000000000.0", 1e16: "1e+16", 123456789012345678: "1.2345678901234568e+17",
		-3: "-3.0", math.Inf(1): "inf", math.Inf(-1): "-inf", math.NaN(): "nan", 1e100: "1e+100",
	}
	for in, want := range cases {
		if got := swiftDouble(in); got != want {
			t.Errorf("%v: %s, want %s", in, got, want)
		}
	}
}

func TestImageSizes(t *testing.T) {
	sized := func(path string) Rect {
		p := NewImageProvider(1)
		p.Resolve = func(string) *CachedImage { return &CachedImage{Size: Size{100, 50}} }
		r, ok := p.ResolveImagePath(path)
		if !ok {
			t.Fatalf("%q not resolved", path)
		}
		return r.Rect
	}
	for path, want := range map[string]Rect{
		"a":       {0, 0, 100, 50},
		"a|40x40": {0, 0, 40, 40},
		"a|40":    {0, 0, 40, 20},
		"a|x25":   {0, 0, 50, 25},
	} {
		if got := sized(path); got != want {
			t.Errorf("%q: %v, want %v", path, got, want)
		}
	}
	if got := GetMaxRect(200, 200, 100, 50); !reflect.DeepEqual(got, Rect{0, 0, 100, 50}) {
		t.Errorf("GetMaxRect: %v", got)
	}
	if _, ok := NewImageProvider(1).ResolveImagePath("a"); ok {
		t.Error("provider without Resolve resolved an image")
	}
}

func TestColorNamesOrder(t *testing.T) {
	if len(ColorNames) != len(colorNameTable) {
		t.Errorf("duplicate names: %d map entries, %d rows", len(ColorNames), len(colorNameTable))
	}
}
