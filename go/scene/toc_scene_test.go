package scene_test

import (
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/scene"
)

const linkMD = "# План\n\nВступление.\n\n## Цель\n\nТекст.\n"

func TestBodyLinkChecks(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"docs/plan.md":     linkMD,
		"docs/ok.md":       "# Год\n\n## Раздел\n\n[Цель](plan.md#Цель) [внешняя](https://example.com#нет)\n",
		"docs/sections.md": "# A\n\n## B\n\n[в C](#C)\n\n## C\n\nтекст\n",
		"docs/broken.md":   "# A\n\n## B\n\n[в D](#D)\n\n## C\n",
		"ok.tenn":          "element \"E\" { item \"Doc\" { body --from-file \"docs/ok.md\" } }\n",
		"sectionok.tenn":   "element \"E\" { item \"Doc\" { body --from-file \"docs/sections.md#B\" } }\n",
		"sectionbad.tenn":  "element \"E\" { item \"Doc\" { body --from-file \"docs/broken.md#B\" } }\n",
		"section.tenn":     "element \"E\" { item \"Doc\" { body %{ [Цель](docs/plan.md#Нет) } } }\n",
		"file.tenn":        "element \"E\" { item \"Doc\" { body %{ [x](nope.md#A) } } }\n",
		"same.tenn":        "element \"E\" { item \"Doc\" { body %{ # Цель\n\n[x](#Дальше) } } }\n",
		"outer.tenn":       "element \"E\" { item \"Doc\" { body %{ [x](../plan.md#Цель) } } }\n",
	})
	es, _, err := scene.Diagnose(root, "ok.tenn", nil)
	if err != nil || len(es) > 0 {
		t.Errorf("a working link reports %v %v", es, err)
	}
	es, _, err = scene.Diagnose(root, "sectionok.tenn", nil)
	if err != nil || len(es) > 0 {
		t.Errorf("a fragment of another section of the same file reports %v %v", es, err)
	}
	for name, want := range map[string][]string{
		"sectionbad.tenn": {"sectionbad.tenn:1:", `no section "D" in docs/broken.md`, "headings: A, B, C"},
		"section.tenn":    {"section.tenn:1:", `no section "Нет" in docs/plan.md`, "headings: План, Цель"},
		"file.tenn":       {"file.tenn:1:", "no such file nope.md"},
		"same.tenn":       {"same.tenn:1:", `no section "Дальше" in same.tenn`, "headings: Цель"},
		"outer.tenn":      {"outer.tenn:1:", "leaves the workspace root"},
	} {
		es, _, err := scene.Diagnose(root, name, nil)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := es.Error(); !es.Fatal() || !strings.Contains(got, want[0]) {
			t.Fatalf("%s: no error at %q: %s", name, want[0], got)
		}
		for _, w := range want[1:] {
			if got := es.Error(); !strings.Contains(got, w) {
				t.Errorf("%s: no %q in: %s", name, w, got)
			}
		}
	}
}

func TestBodyTOCScene(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"docs/plan.md": linkMD,
		"a.tenn":       "element \"E\" { item \"Doc\" { width 400; body --from-file \"docs/plan.md\"; toc 2 } }\n",
		"b.tenn":       "element \"E\" { item \"Doc\" { width 400; body --from-file \"docs/plan.md\"; toc 1 } }\n",
	})
	tree, err := scene.Load(root, "a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	_, svg := buildFile(t, tree, "E")
	// The toc comes first and is 2 points below the body; the heading itself keeps its own size.
	if got := sizeOf(t, svg, "План"); got != "14" {
		t.Errorf("first План is %s points, want the toc at 14", got)
	}
	if i, j := strings.Index(svg, ">План<"), strings.Index(svg, ">Цель<"); i < 0 || j < 0 || i > j {
		t.Errorf("the toc is not before the body: План at %d, Цель at %d", i, j)
	}
	// Depth 1 leaves Цель out of the toc: it is drawn once, as the body heading.
	tree, err = scene.Load(root, "b.tenn")
	if err != nil {
		t.Fatal(err)
	}
	_, svg = buildFile(t, tree, "E")
	if n := strings.Count(svg, ">Цель<"); n != 1 {
		t.Errorf("Цель drawn %d times, want once with toc 1", n)
	}
}
