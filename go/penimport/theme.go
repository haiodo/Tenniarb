package penimport

import (
	"fmt"
	"sort"
	"strings"
)

// A pen theme is a named axis of the file - "themes": {"direction": ["home", "sale"]} - and a node names
// the value of it for its subtree. That is the context of .tenn, so the theme becomes `context <key>
// <value>`, and a variable whose value the file writes per theme becomes an expression that reads it back.

// contextProp is one `context <key> <value>` declaration.
func contextProp(key, value string) Prop {
	arg := wordArg(value)
	if !simpleIdent(value) {
		arg = strArg(value)
	}
	return prop("context", wordArg(key), arg)
}

// rootContext is the first value of every theme axis at the root of the file: a node that names no theme
// sees it, and a themed variable has a context to read wherever it is written. The axes of the imported
// libraries are here too: a variable declared there is written as an expression in this file.
func (f *File) rootContext() []Prop {
	seen := map[string]bool{}
	var ps []Prop
	files := importGraph(f)
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	for _, g := range files {
		th, _ := g.raw["themes"].(map[string]any)
		for _, k := range sortedKeys(th) {
			if v := g.themeDefault(k); v != "" && !seen[k] {
				seen[k] = true
				ps = append(ps, contextProp(k, v))
			}
		}
	}
	return ps
}

// themeProps is the theme a node declares for its subtree, as context commands.
func themeProps(m map[string]any) []Prop {
	th, _ := m["theme"].(map[string]any)
	var ps []Prop
	for _, k := range sortedKeys(th) {
		if v := toStr(th[k]); v != "" {
			ps = append(ps, contextProp(k, v))
		}
	}
	return ps
}

// themeDefault is the first value of an axis: what a node that names no theme sees.
func (f *File) themeDefault(key string) string {
	th, _ := f.raw["themes"].(map[string]any)
	list, _ := th[key].([]any)
	if len(list) == 0 {
		return ""
	}
	return toStr(list[0])
}

// themedColor is a color the file writes once per theme value, as an expression of the context:
// ${ctx.direction === "sale" ? "#a1b2c3" : "#c6f24e"}. The last branch is the first value of the axis, so
// a node outside any theme keeps the color the file would show there. ok is false when the values do not
// share one key of one axis, and the caller resolves the color for one theme instead.
func (c *conv) themedColor(f *File, v map[string]any) (string, bool) {
	items, _ := v["value"].([]any)
	if len(items) < 2 {
		return "", false
	}
	key := ""
	for _, it := range items {
		im, _ := it.(map[string]any)
		th, _ := im["theme"].(map[string]any)
		if len(th) != 1 {
			return "", false
		}
		for k := range th {
			if key != "" && k != key {
				return "", false
			}
			key = k
		}
	}
	def := f.themeDefault(key)
	if def == "" {
		return "", false
	}
	type branch struct{ val, col string }
	all := make([]branch, 0, len(items))
	fallback := ""
	for _, it := range items {
		im, _ := it.(map[string]any)
		th, _ := im["theme"].(map[string]any)
		col, ok := c.color(f, toStr(im["value"]), "", false)
		if !ok || strings.HasPrefix(col, "${") {
			return "", false
		}
		if toStr(th[key]) == def {
			fallback = col
		}
		all = append(all, branch{toStr(th[key]), col})
	}
	if fallback == "" {
		return "", false
	}
	var sb strings.Builder
	sb.WriteString("${")
	conds := 0
	for _, b := range all {
		if b.val != def && b.col != fallback {
			fmt.Fprintf(&sb, "ctx.%s === %q ? %q : ", key, b.val, b.col)
			conds++
		}
	}
	if conds == 0 {
		return "", false
	}
	sb.WriteString(fmt.Sprintf("%q", fallback))
	sb.WriteString("}")
	return sb.String(), true
}
