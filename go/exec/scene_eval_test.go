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
