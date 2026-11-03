package scene_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
)

// svgTextY is the baseline y of the one text run with that text in the SVG, its group transform
// included: every node is written as a group of its own.
func svgTextY(t *testing.T, svg, text string) float64 {
	t.Helper()
	idx := strings.Index(svg, ">"+text+"</text>")
	if idx < 0 {
		t.Fatalf("no %q text in:\n%s", text, svg)
	}
	head := svg[:idx]
	open := strings.LastIndex(head, "<text ")
	if open < 0 {
		t.Fatalf("no text tag before %q", text)
	}
	ym := regexp.MustCompile(`y="([-0-9.]+)"`).FindStringSubmatch(svg[open:idx])
	if ym == nil {
		t.Fatalf("no y in %q", svg[open:idx])
	}
	y, err := strconv.ParseFloat(ym[1], 64)
	if err != nil {
		t.Fatalf("%q: %v", ym[1], err)
	}
	re := regexp.MustCompile(`<g transform="matrix\(1 0 0 1 [-0-9.]+ ([-0-9.]+)\)">`)
	if gs := re.FindAllStringSubmatch(head, -1); len(gs) > 0 {
		ty, err := strconv.ParseFloat(gs[len(gs)-1][1], 64)
		if err != nil {
			t.Fatalf("%q: %v", gs[len(gs)-1][1], err)
		}
		y += ty
	}
	return y
}

// Body geometry with frac (10 points a rune, line box round(8.3+9.1) = 17): a title line, body
// lines and the 5 point gap between body blocks, no padding in a layout context.
func TestBodyHeight(t *testing.T) {
	s, _ := buildSrc(t, `element "E" {
    item "Col" {
        auto-layout column
        item "A" { title "T"; width 100; body "aaaa bbbb" }
        item "B" { title "T"; width 50; body "aaaa bbbb" }
        item "G" { title "T"; width 1000; body %{
            one

            two
        } }
        item "L" { title "T"; width 1000; body %{
            - a
            - b
        } }
    }
}`)
	for path, want := range map[string]float64{
		"Col.A": 17 + 17,          // one title line, one body line
		"Col.B": 17 + 2*17,        // "aaaa bbbb" does not fit 50: two body lines
		"Col.G": 17 + 17 + 5 + 17, // two paragraphs: one gap between them
		"Col.L": 17 + 2*17,        // two bullets: compact, no gap
	} {
		if got := boxH(s, path); got != want {
			t.Errorf("%s height %v, want %v", path, got, want)
		}
	}
}

func TestBodyGrowsWithText(t *testing.T) {
	s, _ := buildSrc(t, `element "E" {
    item "Col" {
        auto-layout column
        item "S" { title "T"; body "aaaa" }
        item "L" { title "T"; body "aaaaaa" }
    }
}`)
	want := map[string]float64{"Col.S": 40, "Col.L": 60} // 4 and 6 runes at 10 points, no padding
	for path, w := range want {
		var got float64 = -1
		for _, b := range s.Boxes() {
			if b.Path == path {
				got = b.W
			}
		}
		if got != w {
			t.Errorf("%s width %v, want %v", path, got, w)
		}
	}
}

// Body draws under the title, in markdown: a heading is larger, code is monospaced over its
// background, a link is its label, not the URL.
func TestBodySVG(t *testing.T) {
	_, svg := buildSrc(t, `element "E" {
    item "Doc" {
        width 300
        body %{
            # Head
            A **bold** and `+"`code`"+` [link](http://x).
        }
    }
}`)
	for _, part := range []string{
		">Doc<", ">Head<", ">bold<", ">code<", ">link<",
		`font-size="22"`,          // heading: body 16 + 6
		`font-family="Menlo"`,     // code
		`fill="rgb(238,238,238)"`, // code background
	} {
		if !strings.Contains(svg, part) {
			t.Errorf("missing %q in:\n%s", part, svg)
		}
	}
	// The bold run has its own weight in the SVG, not only in the font of the recorder.
	if m := regexp.MustCompile(`<text([^>]*)>bold</text>`).FindStringSubmatch(svg); m == nil || !strings.Contains(m[1], `font-weight="700"`) {
		t.Errorf("the bold run carries no weight:\n%s", svg)
	}
	if strings.Contains(svg, "http://x") {
		t.Errorf("the URL is drawn:\n%s", svg)
	}
}

// A body of an auto-layout item is a block after the children: below the title and inside the
// card, whose height includes it (frac: 17 a line, 20 padding, 12 gap).
func TestBodyFlowsBelowTitle(t *testing.T) {
	s, svg := buildSrc(t, `element "E" {
    item "Card" {
        auto-layout column
        width 420
        padding 20
        gap 12
        item "T" { title "Head" }
        body %{
            one
        }
    }
}`)
	if got, want := boxH(s, "Card"), 20.0+17+12+17+20; got != want {
		t.Errorf("card height %v, want %v", got, want)
	}
	if yBody, yTitle := svgTextY(t, svg, "one"), svgTextY(t, svg, "Head"); yBody <= yTitle {
		t.Errorf("body baseline %v, title baseline %v: body is not below", yBody, yTitle)
	}
}

// More body text wraps and the card grows by every line.
func TestBodyGrowsCard(t *testing.T) {
	s, _ := buildSrc(t, `element "E" {
    item "Card" {
        auto-layout column
        width 80
        padding 10
        gap 10
        item "T" { title "Head" }
        body "one two three four five"
    }
}`)
	// 5 body lines at 10 points a rune (inner width 60), title 1, gap 10, padding 20.
	if got, want := boxH(s, "Card"), 10.0+17+10+5*17+10; got != want {
		t.Errorf("card height %v, want %v", got, want)
	}
}

// The three forms of the classic body command load and build without errors.
func TestBodyForms(t *testing.T) {
	tree, err := scene.LoadSource(t.TempDir(), "a.tenn", `element "E" {
    item "A" { body "one line" }
    item "B" { body %{ two lines } }
    item "C" { body { text "custom"; font-size 14; font-family "Menlo"; text-color #112233; line-height 20; gap 3 } }
}`)
	if err != nil {
		t.Fatal(err)
	}
	var screen *scene.Screen
	sc := gfx.NewScene()
	sc.Update(func(tx *gfx.Tx) { screen, err = tree.Build(tx, nil, "E", scene.Options{Measurer: frac{}}) })
	if err != nil {
		t.Fatal(err)
	}
	if len(screen.Boxes()) != 4 {
		t.Errorf("boxes %v, want the element and three items", screen.Boxes())
	}
}

// A closing "%}" of the markdown literal leaves no percent in the text.
func TestBodyMarkdownLiteral(t *testing.T) {
	_, svg := buildSrc(t, `element "E" {
    item "D" {
        body %{ hi there %}
    }
}`)
	if !strings.Contains(svg, "hi there") || strings.Contains(svg, "%") {
		t.Errorf("body literal:\n%s", svg)
	}
}
