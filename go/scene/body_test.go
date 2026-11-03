package scene

import (
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
)

// bodyMeasure is a text measurer that is the same for every font: 10 points a rune, 8.3/9.1 metrics.
type bodyMeasure struct{}

func (bodyMeasure) MeasureText(s string, f gfx.Font) gfx.TextMetrics {
	return gfx.TextMetrics{Width: 10 * float64(len([]rune(s))), Ascent: 8.3, Descent: 9.1}
}

func newTestBody() *bodyText {
	return &bodyText{m: bodyMeasure{}, font: gfx.Font{Name: "Helvetica, Arial, sans-serif", Size: 16}, mono: gfx.Font{Size: 16, Family: bodyMonoFace},
		metrics: map[gfx.Font]gfx.TextMetrics{}, widths: map[bodyWidthKey]float64{}, gap: bodyBlockGap}
}

func TestBodyBlocks(t *testing.T) {
	b := newTestBody()
	b.parse("# Head\n\npara one\nline two\n\n- a **b**\n- c\n\n```\nx := 1\n```")
	want := []struct {
		kind byte
		text string
		item int
	}{
		{bodyHeading, "Head", 0},
		{bodyParagraph, "para one line two", 0},
		{bodyList, "a b", 0},
		{bodyList, "c", 1},
		{bodyCode, "x := 1", 0},
	}
	if len(b.blocks) != 4 {
		t.Fatalf("%d blocks, want 4: %+v", len(b.blocks), b.blocks)
	}
	got := []string{runsText(b.blocks[0].runs), runsText(b.blocks[1].runs), runsText(b.blocks[2].items[0]), runsText(b.blocks[2].items[1])}
	if got[0] != want[0].text || got[1] != want[1].text || got[2] != want[2].text || got[3] != want[3].text {
		t.Errorf("texts %q, want %q", got, []string{want[0].text, want[1].text, want[2].text, want[3].text})
	}
	if b.blocks[0].kind != bodyHeading || b.blocks[1].kind != bodyParagraph || b.blocks[2].kind != bodyList || b.blocks[3].kind != bodyCode {
		t.Errorf("kinds %c %c %c %c", b.blocks[0].kind, b.blocks[1].kind, b.blocks[2].kind, b.blocks[3].kind)
	}
	if code := b.blocks[3].code; len(code) != 1 || code[0] != want[4].text {
		t.Errorf("code %q, want %q", code, want[4].text)
	}
	if l := b.blocks[0]; l.kind != bodyHeading || l.level != 1 {
		t.Errorf("heading level %d, want 1", l.level)
	}
	if got := b.blocks[1].runs[0].font.Size; got != 16 {
		t.Errorf("paragraph size %v, want 16", got)
	}
	if got := b.blocks[0].runs[0].font.Size; got != 22 { // 16 + 6
		t.Errorf("heading size %v, want 22", got)
	}
	if f := b.blocks[0].runs[0].font; f.Weight != 700 || f.Name != "" || f.Family != "Helvetica" {
		t.Errorf("heading font %+v, want bold Helvetica", f)
	}
}

func TestBodyHeadingSizes(t *testing.T) {
	b := newTestBody()
	b.parse("# one\n\n## two\n\n### three")
	want := []float64{22, 20, 18} // body 16 + 6, +4, +2
	for i, size := range want {
		f := b.blocks[i].runs[0].font
		if f.Size != size || f.Weight != 700 || f.Name != "" {
			t.Errorf("h%d font %+v, want size %v bold without a face Name", i+1, f, size)
		}
	}
	lines := b.lines(1000)
	if lines[0].gap != 0 || lines[1].gap != bodyBlockGap+bodyHeadGap2 || lines[2].gap != bodyBlockGap {
		t.Errorf("heading gaps %v %v %v", lines[0].gap, lines[1].gap, lines[2].gap)
	}
}

func runsText(runs []bodyRun) string {
	s := ""
	for _, r := range runs {
		s += r.text
	}
	return s
}

