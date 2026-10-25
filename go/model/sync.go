package model

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/haiodo/tenniarb/go/tenn"
)

// Port of web/packages/core/src/sync.ts (SyncExtension.swift toSyncJson).

func nl(s string) string { return strings.ReplaceAll(s, "\n", "\\n") }

// jsString is JSON.stringify for a string: no HTML escaping, U+2028 and U+2029 stay as they are.
func jsString(s string) string {
	var sb strings.Builder
	enc := json.NewEncoder(&sb)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // a string always encodes
	out := strings.TrimSuffix(sb.String(), "\n")
	out = strings.ReplaceAll(out, ` `, " ")
	return strings.ReplaceAll(out, ` `, " ")
}

// jsNumber is JSON.stringify for a number: shortest digits, exponent form from 1e21 and below 1e-6, null for NaN and infinities.
func jsNumber(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "null"
	}
	if v == 0 {
		return "0"
	}
	if a := math.Abs(v); a >= 1e21 || a < 1e-6 {
		s := strconv.FormatFloat(v, 'e', -1, 64)
		mant, exp, _ := strings.Cut(s, "e")
		sign := exp[:1]
		exp = strings.TrimLeft(exp[1:], "0")
		if sign == "-" {
			return mant + "e-" + exp
		}
		return mant + "e+" + exp
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

type syncWriter struct {
	sb strings.Builder
}

// field writes `"key" : value` as an entry of the object at the given depth; first says whether it opens the object.
func (w *syncWriter) field(depth int, first bool, key, value string) {
	if !first {
		w.sb.WriteString(",")
	}
	w.sb.WriteString("\n")
	w.sb.WriteString(strings.Repeat("  ", depth))
	w.sb.WriteString(jsString(key) + " : " + value)
}

// ToSyncJSON converts an element to the JSON of its items and edges, in the layout of Swift JSONEncoder
// .prettyPrinted: `"key" : value`, escaped slashes, absent keys for nil.
func ToSyncJSON(el *Element) string {
	var items, edges []string
	for _, it := range el.Items {
		link := it.isLink && it.Source != nil && it.Target != nil
		obj := &syncWriter{}
		obj.sb.WriteString("{")
		obj.field(3, true, "kind", jsString(ItemCommandName(it.Kind)))
		obj.field(3, false, "name", jsString(nl(it.Name)))
		obj.field(3, false, "id", jsString(it.ID))
		obj.field(3, false, "pos", "{"+
			"\n"+strings.Repeat("  ", 4)+`"x" : `+jsNumber(it.X)+","+
			"\n"+strings.Repeat("  ", 4)+`"y" : `+jsNumber(it.Y)+
			"\n"+strings.Repeat("  ", 3)+"}")
		if it.Description != nil {
			obj.field(3, false, "description", jsString(nl(*it.Description)))
		}
		if props := it.Properties.Nodes(); len(props) > 0 {
			obj.field(3, false, "properties", syncProperties(props))
		}
		if link {
			obj.field(3, false, "source", jsString(it.Source.ID))
			obj.field(3, false, "target", jsString(it.Target.ID))
		}
		obj.sb.WriteString("\n" + strings.Repeat("  ", 2) + "}")
		if link {
			edges = append(edges, obj.sb.String())
		} else {
			items = append(items, obj.sb.String())
		}
	}

	w := &syncWriter{}
	w.sb.WriteString("{")
	w.field(1, true, "name", jsString(el.Name))
	if el.Description != nil {
		w.field(1, false, "description", jsString(*el.Description))
	}
	if len(items) > 0 {
		w.field(1, false, "items", syncArray(items, 2))
	}
	if len(edges) > 0 {
		w.field(1, false, "edges", syncArray(edges, 2))
	}
	w.sb.WriteString("\n}")
	return strings.ReplaceAll(w.sb.String(), "/", `\/`)
}

// syncArray joins already formatted elements, which carry their own indentation of depth.
func syncArray(elems []string, depth int) string {
	pad := "\n" + strings.Repeat("  ", depth)
	return "[" + pad + strings.Join(elems, ","+pad) + "\n" + strings.Repeat("  ", depth-1) + "]"
}

func syncProperties(props []*tenn.Node) string {
	rows := make([]string, 0, len(props))
	for _, p := range props {
		cells := make([]string, 0, len(p.Children))
		for _, c := range p.Children {
			cells = append(cells, jsString(nl(tenn.ToStr(c, 0, true))))
		}
		if len(cells) == 0 {
			rows = append(rows, "[]")
			continue
		}
		pad := "\n" + strings.Repeat("  ", 5)
		rows = append(rows, "["+pad+strings.Join(cells, ","+pad)+"\n"+strings.Repeat("  ", 4)+"]")
	}
	pad := "\n" + strings.Repeat("  ", 4)
	return "[" + pad + strings.Join(rows, ","+pad) + "\n" + strings.Repeat("  ", 3) + "]"
}
