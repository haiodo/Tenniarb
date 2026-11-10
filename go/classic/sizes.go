package classic

import (
	"golang.org/x/text/unicode/norm"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

// ItemAt is the item of el a node of the element is written for, by its place among the items of its
// parent. Only a node of the element itself answers: a part of a component has no item of the model.
func ItemAt(e *model.Element, n *scene.Node) *model.DiagramItem {
	if e == nil || n == nil || n.Parent == nil {
		return nil
	}
	k := 0
	for _, c := range n.Parent.Content {
		if c == n {
			break
		}
		if c.Kind == scene.KItem {
			k++
		}
	}
	for _, it := range e.Items {
		if it.Kind != model.ItemKindItem {
			continue
		}
		if k == 0 {
			return it
		}
		k--
	}
	return nil
}

// Sizes is how big an item is, read from the box the scene draws
// it in. The evaluator asks at every round with the values of the round before, so one build of the element
// answers every item of that round and the next round builds again.
func Sizes(tree *scene.Tree, path string, el *model.Element, measure gfx.TextMeasurer) func(*model.DiagramItem, model.IExecutionContext) *model.DrawableSize {
	var (
		built  model.IExecutionContext
		sizes  map[*model.DiagramItem]model.DrawableSize
		failed bool
	)
	return func(item *model.DiagramItem, prev model.IExecutionContext) *model.DrawableSize {
		if item == nil || tree == nil {
			// No tree, no sizes: a caller that has no scene places the items from the model alone.
			return nil
		}
		if sizes == nil || built != prev {
			sizes, failed = map[*model.DiagramItem]model.DrawableSize{}, false
			built = prev
			sc := gfx.NewScene()
			sc.Update(func(tx *gfx.Tx) {
				screen, err := tree.Build(tx, nil, path, scene.Options{
					Measurer: measure,
					Classic:  Values(el, prev),
				})
				if err != nil || screen == nil {
					failed = true
					return
				}
				collectSizes(el, screen.Placed(), sizes)
			})
		}
		if failed {
			return nil
		}
		if sz, ok := sizes[item]; ok {
			return &sz
		}
		return nil
	}
}

// collectSizes walks what the scene placed and writes the box of every item of el.
func collectSizes(el *model.Element, p *scene.Placed, out map[*model.DiagramItem]model.DrawableSize) {
	for _, k := range p.Kids {
		if k.Src != nil && k.Src.Kind == scene.KItem {
			if it := ItemAt(el, k.Src); it != nil {
				out[it] = model.DrawableSize{Width: k.W, Height: k.H}
			}
		}
		collectSizes(el, k, out)
	}
}

// SceneContext is Context with the item sizes of the scene: what the evaluator asks for `width` and
// `height` is answered by the box the scene draws, and measureText is the size of a text the caller
// measures with - the same measurer the canvas draws it with.
func SceneContext(tree *scene.Tree, path string, measure gfx.TextMeasurer, measureText func(string, float64) model.DrawableSize) Context {
	return func(el *model.Element) model.IExecutionContext {
		ctx := exec.NewExecutionContext(exec.Options{
			ItemSize:    Sizes(tree, path, el, measure),
			MeasureText: measureText,
		})
		ctx.SetElement(el)
		return ctx
	}
}

// LinkAt is the link of el a node of the element is written for: a link is matched by its ends and by its
// place among the links with the same ends, which is how the file resolves `link A B` when it repeats.
func LinkAt(e *model.Element, n *scene.Node) *model.DiagramItem {
	if e == nil || n == nil || n.Kind != scene.KLink {
		return nil
	}
	nth := 0
	if n.Parent != nil {
		for _, c := range n.Parent.Content {
			if c == n {
				break
			}
			if c.Kind == scene.KLink && c.From == n.From && c.To == n.To {
				nth++
			}
		}
	}
	seen := 0
	for _, it := range e.Items {
		if it.Kind != model.ItemKindLink {
			continue
		}
		from, to := endNames(it)
		if norm.NFC.String(from) != norm.NFC.String(n.From) || norm.NFC.String(to) != norm.NFC.String(n.To) {
			continue
		}
		if seen == nth {
			return it
		}
		seen++
	}
	return nil
}
