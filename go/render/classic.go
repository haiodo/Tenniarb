package render

import (
	"golang.org/x/text/unicode/norm"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/tenn"
)

// ClassicValues gives the values of classic diagram el for scene.Options.Classic: the text of every
// argument of prop on the node at, evaluated by ctx. An argument without a value is its literal text;
// a node or a property the model does not hold gives nothing.
func ClassicValues(el *model.Element, ctx model.IExecutionContext) func(at scene.ClassicRef, prop string) ([]string, bool) {
	targets := map[scene.ClassicRef]model.EvalTarget{
		{Kind: scene.RItem, Name: el.Name, Root: true}: el,
	}
	items := map[string]int{}
	for _, it := range el.Items {
		if it.Kind != model.ItemKindItem {
			continue
		}
		key := norm.NFC.String(it.Name)
		targets[scene.ClassicRef{Kind: scene.RItem, Name: it.Name, Nth: items[key]}] = it
		items[key]++
	}
	links := map[[2]string]int{}
	for _, it := range el.Items {
		if it.Kind != model.ItemKindLink {
			continue
		}
		from, to := endNames(it)
		key := [2]string{norm.NFC.String(from), norm.NFC.String(to)}
		targets[scene.ClassicRef{Kind: scene.RLink, From: from, To: to, Nth: links[key]}] = it
		links[key]++
	}
	return func(at scene.ClassicRef, prop string) ([]string, bool) {
		target, ok := targets[at]
		if !ok {
			return nil, false
		}
		props := propertiesOf(target)
		cmd := props.Get(prop)
		if cmd == nil {
			return nil, false
		}
		ev := ctx.GetEvaluated(target, nil, nil)
		out := make([]string, 0, len(cmd.Children))
		for _, a := range argsOf(cmd) {
			if v, ok := ev[a.Token]; ok {
				out = append(out, jsToString(v))
				continue
			}
			s, _ := a.GetIdentText()
			out = append(out, s)
		}
		return out, true
	}
}

func propertiesOf(target model.EvalTarget) *model.ModelProperties {
	switch t := target.(type) {
	case *model.Element:
		return t.Properties
	case *model.DiagramItem:
		return t.Properties
	}
	return nil
}

// endNames are the names of the items a link joins, as the format writes them.
func endNames(link *model.DiagramItem) (from, to string) {
	if link.Source != nil {
		from = link.Source.Name
	}
	if link.Target != nil {
		to = link.Target.Name
	}
	return from, to
}

// argsOf are the arguments of a command: its children after the name, a block excluded.
func argsOf(cmd *tenn.Node) []*tenn.Node {
	if len(cmd.Children) == 0 {
		return nil
	}
	out := cmd.Children[1:]
	if n := len(out); n > 0 && out[n-1].Kind == tenn.KindBlockExpr {
		out = out[:n-1]
	}
	return out
}
