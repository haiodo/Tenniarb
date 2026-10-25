// Package exec is a port of web/packages/core/src/execution.ts (ExecutionModel.swift): expression evaluation for
// $(...), ${...} and "${...}" values on goja. Each ElementContext owns one goja Runtime, as Swift owns one JSContext;
// the runtime's global object is the scope, so implicit globals, `var` and `function` from one expression stay visible
// to the next. Ceiling: no sandbox (host builtins visible); a round is cut off after Options.RoundLimit. Untrusted
// documents: Options.NoEvaluate.
package exec

import (
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf16"

	"github.com/dop251/goja"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/tenn"
)

var _ model.IExecutionContext = (*ExecutionContext)(nil)

// Options are the TS ExecutionOptions. NoEvaluate is inverted (TS `evaluate`, default true) to keep the zero value useful.
type Options struct {
	// NoEvaluate: nothing runs and GetEvaluated is always empty, so callers see raw token text.
	NoEvaluate bool
	// RoundLimit cuts off one evaluation round (SetElement, UpdateAll, NotifyChanges, a preview GetEvaluated):
	// running expressions fail with an Error value. Zero is DefaultRoundLimit, negative is no limit.
	RoundLimit time.Duration
	// ItemSize stands in for the drawable's bounds; without it (or when it returns nil) `width`, `height`,
	// `defaults` are not defined. It runs under the context lock and must not call the context, only prev:
	// a read-only view of the values of the previous round (Swift's throwaway scene reads the same), valid
	// until ItemSize returns.
	ItemSize func(item *model.DiagramItem, prev model.IExecutionContext) *model.DrawableSize
	// MeasureText is the raw text frame size (CTFramesetter in Swift); utils.textSize adds the padding on top.
	// It runs inside an expression, under the lock, and must not call the context.
	MeasureText func(text string, fontSize float64) model.DrawableSize
}

// DefaultRoundLimit is the RoundLimit used when none is set.
const DefaultRoundLimit = time.Second

const errRoundLimit = "Execution time limit exceeded"

// Swift value kinds that checkChanges and sum tell apart; a JS number cannot carry Int vs Double.
// A stored value is one of: string, int64 (Swift Int), swiftDouble, jsValue, position, *dict, []any, nil (null).
type swiftDouble float64

type jsValue struct{ v goja.Value }

type position struct{ x, y float64 }

type orderedMap[K comparable, V any] struct {
	keys []K
	m    map[K]V
}

