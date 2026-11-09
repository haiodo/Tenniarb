package scene_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
)

const bodyFileMD = `# План

Вступление.

## Цель

Документация в markdown.
Вторая строка.

- пункт

## Дальше

Мимо.
`

// buildFile loads root/file and writes element of it as SVG.
func buildFile(t *testing.T, tree *scene.Tree, element string) (*scene.Screen, string) {
	t.Helper()
	sc := gfx.NewScene()
	var screen *scene.Screen
	var err error
	sc.Update(func(tx *gfx.Tx) { screen, err = tree.Build(tx, nil, element, scene.Options{Measurer: frac{}}) })
	if err != nil {
		t.Fatal(err)
	}
	sc.Commit()
	var out bytes.Buffer
	if err := gfx.WriteSubtreeSVG(&out, sc, screen.Root, gfx.Rect{}, 1); err != nil {
		t.Fatal(err)
	}
	return screen, out.String()
}

// sizeOf is the font size of the text run with exactly that text in the SVG.
func sizeOf(t *testing.T, svg, text string) string {
	t.Helper()
	m := regexp.MustCompile(`font-size="([0-9.]+)"[^>]*>` + regexp.QuoteMeta(text) + `<`).FindStringSubmatch(svg)
	if m == nil {
		t.Fatalf("no run %q in:\n%s", text, svg)
	}
	return m[1]
}

func TestBodyFromFileSection(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"docs/plan.md": bodyFileMD,
		// the block of the body sets its view, the text comes from the file
		"a.tenn": "element \"E\" { item \"Doc\" { width 1000; body --from-file \"docs/plan.md#Цель\" { font-size 14 } } }\n",
	})
	tree, err := scene.Load(root, "a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	_, svg := buildFile(t, tree, "E")
	// A classic body breaks its lines as written: go/render draws one paragraph line per line.
	for _, want := range []string{"Цель", "Документация в markdown.", "Вторая строка.", "• пункт"} {
		if !strings.Contains(svg, want) {
			t.Errorf("no %q in:\n%s", want, svg)
		}
	}
	for _, gone := range []string{"Вступление", "Мимо", "План"} {
		if strings.Contains(svg, gone) {
			t.Errorf("%q is outside the section, but drawn:\n%s", gone, svg)
		}
	}
	// The heading keeps the size rule of a body written in the .tenn: h2 is the body plus 4 points.
	body, head := sizeOf(t, svg, "Документация в markdown."), sizeOf(t, svg, "Цель")
	if head != "18" || body != "14" {
		t.Errorf("heading %s, body %s, want 18 and 14", head, body)
	}
}

func TestBodyFromFileWhole(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"docs/plan.md": bodyFileMD,
		"a.tenn":       "element \"E\" { item \"Doc\" { width 400; body --from-file \"docs/plan.md\" } }\n",
	})
	tree, err := scene.Load(root, "a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	_, svg := buildFile(t, tree, "E")
	for _, want := range []string{"План", "Вступление.", "Цель", "Мимо."} {
		if !strings.Contains(svg, want) {
			t.Errorf("no %q in:\n%s", want, svg)
		}
	}
}

func TestBodyFromFileHeadingSpelling(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"docs/plan.md": "# One\n\ntext one\n\n##   Two   ##\n\ntext two\n\n### Three\n\ntext three\n",
		"a.tenn":       "element \"E\" { item \"Doc\" { width 400; body --from-file \"docs/plan.md#Two\" } }\n",
	})
	tree, err := scene.Load(root, "a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	_, svg := buildFile(t, tree, "E")
	for _, want := range []string{"Two", "text two", "Three", "text three"} {
		if !strings.Contains(svg, want) {
			t.Errorf("no %q in:\n%s", want, svg)
		}
	}
	if strings.Contains(svg, "text one") {
		t.Errorf("a heading above the section is drawn:\n%s", svg)
	}
}

// TestBodyFromFileMount: the path is relative to the file that writes the body, not to the workspace root.
func TestBodyFromFileMount(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"lib/page.tenn": "element Page { item \"Doc\" { width 1000; body --from-file \"plan.md\" } }\n",
		"lib/plan.md":   "# Lib\n\nИз монтированного файла.\n",
		"a.tenn":        "element \"E\" --from-file \"lib/page.tenn\"\n",
	})
	tree, err := scene.Load(root, "a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	if _, svg := buildFile(t, tree, "E.Page"); !strings.Contains(svg, "Из монтированного файла.") {
		t.Errorf("the file next to the mounted .tenn is not read:\n%s", svg)
	}
}

func TestBodyFromFileReread(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"docs/plan.md": bodyFileMD,
		"a.tenn":       "element \"E\" { item \"Doc\" { width 400; body --from-file \"docs/plan.md#Цель\" } }\n",
	})
	tree, err := scene.Load(root, "a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(bodyFileMD, "Документация в markdown.", "Правка после загрузки.", 1)
	if err := os.WriteFile(filepath.Join(root, "docs/plan.md"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, svg := buildFile(t, tree, "E"); !strings.Contains(svg, "Правка после загрузки.") {
		t.Errorf("the edited file is not read again:\n%s", svg)
	}
}

func TestBodyFromFileErrors(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"docs/plan.md": bodyFileMD,
		"section.tenn": "element \"E\" { item \"Doc\" { body --from-file \"docs/plan.md#Нет\" } }\n",
		"missing.tenn": "element \"E\" { item \"Doc\" { body --from-file \"nope.md\" } }\n",
		"outside.tenn": "element \"E\" { item \"Doc\" { body --from-file \"../plan.md\" } }\n",
		"nopath.tenn":  "element \"E\" { item \"Doc\" { body --from-file } }\n",
	})
	for name, want := range map[string][]string{
		"section.tenn": {"section.tenn:1:", `no heading "Нет"`, "headings: План, Цель, Дальше"},
		"missing.tenn": {"missing.tenn:1:", "nope.md", "no such file"},
		"outside.tenn": {"outside.tenn:1:", "leaves the workspace root"},
		"nopath.tenn":  {"nopath.tenn:1:", "needs a file"},
	} {
		es, _, err := scene.Diagnose(root, name, nil)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got := es.Error()
		if !es.Fatal() {
			t.Errorf("%s: no error: %s", name, got)
		}
		for _, w := range want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: no %q in: %s", name, w, got)
			}
		}
	}
}

func TestBodyFromFileHeight(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"docs/plan.md": bodyFileMD,
		"hug.tenn":     "element \"E\" { item \"Doc\" { width 400; body --from-file \"docs/plan.md#Цель\" } }\n",
		"short.tenn":   "element \"E\" { item \"Doc\" { width 400; height 40; body --from-file \"docs/plan.md#Цель\" } }\n",
	})
	es, _, err := scene.Diagnose(root, "hug.tenn", nil)
	if err != nil || len(es) > 0 {
		t.Errorf("a hugging item warns: %v %v", es, err)
	}
	es, _, err = scene.Diagnose(root, "short.tenn", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 1 || !es[0].Warn {
		t.Fatalf("warnings %v, want one warning", es)
	}
	if m := es[0].Msg; !strings.Contains(m, "needs") || !strings.Contains(m, "the item leaves") {
		t.Errorf("the warning has no numbers: %s", m)
	}
}
