package scene

import (
	"strings"
	"testing"
)

func TestBodyAnchors(t *testing.T) {
	b := newTestBody()
	b.parse("# Цель\n\n## Раздел 2: Итоги!\n\n## Цель\n\n### цель")
	var got []string
	for _, blk := range b.blocks {
		if blk.kind == bodyHeading {
			got = append(got, blk.anchor)
		}
	}
	want := []string{"цель", "раздел-2-итоги", "цель-2", "цель-3"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("anchors %q, want %q", got, want)
	}
	for in, want := range map[string]string{
		"  Пробелы   и   тире  ": "пробелы-и-тире",
		"Ёлка, ёж":               "ёлка-ёж",
		"**Жирный** `код`":       "жирный-код",
		"—":                      "",
	} {
		if got := anchorOf(in); got != want {
			t.Errorf("anchorOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBodyTOC(t *testing.T) {
	const text = "# План\n\nтекст\n\n## Цель\n\n## Цель\n\n### Дальше"
	b := newTestBody()
	b.parse(text)
	b.toc(2)
	if len(b.blocks) == 0 || b.blocks[0].kind != bodyTOC {
		t.Fatalf("blocks %+v, want the toc first", b.blocks)
	}
	items := b.blocks[0].toc
	wantLevels, wantText := []int{1, 2, 2}, []string{"План", "Цель", "Цель"}
	if len(items) != len(wantLevels) {
		t.Fatalf("%d toc items, want %d: %+v", len(items), len(wantLevels), items)
	}
	for i, it := range items {
		if it.level != wantLevels[i] || runsString(it.runs) != wantText[i] {
			t.Errorf("toc item %d = %d %q, want %d %q", i, it.level, runsString(it.runs), wantLevels[i], wantText[i])
		}
	}
	lines := b.lines(1000)
	wantX := []float64{0, bodyListIndent, bodyListIndent}
	for i, x := range wantX {
		if lines[i].x != x {
			t.Errorf("toc line %d at %v, want %v", i, lines[i].x, x)
		}
	}
	if got := lines[1].runs[0].font.Size; got != 14 { // body 16 - 2
		t.Errorf("toc font %v, want 14", got)
	}
	plain := newTestBody()
	plain.parse(text)
	if b.height(1000) <= plain.height(1000) {
		t.Errorf("with the toc the body is %v high, without %v: it adds nothing", b.height(1000), plain.height(1000))
	}
}

func TestBodyLinks(t *testing.T) {
	text := "[a](one.md#A) `[b](two.md#B)`\n\n```\n[c](three.md#C)\n```\n\n![pic](four.md#F) [d](https://x#D) [e](#E)"
	got := bodyLinks(text)
	want := []string{"one.md#A", "https://x#D", "#E"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("links %q, want %q", got, want)
	}
}
