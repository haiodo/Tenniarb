// Package jsval reads a value of the expression engine as text and as a number, with the rules of
// JavaScript (JSValue.toString / toDouble): the editor and the renderer both need them, and neither owns
// the other.
package jsval

import (
	"math"
	"strconv"
	"strings"

	"github.com/haiodo/tenniarb/go/exec"
)

// Package jsval reads the values an expression engine returns the way JavaScript prints them.
func String(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return number(x)
	case float32:
		return number(float64(x))
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			if _, undef := e.(exec.UndefinedType); e != nil && !undef {
				parts[i] = String(e)
			}
		}
		return strings.Join(parts, ",")
	}
	// undefined, Error, Function, objects: what the engine exports.
	return exec.ToString(v)
}

func number(f float64) string {
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
		// Go writes e+21 and e-07, JS e+21 and e-7.
		mant, exp, _ := strings.Cut(s, "e")
		sign := exp[:1]
		exp = strings.TrimLeft(exp[1:], "0")
		return mant + "e" + sign + exp
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}