func TestBodyInline(t *testing.T) {
	b := newTestBody()
	runs := b.inline("A **bold** *it* `code` [link](http://x)")
	want := []struct {
		text   string
		weight int
		italic bool
		code   bool
		link   string
	}{
		{"A ", 0, false, false, ""},
		{"bold", 700, false, false, ""},
		{" ", 0, false, false, ""},
		{"it", 0, true, false, ""},
		{" ", 0, false, false, ""},
		{"code", 0, false, true, ""},
		{" ", 0, false, false, ""},
		{"link", 0, false, false, "http://x"},
	}
	if len(runs) != len(want) {
		t.Fatalf("%d runs, want %d: %+v", len(runs), len(want), runs)
	}
	for i, w := range want {
		got := runs[i]
		if got.text != w.text || got.font.Weight != w.weight || got.font.Italic != w.italic || got.code != w.code || got.link != w.link {
			t.Errorf("run %d = %+v, want %+v", i, got, w)
		}
	}
	if got := runs[5].font.Family; got != bodyMonoFace {
		t.Errorf("code family %q, want %q", got, bodyMonoFace)
	}
	for i, weight := range map[int]int{1: 700, 3: 0} {
		f := runs[i].font
		if f.Name != "" || f.Family != "Helvetica" || f.Weight != weight {
			t.Errorf("run %d font %+v: want family Helvetica, weight %d", i, f, weight)
		}
	}
	if !runs[3].font.Italic {
		t.Errorf("italic run %+v", runs[3].font)
	}
}

func TestBodyInlineUnknownAsText(t *testing.T) {
	b := newTestBody()
	if got := runsText(b.inline("see @(pic)")); got != "see @(pic)" {
		t.Errorf("unknown markdown %q, want it as written", got)
	}
}

func TestBodyQuote(t *testing.T) {
	b := newTestBody()
	b.parse("> quoted text")
	if len(b.blocks) != 1 || b.blocks[0].kind != bodyQuote {
		t.Fatalf("blocks %+v, want one quote", b.blocks)
	}
	lines := b.lines(1000)
	if !lines[0].quote || lines[0].x != bodyQuoteIndent {
		t.Errorf("quote line %+v, want a bar at %v", lines[0], bodyQuoteIndent)
	}
	if got := runsText(lines[0].runs); got != "quoted text" {
		t.Errorf("quote text %q", got)
	}
}

func TestBodyListHangingIndent(t *testing.T) {
	b := newTestBody()
	b.parse("- word word word word")
	lines := b.lines(100)
	if len(lines) < 2 {
		t.Fatalf("%d lines, want a wrapped list item: %+v", len(lines), lines)
	}
	if lines[0].x != bodyListIndent || !strings.HasPrefix(runsText(lines[0].runs), "• ") {
		t.Errorf("marker at %v %q, want %v", lines[0].x, runsText(lines[0].runs), bodyListIndent)
	}
	want := bodyListIndent + b.widthOf("• ", b.font)
	for _, l := range lines[1:] {
		if l.x != want {
			t.Errorf("continuation x %v, want %v", l.x, want)
		}
	}
}

func TestBodyCodeBlockWrap(t *testing.T) {
	b := newTestBody()
	b.parse("```\nfunc main() {}\n```")
	lines := b.lines(100)
	if len(lines) < 2 {
		t.Fatalf("%d lines, want the code wrapped: %+v", len(lines), lines)
	}
	for _, l := range lines {
		if !l.code || l.x != bodyCodePad {
			t.Errorf("code line at %v code %v, want x %v", l.x, l.code, bodyCodePad)
		}
		if l.w > 100-2*bodyCodePad+1e-9 {
			t.Errorf("code line %q is %v wide, the inner width is %v", runsText(l.runs), l.w, 100-2*bodyCodePad)
		}
	}
}

func TestNormalizeInline(t *testing.T) {
	for in, want := range map[string]string{
		"**bold**":         "*bold*",
		"*it*":             "_it_",
		"[link](http://x)": "link",
		"`a *b* [c](d)`":   "`a *b* [c](d)`",
		"unclosed [x]":     "unclosed [x]",
		"2*3*4":            "2*3*4",
		"a**b**":           "a**b**",
	} {
		if got := normalizeInline(in); got != want {
			t.Errorf("normalizeInline(%q) = %q, want %q", in, got, want)
		}
	}
}