// set keeps the place of an existing key, like a JS Map.
func (o *orderedMap[K, V]) set(k K, v V) {
	if o.m == nil {
		o.m = map[K]V{}
	}
	if _, ok := o.m[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.m[k] = v
}

func (o *orderedMap[K, V]) get(k K) (V, bool) {
	v, ok := o.m[k]
	return v, ok
}

func (o *orderedMap[K, V]) delete(k K) bool {
	if _, ok := o.m[k]; !ok {
		return false
	}
	delete(o.m, k)
	o.keys = slices.DeleteFunc(o.keys, func(x K) bool { return x == k })
	return true
}

func (o *orderedMap[K, V]) len() int { return len(o.keys) }

type dict = orderedMap[string, any]

// toJS is setObject: native containers are copied, jsValues pass through.
func (j *jsContext) toJS(v any) goja.Value {
	switch x := v.(type) {
	case nil:
		return goja.Null()
	case string:
		return j.rt.ToValue(x)
	case int64:
		return j.rt.ToValue(float64(x))
	case swiftDouble:
		return j.rt.ToValue(float64(x))
	case jsValue:
		if x.v == nil {
			return goja.Undefined()
		}
		return x.v
	case position:
		return j.frozen(map[string]float64{"x": x.x, "y": x.y}, "x", "y")
	case *dict:
		obj := j.rt.NewObject()
		for _, k := range x.keys {
			_ = obj.Set(k, j.toJS(x.m[k]))
		}
		return obj
	case []any:
		vals := make([]any, len(x))
		for i, e := range x {
			vals[i] = j.toJS(e)
		}
		return j.rt.NewArray(vals...)
	}
	return goja.Undefined()
}

// frozen is Object.freeze({keys...}).
func (j *jsContext) frozen(vals map[string]float64, keys ...string) goja.Value {
	obj := j.rt.NewObject()
	for _, k := range keys {
		_ = obj.Set(k, vals[k])
	}
	res, _ := j.freeze(goja.Undefined(), obj)
	return res
}

// jsContext is the stand-in for the JSContext of one ElementContext; its global object is the scope.
// Not goroutine-safe: ExecutionContext.mu guards every use.
type jsContext struct {
	rt     *goja.Runtime
	global *goja.Object
	eval   goja.Callable
	freeze goja.Callable
	// expired: the round limit has passed. goja clears its own interrupt flag when it unwinds an interrupted
	// call, so this keeps the rest of the round failing fast.
	expired atomic.Bool
}

// maxCallStack makes runaway recursion a catchable RangeError; goja has no limit by default and would exhaust the Go stack.
const maxCallStack = 10000

func newJSContext() *jsContext {
	rt := goja.New()
	rt.SetMaxCallStackSize(maxCallStack)
	eval, _ := goja.AssertFunction(rt.Get("eval"))
	freeze, _ := goja.AssertFunction(rt.Get("Object").ToObject(rt).Get("freeze"))
	return &jsContext{rt: rt, global: rt.GlobalObject(), eval: eval, freeze: freeze}
}

// set ignores a failed write (a read-only global such as `undefined`), as JSC does for setObject.
func (j *jsContext) set(name string, v goja.Value) {
	_ = j.global.Set(name, v)
}

// evaluate is an indirect eval, so `let`/`const` stay local to one expression while `var` and functions land in the global.
// A thrown value comes back as is, with failed set.
func (j *jsContext) evaluate(code string) (value goja.Value, failed bool) {
	if j.expired.Load() {
		return j.limitError(), true
	}
	res, err := j.eval(goja.Undefined(), j.rt.ToValue(code))
	if err == nil {
		return res, false
	}
	var ex *goja.Exception
	if errors.As(err, &ex) {
		return ex.Value(), true
	}
	var intr *goja.InterruptedError
	if errors.As(err, &intr) {
		return j.limitError(), true
	}
	var so *goja.StackOverflowError
	if errors.As(err, &so) {
		if o, err := j.rt.New(j.rt.Get("RangeError"), j.rt.ToValue("Maximum call stack size exceeded")); err == nil {
			return o, true
		}
	}
	return j.rt.ToValue(err.Error()), true
}

func (j *jsContext) limitError() goja.Value {
	o, err := j.rt.New(j.rt.Get("Error"), j.rt.ToValue(errRoundLimit))
	if err != nil {
		return j.rt.ToValue(errRoundLimit)
	}
	return o
}

// startRound interrupts running code after limit and fails every evaluation after that until the returned
// func runs; it must run before the lock is released.
func (j *jsContext) startRound(limit time.Duration) (end func()) {
	if limit < 0 {
		return func() {}
	}
	if limit == 0 {
		limit = DefaultRoundLimit
	}
	fired := make(chan struct{})
	timer := time.AfterFunc(limit, func() {
		defer close(fired)
		j.expired.Store(true)
		j.rt.Interrupt(errRoundLimit)
	})
	return func() {
		if !timer.Stop() {
			<-fired // the callback is running or done: it must not set the flag after the clear
		}
		j.rt.ClearInterrupt()
		j.expired.Store(false)
	}
}

var reserved = map[string]bool{}

func init() {
	for _, w := range strings.Fields("break case catch class const continue debugger default delete do else enum export extends false finally for function " +
		"if import in instanceof new null return super switch this throw true try typeof var void while with") {
		reserved[w] = true
	}
}

func isIDStart(r rune) bool {
	return r == '$' || r == '_' || unicode.IsLetter(r) || unicode.Is(unicode.Nl, r)
}

func isIdentifier(name string) bool {
	for i, r := range name {
		if !isIDStart(r) && (i == 0 || !(unicode.In(r, unicode.Mn, unicode.Mc, unicode.Nd, unicode.Pc) || r == '‌' || r == '‍')) {
			return false
		}
	}
	return name != ""
}

// deleteName: Swift runs `delete name` as a script, which is a SyntaxError (nothing deleted) for non-identifiers and keywords.
func (j *jsContext) deleteName(name string) {
	if isIdentifier(name) && !reserved[name] {
		_ = j.global.Delete(name) // a `var` binding is not configurable and stays, as in JS
	}
}

func convertNameJS(name string) (string, bool) {
	var b strings.Builder
	for _, c := range name {
		if unicode.IsLetter(c) || unicode.IsMark(c) || unicode.IsNumber(c) {
			b.WriteRune(c)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String(), b.Len() > 0
}

func nameOf(name string) string {
	if n, ok := convertNameJS(name); ok {
		return n
	}
	return name
}

func (ec *elementContext) runScript(code string, tok *tenn.Token, evaluated map[*tenn.Token]any) jsValue {
	v, failed := ec.js.evaluate(code)
	if tok != nil {
		evaluated[tok] = exportValue(v)
	}
	if failed {
		return jsValue{goja.Undefined()}
	}
	return jsValue{v}
}

func (ec *elementContext) processBlock(node *tenn.Node, levelObject *dict, evaluated map[*tenn.Token]any) bool {
	has := false
	model.TraverseBlock(node, func(cmdNameRaw string, blChild *tenn.Node) {
		cmdName, ok := convertNameJS(cmdNameRaw)
		if !ok {
			return
		}
		switch {
		case blChild.Count() == 2:
			ci := blChild.GetChild(1)
			has = has || ci.Kind == tenn.KindExpressionBlock || ci.Kind == tenn.KindExpression
			// We have simple field assignement, not array
			value := ec.calculateValue(ci, levelObject, &has, evaluated)
			if value != nil {
				levelObject.set(cmdName, value)
				// An empty block is stored but not published to the context.
				if d, isDict := value.(*dict); !isDict || d.len() > 0 {
					ec.js.set(cmdName, ec.js.toJS(value))
				}
			}
		case blChild.Count() > 2:
			// We have array assignement
			values := make([]any, 0, blChild.Count()-1)
			for i := 1; i < blChild.Count(); i++ {
				ci := blChild.GetChild(i)
				has = has || ci.Kind == tenn.KindExpressionBlock || ci.Kind == tenn.KindExpression
				values = append(values, ec.calculateValue(ci, levelObject, &has, evaluated))
			}
			levelObject.set(cmdName, values)
			ec.js.set(cmdName, ec.js.toJS(values))
		default:
			// Set empty value, to mark field defined
			levelObject.set(cmdName, "")
			ec.js.set(cmdName, ec.js.rt.ToValue(""))
		}
	})
	return has
}

// calculateValue returns nil for TS null.
func (ec *elementContext) calculateValue(nde *tenn.Node, currentScope *dict, has *bool, evaluated map[*tenn.Token]any) any {
	if nde == nil {
		return nil
	}
	identText, hasText := nde.GetIdentText()
	switch nde.Kind {
	case tenn.KindFloatLit:
		if !hasText {
			return nil
		}
		d := jsStringToNumber(identText)
		if math.IsNaN(d) {
			d = 0
		}
		return swiftDouble(d)
	case tenn.KindIntLit:
		if !hasText {
			return nil
		}
		// Swift Int(String) is 64-bit and strict; anything else falls back to 0.0.
		n, err := strconv.ParseInt(identText, 10, 64)
		if err != nil {
			return swiftDouble(0)
		}
		return n
	case tenn.KindStringLit, tenn.KindCharLit, tenn.KindIdent, tenn.KindMarkdownLit:
		if !hasText {
			return nil
		}
		if strings.Contains(identText, "${") {
			*has = true
			// We need to perform substituion
			return ec.runScript("`"+identText+"`", nde.Token, evaluated)
		}
		return identText
	case tenn.KindBlockExpr:
		// A subcontext need to be constructed
		blockScope := &dict{}
		he := ec.processBlock(nde, blockScope, evaluated)
		// We need to cleanup current context from inner scope values
		for _, k := range blockScope.keys {
			if csv, ok := currentScope.get(k); ok {
				ec.js.set(k, ec.js.toJS(csv))
			}
		}
		*has = *has || he
		return blockScope
	case tenn.KindExpressionBlock, tenn.KindExpression:
		*has = true
		if !hasText {
			return nil
		}
		return ec.runScript(identText, nde.Token, evaluated)
	}
	return nil
}

// estimateText is used when no MeasureText is given; utf16 length matches TS text.length.
func estimateText(text string, fontSize float64) model.DrawableSize {
	return model.DrawableSize{Width: float64(len(utf16.Encode([]rune(text)))) * fontSize * 0.55, Height: fontSize * 1.2}
}

// utilsObject is UtilsContext: now(), textWidth(text, fontSize?), textSize(text, fontSize?).
func (j *jsContext) utilsObject(measure func(string, float64) model.DrawableSize) goja.Value {
	textSize := func(call goja.FunctionCall) (float64, float64) {
		fontSize := 18.0
		if f, ok := call.Argument(1).Export().(float64); ok {
			fontSize = f
		} else if i, ok := call.Argument(1).Export().(int64); ok {
			fontSize = float64(i)
		}
		size := measure(call.Argument(0).String(), fontSize)
		return size.Width + 6, size.Height + 4
	}
	obj := j.rt.NewObject()
	_ = obj.Set("now", func(goja.FunctionCall) goja.Value {
		return j.rt.ToValue(float64(time.Now().UnixMilli()) / 1000)
	})
	_ = obj.Set("textWidth", func(call goja.FunctionCall) goja.Value {
		w, _ := textSize(call)
		return j.rt.ToValue(w)
	})
	_ = obj.Set("textSize", func(call goja.FunctionCall) goja.Value {
		w, h := textSize(call)
		return j.rt.NewArray(w, h)
	})
	return obj
}

type itemContext struct {
	item       *model.DiagramItem
	itemObject *dict // To be used from references
	hasExpr    bool
	parent     *elementContext
	evaluated  map[*tenn.Token]any
}

func newItemContext(parent *elementContext, item *model.DiagramItem) *itemContext {
	return &itemContext{item: item, parent: parent, itemObject: &dict{}, evaluated: map[*tenn.Token]any{}}
}

func (ic *itemContext) updateContext() bool {
	newItems := &dict{}
	newEvaluated := map[*tenn.Token]any{}

	var drawable *model.DrawableSize
	if size := ic.parent.ctx.itemSize; size != nil {
		drawable = size(ic.item, roundView{ic.parent})
	}
	ic.hasExpr = ic.updateGetContext(nil, newItems, newEvaluated, drawable)

	// Check if we had value changes
	result := checkChanges(ic.itemObject, newItems)

	ic.itemObject = newItems
	ic.evaluated = newEvaluated

	return result
}

// jsToString is JSValue.toString(); a throwing toString gives "".
func jsToString(v goja.Value) (s string) {
	if v == nil {
		return "undefined"
	}
	defer func() {
		if recover() != nil {
			s = ""
		}
	}()
	return v.String()
}

// checkChanges compares only String, Int and jsValue entries; Doubles, arrays, blocks and pos are not (as in Swift).
func checkChanges(oldItems, newItems *dict) bool {
	if oldItems.len() != newItems.len() {
		return true
	}
	for _, k := range oldItems.keys {
		v := oldItems.m[k]
		nk, ok := newItems.get(k)
		if !ok {
			// Not pressent, we had changed.
			return true
		}
		switch n := nk.(type) {
		case jsValue:
			if o, ok := v.(jsValue); ok && jsToString(n.v) != jsToString(o.v) {
				return true
			}
		case string:
			if o, ok := v.(string); ok && o != n {
				return true
			}
		case int64:
			if o, ok := v.(int64); ok && o != n {
				return true
			}
		}
	}
	return false
}

func (ic *itemContext) getRelativeItems(source, target bool) []any {
	var result []any
	for _, itm := range ic.parent.element.GetRelatedItems(ic.item, source, target) {
		if itm.ID == ic.item.ID || itm.Kind != model.ItemKindLink || !itm.IsLink() {
			continue
		}
		opposite := itm.Source
		if itm.Source == ic.item {
			opposite = itm.Target
		}
		if opposite != nil {
			if ictx, ok := ic.parent.itemsMap.get(opposite); ok {
				result = append(result, ictx.itemObject)
			}
		}
	}
	return result
}

// jsArray turns Swift containers into a fresh JS array.
func (j *jsContext) jsArray(items []any) goja.Value {
	vals := make([]any, len(items))
	for i, e := range items {
		vals[i] = j.toJS(e)
	}
	return j.rt.NewArray(vals...)
}

func (ic *itemContext) registerSum() {
	js := ic.parent.js
	js.set("sum", js.rt.ToValue(func(call goja.FunctionCall) goja.Value {
		tag := call.Argument(0).String()
		// Swift only reads a String second argument; anything else means the tag itself.
		field := tag
		if s, ok := call.Argument(1).Export().(string); ok && s != "" {
			field = s
		}
		result := 0.0
		for _, k := range ic.parent.itemsMap.keys {
			e := ic.parent.itemsMap.m[k]
			if _, has := e.itemObject.get(tag); !has {
				continue
			}
			vv, _ := e.itemObject.get(field)
			switch x := vv.(type) {
			case int64:
				result += float64(x)
			case swiftDouble:
				result += float64(x)
			case jsValue:
				if x.v == nil {
					result += math.NaN()
				} else {
					result += x.v.ToFloat()
				}
			}
		}
		return js.rt.ToValue(result)
	}))
}

func (ic *itemContext) registerByTag() {
	js := ic.parent.js
	js.set("byTag", js.rt.ToValue(func(call goja.FunctionCall) goja.Value {
		tag := call.Argument(0).String()
		// Make it in right order every time.
		var result []any
		for _, e := range ic.parent.element.Items {
			if c, ok := ic.parent.itemsMap.get(e); ok {
				if _, has := c.itemObject.get(tag); has {
					result = append(result, c.itemObject)
				}
			}
		}
		return js.jsArray(result)
	}))
}

func (ic *itemContext) registerInputsOutputs() {
	js := ic.parent.js
	// Edge operations
	related := func(source, target bool) goja.Value {
		return js.rt.ToValue(func(goja.FunctionCall) goja.Value {
			return js.jsArray(ic.getRelativeItems(source, target))
		})
	}
	js.set("edges", related(true, true))
	js.set("inputs", related(false, true))
	js.set("outputs", related(true, false))
}

// itemsProxy is the `items` object: a property read finds the item by its converted name.
type itemsProxy struct {
	ic *itemContext
	js *jsContext
}

func (p itemsProxy) Get(key string) goja.Value {
	if itm, ok := p.ic.parent.namedItems[key]; ok {
		return p.js.toJS(itm.itemObject)
	}
	return nil
}
func (itemsProxy) Set(string, goja.Value) bool { return false }
func (itemsProxy) Has(string) bool             { return false }
func (itemsProxy) Delete(string) bool          { return false }
func (itemsProxy) Keys() []string              { return nil }

func (ic *itemContext) registerItems() {
	js := ic.parent.js
	proxy := itemsProxy{ic, js}
	js.set("__valueForKey", js.rt.ToValue(func(call goja.FunctionCall) goja.Value {
		if v := proxy.Get(call.Argument(1).String()); v != nil {
			return v
		}
		return goja.Undefined()
	}))
	js.set("items", js.rt.NewDynamicObject(proxy))
}

func (ic *itemContext) cleanContext() {
	// We need to set old values to be empty
	for _, k := range ic.itemObject.keys {
		ic.parent.js.deleteName(k)
	}
}

// updateGetContext fills the scope of this item and evaluates node (the item's properties when nil).
func (ic *itemContext) updateGetContext(node *tenn.Node, newItems *dict, newEvaluated map[*tenn.Token]any, drawable *model.DrawableSize) bool {
	ic.cleanContext()

	ec := ic.parent
	js := ec.js
	ic.registerItems()

	itemsOfKind := func(kind model.ItemKind) func(goja.FunctionCall) goja.Value {
		return func(goja.FunctionCall) goja.Value {
			var result []any
			for _, e := range ec.element.Items {
				if ic, ok := ec.itemsMap.get(e); ok && e.Kind == kind {
					result = append(result, ic.itemObject)
				}
			}
			return js.jsArray(result)
		}
	}
	parentObj := js.rt.NewObject()
	_ = parentObj.DefineAccessorProperty("items", js.rt.ToValue(itemsOfKind(model.ItemKindItem)), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
	_ = parentObj.DefineAccessorProperty("links", js.rt.ToValue(itemsOfKind(model.ItemKindLink)), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
	js.set("parent", parentObj)
	js.set("utils", ec.utils)

	// Update position
	posObj := position{ic.item.X, ic.item.Y}
	js.set("pos", js.toJS(posObj))

	if drawable != nil {
		width := math.Round(drawable.Width)
		height := math.Round(drawable.Height)
		js.set("defaults", js.frozen(map[string]float64{"width": width, "height": height}, "width", "height"))
		js.set("width", js.rt.ToValue(width))
		js.set("height", js.rt.ToValue(height))

		newItems.set("width", swiftDouble(width))
		newItems.set("height", swiftDouble(height))
	}

	// Update name
	js.set("name", js.rt.ToValue(ic.item.Name))
	js.set("kind", js.rt.ToValue(model.ItemCommandName(ic.item.Kind)))
	js.set("id", js.rt.ToValue(strings.ToUpper(ic.item.ID))) // Swift UUID.uuidString
	newItems.set("name", ic.item.Name)
	newItems.set("pos", posObj)

	ic.registerInputsOutputs()

	ic.registerByTag()
	ic.registerSum()

	if node == nil {
		node = ic.item.Properties.AsNode()
	}
	result := ec.processBlock(node, newItems, newEvaluated)

	// Cleanup most of context
	ic.cleanContext()
	return result
}

type elementContext struct {
	element       *model.Element
	elementObject *dict
	ctx           *ExecutionContext
	js            *jsContext
	utils         goja.Value
	hasExpr       bool
	evaluated     map[*tenn.Token]any
	itemsMap      orderedMap[*model.DiagramItem, *itemContext]
	namedItems    map[string]*itemContext
}

// reCalculate re-evaluates items with expressions until nothing changes; it returns the items that changed.
func (ec *elementContext) reCalculate(withExprs []*itemContext) []*model.DiagramItem {
	iterations := 100
	var changes []*model.DiagramItem
	// TODO(Swift original): smarter cycle detection; a cycle just stops after 100 iterations.
	for iterations > 0 && len(withExprs) > 0 {
		changed := 0
		for _, ic := range withExprs {
			if ic.hasExpr && ic.updateContext() {
				if !slices.Contains(changes, ic.item) {
					changes = append(changes, ic.item)
				}
				changed++
			}
		}
		if changed == 0 {
			break
		}
		iterations--
	}
	return changes
}

func newElementContext(ctx *ExecutionContext, element *model.Element) *elementContext {
	ec := &elementContext{
		element:       element,
		elementObject: &dict{},
		ctx:           ctx,
		js:            newJSContext(),
		evaluated:     map[*tenn.Token]any{},
		namedItems:    map[string]*itemContext{},
	}
	defer ec.js.startRound(ctx.roundLimit)()
	ec.utils = ec.js.utilsObject(ctx.measureText)

	ec.updateContext(nil)

	var withExprs []*itemContext
	for _, itm := range element.Items {
		ic := newItemContext(ec, itm)
		ic.updateContext()
		ec.itemsMap.set(itm, ic)
		if ic.hasExpr {
			withExprs = append(withExprs, ic)
		}
		ec.namedItems[nameOf(itm.Name)] = ic
	}
	if ec.hasExpr {
		ec.updateContext(nil)
	}

	ec.reCalculate(withExprs)
	return ec
}

func (ec *elementContext) updateGetContext(node *tenn.Node, newItems *dict, newEvaluated map[*tenn.Token]any) bool {
	// We need to set old values to be empty
	for _, k := range ec.elementObject.keys {
		ec.js.deleteName(k)
	}

	ec.js.set("utils", ec.utils)

	if node == nil {
		node = ec.element.Properties.AsNode()
	}
	return ec.processBlock(node, newItems, newEvaluated)
}

func (ec *elementContext) updateContext(node *tenn.Node) {
	newItems := &dict{}
	newEvaluated := map[*tenn.Token]any{}

	ec.hasExpr = ec.updateGetContext(node, newItems, newEvaluated)

	ec.elementObject = newItems
	ec.evaluated = newEvaluated
}

func (ec *elementContext) processEvent(event *model.ModelEvent) {
	needUpdateNamed := false
	for _, k := range slices.Clone(event.Items.Keys()) {
		op, _ := event.Items.Get(k)
		switch op {
		case model.EventAppend:
			// New item we need to add it to calculation
			ikc := newItemContext(ec, k)
			ikc.updateContext()
			ec.itemsMap.set(k, ikc)
			needUpdateNamed = true
		case model.EventRemove:
			if ec.itemsMap.delete(k) {
				delete(ec.namedItems, nameOf(k.Name))
			}
			needUpdateNamed = true
		case model.EventUpdate:
			if ic, ok := ec.itemsMap.get(k); ok {
				ic.updateContext()
			}
			// Probable we need to update list of named items.
			needUpdateNamed = true
		}
	}

	if needUpdateNamed {
		clear(ec.namedItems)
		for _, itm := range ec.element.Items {
			if itmVal, ok := ec.itemsMap.get(itm); ok {
				ec.namedItems[nameOf(itm.Name)] = itmVal
			}
		}
	}

	var withExprs []*itemContext
	for _, k := range ec.itemsMap.keys {
		if ic := ec.itemsMap.m[k]; ic.hasExpr {
			withExprs = append(withExprs, ic)
		}
	}

	for _, c := range ec.reCalculate(withExprs) {
		if _, ok := event.Items.Get(c); !ok {
			event.Items.Set(c, model.EventUpdate)
		}
	}
}

// ExecutionContext evaluates the expressions of one root element. One mutex, mu, guards every goja runtime and all
// state below (the Swift syncQueue); public methods take it and nothing inside takes it again, so JS callbacks
// (sum, items, ...) read the state directly. Values handed out are plain Go values, see values.go.
type ExecutionContext struct {
	mu          sync.Mutex
	root        *elementContext
	scaleFactor float64
	noEvaluate  bool
	roundLimit  time.Duration
	itemSize    func(item *model.DiagramItem, prev model.IExecutionContext) *model.DrawableSize
	measureText func(text string, fontSize float64) model.DrawableSize
}

// roundView is what ItemSize gets: GetEvaluated without the lock. ItemSize runs on the goroutine that holds
// ExecutionContext.mu, between two evaluations, and evaluated maps are replaced as a whole and never written
// after they are stored, so reading needs no lock. A node would run JS inside JS (MeasureText) or inside a
// half-built scope, so it gives an empty result.
type roundView struct{ ec *elementContext }

func (roundView) SetScaleFactor(float64)          {}
func (roundView) SetElement(*model.Element)       {}
func (roundView) UpdateAll(func())                {}
func (roundView) NotifyChanges(*model.ModelEvent) {}

func (v roundView) GetEvaluated(target model.EvalTarget, node *tenn.Node, _ *model.DrawableSize) map[*tenn.Token]any {
	empty := map[*tenn.Token]any{}
	if node != nil {
		return empty
	}
	switch t := target.(type) {
	case *model.DiagramItem:
		if ic, ok := v.ec.itemsMap.get(t); ok {
			return ic.evaluated
		}
	case *model.Element:
		if v.ec.element == t {
			return v.ec.evaluated
		}
	}
	return empty
}

func NewExecutionContext(opts Options) *ExecutionContext {
	c := &ExecutionContext{scaleFactor: 1, noEvaluate: opts.NoEvaluate, roundLimit: opts.RoundLimit, itemSize: opts.ItemSize, measureText: opts.MeasureText}
	if c.measureText == nil {
		c.measureText = estimateText
	}
	return c
}

func (c *ExecutionContext) SetScaleFactor(factor float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.scaleFactor = factor
}

func (c *ExecutionContext) SetElement(element *model.Element) {
	if c.noEvaluate {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	// root is assigned after construction, so GetEvaluated is empty while the context builds (as in Swift).
	c.root = newElementContext(c, element)
}

// UpdateAll is synchronous here (Swift hops to a utility queue); notifier runs after the lock is released,
// so it may call GetEvaluated.
func (c *ExecutionContext) UpdateAll(notifier func()) {
	c.mu.Lock()
	root := c.root
	if c.noEvaluate || root == nil {
		c.mu.Unlock()
		return
	}
	end := root.js.startRound(c.roundLimit)
	root.updateContext(nil)
	// We need to recalculate all stuff
	for _, k := range root.itemsMap.keys {
		if ic := root.itemsMap.m[k]; ic.hasExpr {
			ic.updateContext()
		}
	}
	end()
	c.mu.Unlock()
	if notifier != nil {
		notifier()
	}
}

func (c *ExecutionContext) NotifyChanges(event *model.ModelEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.root != nil && event.Element == c.root.element {
		defer c.root.js.startRound(c.roundLimit)()
		c.root.processEvent(event)
	}
}

// GetEvaluated returns the value per token. With node it re-evaluates that node in the item/element scope
// (editor preview) and returns only its values. The map is never modified after it is returned: callers must not write to it.
func (c *ExecutionContext) GetEvaluated(target model.EvalTarget, node *tenn.Node, drawable *model.DrawableSize) map[*tenn.Token]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	empty := map[*tenn.Token]any{}
	root := c.root
	if root == nil {
		return empty
	}
	switch t := target.(type) {
	case *model.DiagramItem:
		ic, ok := root.itemsMap.get(t)
		if !ok {
			return empty
		}
		if node == nil {
			return ic.evaluated
		}
		defer root.js.startRound(c.roundLimit)()
		value := map[*tenn.Token]any{}
		ic.updateGetContext(node, &dict{}, value, drawable)
		return value
	case *model.Element:
		if root.element != t {
			return empty
		}
		if node == nil {
			return root.evaluated
		}
		defer root.js.startRound(c.roundLimit)()
		value := map[*tenn.Token]any{}
		root.updateGetContext(node, &dict{}, value)
		return value
	}
	return empty
}
