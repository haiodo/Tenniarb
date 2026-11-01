package scene

import (
	"fmt"
	"sort"
	"strings"
)

// Template is the code a component element generates (spec 5c): text where ${name} is the value of the
// parameter name and <slot/> or <slot name/> is the content the use passes.
type Template struct {
	Text string // as written between the braces, the indentation of the component cut
	Pos  Pos    // the template command
}

// Render is the text node generates from its template (spec 5c): ${name} is the value of the parameter
// name, <slot/> and <slot name/> the content the use passes, each node as the title it sets or its name.
// node is a component element, or a use of one: the values and the slots then come from the block of that
// use, and values (the name=value pairs of "tenniarb gen -set") override both. Options.EvalVars evaluates
// the expressions, as it does for the scene; without an evaluator a ${...} is an error. The errors come
// back with the text and nothing is added to the diagnostics of the tree.
func (t *Tree) Render(node *Node, values map[string]string, o Options) (string, error) {
	eval := evaluator(o)
	var errs Errors
	var use *Node
	if node.Kind == KUse {
		use = node
		target, err := t.Resolve(node.Parent, node.Path)
		if err != nil {
			return "", Errors{{Pos: node.Pos, Msg: err.Error()}}
		}
		node = target
	}
	if node.Template == nil {
		return "", Errors{{Pos: node.Pos, Msg: fmt.Sprintf("the %s %q has no template", node.Kind, node.Name)}}
	}
	vars := renderVars(node, use, values, eval, &errs)
	text := renderText(node.Template, vars, renderSlots(node, use, eval, &errs), eval, &errs)
	if errs.Fatal() {
		return "", errs
	}
	return text, nil
}

// useProp is the value the block of use gives the parameter name, the last one written; nil for a use
// without it, or without a use at all.
func useProp(use *Node, name string) *Prop {
	if use == nil {
		return nil
	}
	var found *Prop
	for _, ov := range use.Block {
		if ov.Kind == OvProp && ov.Prop.Name == name {
			found = ov.Prop
		}
	}
	return found
}

