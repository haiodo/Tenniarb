package exec_test

import (
	"testing"

	"github.com/haiodo/tenniarb/go/exec"
)

// TestSceneEval: the value of an expression as text, and false when it throws.
func TestSceneEval(t *testing.T) {
	for _, c := range []struct {
		code string
		want string
	}{
		{"1 + 2", "3"},
		{"`Hi ${'x'}`", "Hi x"},
		{"(24 * 2) + 'px'", "48px"},
	} {
		if got, ok := exec.SceneEval(c.code); !ok || got != c.want {
			t.Errorf("SceneEval(%q) = %q, %v; want %q, true", c.code, got, ok, c.want)
		}
	}
	if got, ok := exec.SceneEval("nosuch()"); ok {
		t.Errorf("SceneEval of a throwing expression = %q, true", got)
	}
}

// TestSceneEvalVars: the parameters of a component instance are variables of the expression, a numeric
// value is a number (spec 5c), and a name that is no variable still throws.
func TestSceneEvalVars(t *testing.T) {
	vars := map[string]string{"size": "40", "who": "Bob", "kind": "primary", "code": "007"}
	for _, c := range []struct{ code, want string }{
		{"size + 12", "52"},
		{"kind", "primary"},
		{"`Hi ${who}`", "Hi Bob"},
		{"code", "007"}, // text that only looks like a number keeps its form
	} {
		if got, ok := exec.SceneEvalVars(c.code, vars); !ok || got != c.want {
			t.Errorf("SceneEvalVars(%q) = %q, %v; want %q, true", c.code, got, ok, c.want)
		}
	}
	if got, ok := exec.SceneEvalVars("nosuch", vars); ok {
		t.Errorf("SceneEvalVars of an unknown name = %q, true", got)
	}
}

// TestSceneEvalColorBranch: a branch of an expression gives back every #rrggbbaa as it is written, so the
// colour of the picture is the branch the variables choose and nothing about the digits.
func TestSceneEvalColorBranch(t *testing.T) {
	for _, c := range []string{"#2e7cc726", "#2e7cc71f", "#79797c26", "#a5bdff0d", "#ffffff17"} {
		code := `theme === "dark" ? "` + c + `" : "#ffffff"`
		if got, ok := exec.SceneEvalVars(code, map[string]string{"theme": "dark"}); !ok || got != c {
			t.Errorf("SceneEvalVars(%q) = %q, %v; want %q, true", code, got, ok, c)
		}
	}
}

