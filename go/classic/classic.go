// Package classic is the element model as a classic diagram reads it: the values go/scene asks for through
// scene.Options.Classic. It is the part of go/render the scene kept after the canvas moved to go/scene, so
// the old renderer can go away without the pictures of an old file changing.
package classic

import (
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/text/unicode/norm"

	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/tenn"
)

// Context builds the evaluator of el. The caller decides where the expressions run: the expression engine
// with the item sizes of go/render today, the one of go/scene once the old renderer is gone (E7).
type Context func(el *model.Element) model.IExecutionContext

// Values gives the values of classic diagram el for scene.Options.Classic: the text of every argument of
// prop on the node at, evaluated by ctx. An argument without a value is its literal text; a node or a
// property the model does not hold gives nothing.
func Values(el *model.Element, ctx model.IExecutionContext) func(at scene.ClassicRef, prop string) ([]string, bool) {
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
				out = append(out, jsValue(v))
				continue
			}
			s, _ := a.GetIdentText()
			out = append(out, s)
		}
		return out, true
	}
}

// Hook is scene.Options.Classic for path when it names an element of the element model, so a classic
// diagram reads the values the model computes. nil for a file root, an element the file does not declare
// or one whose content is in another file: those keep their own evaluator, or ignore the expressions as
// before.
func Hook(tree *scene.Tree, root, file, path string, ctx Context) func(scene.ClassicRef, string) ([]string, bool) {
	if path == "" || ctx == nil {
		return nil
	}
	src, ok := fileSource(tree, root, file)
	if !ok {
		return nil
	}
	parsed, parser := tenn.Parse(strings.TrimPrefix(src, "\xef\xbb\xbf"))
	if parser.Errors.HasErrors() {
		return nil
	}
	el := findElement(&model.ParseTenn(parsed).Element, path)
	if el == nil || el.FromFile != "" {
		return nil
	}
	var vals func(scene.ClassicRef, string) ([]string, bool)
	return func(at scene.ClassicRef, prop string) ([]string, bool) {
		if vals == nil {
			vals = Values(el, ctx(el))
		}
		return vals(at, prop)
	}
}

// fileSource is the text of file as scene.Load keyed it: relative to the workspace root, with slashes.
func fileSource(tree *scene.Tree, root, file string) (string, bool) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	abs := file
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(absRoot, file)
	}
	rel, err := filepath.Rel(absRoot, abs)
	if err != nil {
		return "", false
	}
	return tree.Source(filepath.ToSlash(rel))
}

// findElement follows a dotted scene path (spec 4.2) through the elements of the model: a quoted segment
// is a name as written, a bare one its key.
func findElement(top *model.Element, path string) *model.Element {
	segs, ok := scene.SplitPath(path)
	if !ok {
		return nil
	}
	el := top
	for _, seg := range segs {
		var next *model.Element
		for _, c := range el.Elements {
			if seg.IsName(c.Name) {
				next = c
				break
			}
		}
		if next == nil {
			return nil
		}
		el = next
	}
	return el
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

// jsValue is JSValue.toString() for the values an expression engine hands back, as go/render writes them:
// a list joins with commas, an undefined element leaves an empty place.
func jsValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return jsNum(x)
	case float32:
		return jsNum(float64(x))
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			if _, undef := e.(exec.UndefinedType); e != nil && !undef {
				parts[i] = jsValue(e)
			}
		}
		return strings.Join(parts, ",")
	}
	return exec.ToString(v)
}

// jsNum is JSValue.toString() for a number: NaN and the infinities keep their names.
func jsNum(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == 0:
		return "0"
	}
	if a := math.Abs(f); a >= 1e21 || a < 1e-6 {
		s := strconv.FormatFloat(f, 'e', -1, 64)
		mant, exp, _ := strings.Cut(s, "e")
		sign := exp[:1]
		exp = strings.TrimLeft(exp[1:], "0")
		if sign == "-" {
			return mant + "e-" + exp
		}
		return mant + "e+" + exp
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}