// renderVars is the value of every parameter of target: the one values gives, the one the use writes, the
// default. A parameter without any of them, and a name the element does not declare, are errors.
func renderVars(target, use *Node, values map[string]string, eval evalFunc, errs *Errors) map[string]string {
	vars := make(map[string]string, len(target.Params))
	for _, prm := range target.Params {
		if v, ok := values[prm.Name]; ok {
			vars[prm.Name] = v
			continue
		}
		pr := useProp(use, prm.Name)
		if pr == nil {
			pr = prm.Default
		}
		if pr == nil {
			*errs = append(*errs, Error{Pos: target.Pos,
				Msg: fmt.Sprintf("element %q needs %q: set it in the block of the use or with -set", target.Name, prm.Name)})
			continue
		}
		v, ok := evalWords(eval, pr, nil)
		if !ok {
			*errs = append(*errs, Error{Pos: pr.Pos, Msg: fmt.Sprintf("parameter %q: the expression is not evaluated", prm.Name)})
		}
		vars[prm.Name] = v
	}
	unknown := make([]string, 0, len(values))
	for name := range values {
		if target.Param(name) == nil {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	for _, name := range unknown {
		declared := "it declares no parameters"
		if len(target.Params) > 0 {
			names := make([]string, len(target.Params))
			for i, prm := range target.Params {
				names[i] = prm.Name
			}
			declared = "parameters: " + quoteAll(names)
		}
		*errs = append(*errs, Error{Pos: target.Pos, Msg: fmt.Sprintf("unknown parameter %q (%s)", name, declared)})
	}
	return vars
}

// renderSlots is the content the block of use passes to the slots of the component, by normalized name and
// "" for the default one, in file order, and the fallback of the component for a slot it leaves empty: the
// element's own "slot { ... }" of that name (spec 4.7a).
func renderSlots(target, use *Node, eval evalFunc, errs *Errors) map[string]string {
	slots := map[string]string{}
	if use == nil {
		return fallbackSlots(target, slots, eval, errs)
	}
	for _, ov := range use.Block {
		switch ov.Kind {
		case OvNode:
			slots[""] += nodeText(ov.Node, eval, errs)
		case OvSlot:
			name := NormalizeName(ov.Slot)
			if _, ok := slots[name]; !ok { // a slot filled with no node still counts as filled
				slots[name] = ""
			}
			for _, sub := range ov.Block {
				if sub.Kind == OvNode {
					slots[name] += nodeText(sub.Node, eval, errs)
				}
			}
		}
	}
	return fallbackSlots(target, slots, eval, errs)
}

// fallbackSlots fills the slots the use leaves empty with the fallback the element writes for them: the
// "slot { ... }" of the item that holds the slots (spec 4.7a).
func fallbackSlots(target *Node, slots map[string]string, eval evalFunc, errs *Errors) map[string]string {
	var walk func(n *Node)
	walk = func(n *Node) {
		for _, c := range n.Content {
			if c.Kind != KSlot {
				walk(c)
				continue
			}
			if _, ok := slots[c.Key]; ok {
				continue
			}
			fill := ""
			for _, sub := range c.Content {
				if sub.Kind == KItem {
					fill += nodeText(sub, eval, errs)
				}
			}
			if fill != "" { // an empty fallback leaves the slot unfilled, and that is an error
				slots[c.Key] = fill
			}
		}
	}
	if target != nil {
		walk(target)
	}
	return slots
}

// nodeText is a node the use passes to a slot as code: the title it sets, and its name otherwise.
func nodeText(n *Node, eval evalFunc, errs *Errors) string {
	var title *Prop
	for _, p := range n.Props {
		if p.Name == "title" {
			title = p
		}
	}
	if title == nil {
		return n.Name
	}
	v, ok := evalWords(eval, title, nil)
	if !ok {
		*errs = append(*errs, Error{Pos: title.Pos, Msg: "the title of the slot content is not evaluated"})
	}
	return v
}

// renderText is the text of a template with ${name} replaced by its value and <slot/> by the content.
func renderText(tpl *Template, vars, slots map[string]string, eval evalFunc, errs *Errors) string {
	text := tpl.Text
	var b strings.Builder
	for i := 0; i < len(text); {
		if strings.HasPrefix(text[i:], "${") {
			if j := strings.IndexByte(text[i+2:], '}'); j >= 0 {
				code := text[i+2 : i+2+j]
				if v, ok := eval(code, vars); !ok {
					*errs = append(*errs, Error{Pos: tpl.Pos, Msg: fmt.Sprintf("${%s}: the expression is not evaluated", code)})
				} else {
					b.WriteString(v)
				}
				i += len("${") + j + 1
				continue
			}
		}
		if name, n, ok := slotTag(text[i:]); ok {
			if s, filled := slots[name]; filled {
				b.WriteString(s)
			} else if name == "" {
				*errs = append(*errs, Error{Pos: tpl.Pos, Msg: "the default slot has no content: pass nodes in the block of the use"})
			} else {
				*errs = append(*errs, Error{Pos: tpl.Pos, Msg: fmt.Sprintf("slot %q has no content: pass nodes in the block of the use", name)})
			}
			i += n
			continue
		}
		b.WriteByte(text[i])
		i++
	}
	return b.String()
}

// slotTag reads "<slot/>" or "<slot name/>" at the start of s: the normalized slot name, "" for the
// default slot, and the length of the tag. Any other form, such as <slot name="x"/>, stays text.
func slotTag(s string) (string, int, bool) {
	const open = "<slot"
	if !strings.HasPrefix(s, open) {
		return "", 0, false
	}
	end := strings.IndexByte(s, '>')
	if end < 0 {
		return "", 0, false
	}
	name := strings.TrimSpace(strings.TrimSuffix(s[len(open):end], "/"))
	for _, r := range name {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-", r) {
			return "", 0, false // a self-closing slot of another form: the text stays
		}
	}
	return NormalizeName(name), end + 1, true
}
