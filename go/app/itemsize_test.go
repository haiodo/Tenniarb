//go:build darwin || linux || windows

package app

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/classic"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/scene"
)

// TestSceneSizesItemsLikeRender is the gate of the step that moves the evaluator off go/render: the
// expression engine asks how big an item is (`width` and `height` in an expression), and go/render answers
// by building a throwaway drawable scene. The scene must answer the same, or moving the evaluator would
// change the picture of an item.
//
// It measures every item of docs/Example.tenn through both, and holds the number that agree: the ones that
// do not are the body layouts the parity stand also reports (a list, an emphasis that spans lines, a code
// span the scene draws in `mono` while go/render never read that property), so the ratio may only grow. It
// is 97% now.
func TestSceneSizesItemsLikeRender(t *testing.T) {
	const file = "../../docs/Example.tenn" // from the go directory, as the other tests read it
	const rel = "docs/Example.tenn"        // from the workspace root, as scene.Load wants it
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	m := model.ReadTenn(string(data))
	tree, err := scene.Load("../..", rel)
	if err != nil {
		t.Fatal(err)
	}
	var all []*model.Element
	var walk func(*model.Element)
	walk = func(e *model.Element) {
		all = append(all, e)
		for _, c := range e.Elements {
			walk(c)
		}
	}
	walk(&m.Element)

	compared, agree := 0, 0
	var worst []string
	for _, el := range all {
		if len(el.Items) == 0 {
			continue
		}
		keys := pathKeys(&m.Element, el)
		if len(keys) < 2 {
			continue // the root itself is not a path of the scene
		}
		ctx := render.NewExecContext(el, render.Options{Measurer: gfx.DefaultMeasurer})
		sc := gfx.NewScene()
		var screen *scene.Screen
		var buildErr error
		sc.Update(func(tx *gfx.Tx) {
			screen, buildErr = tree.Build(tx, nil, strings.Join(keys[1:], "."), scene.Options{
				Measurer: gfx.DefaultMeasurer, Classic: classic.Values(el, ctx)})
		})
		if buildErr != nil || screen == nil {
			continue
		}
		sizes := map[*model.DiagramItem]*scene.Placed{}
		var find func(p *scene.Placed)
		find = func(p *scene.Placed) {
			for _, k := range p.Kids {
				if k.Src != nil && k.Src.Kind == scene.KItem {
					if it := classicItemAt(el, k.Src); it != nil {
						sizes[it] = k
					}
				}
				find(k)
			}
		}
		find(screen.Placed())
		for _, it := range el.Items {
			if it.Kind != model.ItemKindItem {
				continue
			}
			pl := sizes[it]
			if pl == nil {
				continue
			}
			sz, err := render.ItemSize(it, render.Options{Measurer: gfx.DefaultMeasurer, Exec: ctx})
			if err != nil {
				continue
			}
			compared++
			d := math.Max(math.Abs(pl.W-sz.Width), math.Abs(pl.H-sz.Height))
			if d <= 0.5 {
				agree++
				continue
			}
			worst = append(worst, fmt.Sprintf("%s/%s off %.2f", el.Name, it.Name, d))
		}
	}
	if compared < 1000 {
		t.Fatalf("only %d items compared; the file or the walk changed", compared)
	}
	ratio := float64(agree) / float64(compared)
	t.Logf("%d of %d items agree (%.2f%%), %d differ", agree, compared, ratio*100, len(worst))
	sort.Slice(worst, func(i, j int) bool { return worst[i] < worst[j] })
	for i, w := range worst {
		if i == 8 {
			break
		}
		t.Logf("  %s", w)
	}
	if ratio < 0.95 {
		t.Errorf("%.2f%% of the items agree, want at least 95%%: the scene and go/render size items differently",
			ratio*100)
	}
}

// pathKeys is the dotted key path of el from the root of the model.
func pathKeys(root, el *model.Element) []string {
	if root == el {
		return []string{root.Name}
	}
	for _, c := range root.Elements {
		if p := pathKeys(c, el); p != nil {
			return append([]string{root.Name}, p...)
		}
	}
	return nil
}
