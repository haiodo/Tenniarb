package exec

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/dop251/goja"
)

// Go values evaluated expressions are exported as, so callers never touch a goja value outside the lock.
//   JS number -> float64, string -> string, boolean -> bool, null -> nil, undefined -> Undefined,
//   array -> []any, plain object -> map[string]any (own enumerable keys; order is lost),
//   function -> Function, Error object -> Error. A thrown value is stored as is, like in TS.

type UndefinedType struct{}

// Undefined is the JS undefined (nil is null).
var Undefined = UndefinedType{}

// Function stands for any JS function; its source is not kept.
type Function struct{}

// Error is a JS Error object (TS: `instanceof Error`). Messages are engine specific, compare Name only.
type Error struct {
	Name    string
	Message string
}

// maxExportDepth cuts runaway nesting; cycles are cut separately.
const maxExportDepth = 16

func exportValue(v goja.Value) any {
	return exportDepth(v, 0, map[*goja.Object]bool{})
}

func exportDepth(v goja.Value, depth int, active map[*goja.Object]bool) any {
	if v == nil || goja.IsUndefined(v) {
		return Undefined
	}
	if goja.IsNull(v) {
		return nil
	}
	obj, isObj := v.(*goja.Object)
	if !isObj {
		switch x := v.Export().(type) {
		case bool:
			return x
		case string:
			return x
		case int64:
			return float64(x)
		case float64:
			return x
		}
		return v.String() // symbol, bigint
	}
	if _, ok := goja.AssertFunction(obj); ok {
		return Function{}
	}
	if obj.ClassName() == "Error" {
		return Error{Name: obj.Get("name").String(), Message: obj.Get("message").String()}
	}
	if depth >= maxExportDepth || active[obj] {
		return nil
	}
	active[obj] = true
	defer delete(active, obj)
	if obj.ClassName() == "Array" {
		n := int(obj.Get("length").ToInteger())
		arr := make([]any, n)
		for i := range arr {
			arr[i] = exportDepth(obj.Get(strconv.Itoa(i)), depth+1, active)
		}
		return arr
	}
	m := map[string]any{}
	for _, k := range obj.Keys() {
		m[k] = exportDepth(obj.Get(k), depth+1, active)
	}
	return m
}

// ToString is JS String(v) for an exported value (JSValue.toString() in Swift). A function has no source here.
func ToString(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case UndefinedType:
		return "undefined"
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return jsNumberString(x)
	case string:
		return x
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			if _, undef := e.(UndefinedType); e != nil && !undef {
				parts[i] = ToString(e)
			}
		}
		return strings.Join(parts, ",")
	case Error:
		if x.Message == "" {
			return x.Name
		}
		return x.Name + ": " + x.Message
	case Function:
		return "function () { [native code] }"
	}
	return "[object Object]"
}

// ToFloat is JS Number(v) for an exported value (JSValue.toDouble() in Swift).
func ToFloat(v any) float64 {
	switch x := v.(type) {
	case nil:
		return 0
	case bool:
		if x {
			return 1
		}
		return 0
	case float64:
		return x
	case string:
		return jsStringToNumber(x)
	case []any:
		return jsStringToNumber(ToString(x))
	}
	return math.NaN()
}

// jsNumberString is Number::toString of ECMAScript.
func jsNumberString(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case f == 0:
		return "0"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	}
	sign := ""
	if f < 0 {
		sign, f = "-", -f
	}
	mant, expStr, _ := strings.Cut(strconv.FormatFloat(f, 'e', -1, 64), "e")
	digits := strings.Replace(mant, ".", "", 1)
	e, _ := strconv.Atoi(expStr)
	k, n := len(digits), e+1
	switch {
	case k <= n && n <= 21:
		return sign + digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		return sign + digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		return sign + "0." + strings.Repeat("0", -n) + digits
	}
	expSign := "+"
	if n-1 < 0 {
		expSign = "-"
	}
	exp := strconv.Itoa(max(n-1, 1-n))
	if k == 1 {
		return sign + digits + "e" + expSign + exp
	}
	return sign + digits[:1] + "." + digits[1:] + "e" + expSign + exp
}

var decimalRe = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)

// jsStringToNumber is StringToNumber of ECMAScript; NaN when the text is not a number.
func jsStringToNumber(s string) float64 {
	s = strings.TrimFunc(s, func(r rune) bool { return unicode.IsSpace(r) || r == 0xFEFF })
	if s == "" {
		return 0
	}
	switch s {
	case "Infinity", "+Infinity":
		return math.Inf(1)
	case "-Infinity":
		return math.Inf(-1)
	}
	if len(s) > 2 && s[0] == '0' {
		base := map[byte]int{'x': 16, 'X': 16, 'o': 8, 'O': 8, 'b': 2, 'B': 2}[s[1]]
		if base != 0 {
			if n, err := strconv.ParseUint(s[2:], base, 64); err == nil {
				return float64(n)
			}
			return math.NaN()
		}
	}
	if !decimalRe.MatchString(s) {
		return math.NaN()
	}
	f, _ := strconv.ParseFloat(s, 64) // ErrRange still returns ±Inf, as JS
	return f
}
