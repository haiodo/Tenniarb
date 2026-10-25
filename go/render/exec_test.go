package render

import (
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/model"
)

// The expression engine and the renderer meet through exec.Options (ItemSize, MeasureText).

func TestExpressionsSeeItemSizeAndTextSize(t *testing.T) {
	el := parseDiagram(t, `element Root {
    item A {
        pos 0 0
        width 120
        height 50
        title ${width + "x" + height}
    }
    item B {
        pos 0 100
        title ${"" + utils.textSize("Hello", 18)[0]}
    }
}`)
	o := testOpts()
	itemSize, measureText := ExecHooks(o)
	ctx := exec.NewExecutionContext(exec.Options{ItemSize: itemSize, MeasureText: measureText})
	ctx.SetElement(el)
	o.Exec = ctx
	s := NewDrawableScene(el, o)
	if got := titleOf(s, el.Items[0]); got != "120x50" {
		t.Errorf("width/height in an expression: %q", got)
	}
	// utils.textSize adds the 6 unit padding on top of the measured frame.
	want := math.Ceil(5*18*0.6) + 6
	if got := titleOf(s, el.Items[1]); got != strconv.FormatFloat(want, 'f', -1, 64) {
		t.Errorf("utils.textSize: %q, want %v", got, want)
	}

	plain := testOpts()
	plain.Exec = exec.NewExecutionContext(exec.Options{NoEvaluate: true})
	if got := titleOf(NewDrawableScene(el, plain), el.Items[0]); got != `width + "x" + height` {
		t.Errorf("NoEvaluate renders tokens: %q", got)
	}
}

// buildWithTimeout fails the test instead of hanging when the engine and the renderer lock each other.
func buildWithTimeout(t *testing.T, el *model.Element, o Options) *DrawableScene {
	t.Helper()
	done := make(chan *DrawableScene, 1)
	go func() { done <- Build(el, o) }()
	select {
	case s := <-done:
		return s
	case <-time.After(10 * time.Second):
		t.Fatal("Build did not return: the item size hook is blocked on the engine lock")
		return nil
	}
}

const evaluatedSizeSrc = `element Root {
    item A {
        pos 0 0
        title ${"W".repeat(20)}
    }
    item B {
        pos 0 200
        title ${"" + items.A.width}
    }
}`

// The size an expression reads is the size of a drawable built from evaluated values of the round before.
func TestItemSizeSeesEvaluatedTitle(t *testing.T) {
	el := parseDiagram(t, evaluatedSizeSrc)
	s := buildWithTimeout(t, el, testOpts())
	b := s.Drawables[el.Items[0]].SelectorBounds()
	if got := titleOf(s, el.Items[1]); got != strconv.FormatFloat(math.Round(b.Width), 'f', -1, 64) {
		t.Errorf("items.A.width = %q, the drawn width is %v", got, b.Width)
	}
	// 20 evaluated characters at the 0.6 em estimate, not the longer raw token.
	if b.Width < 20*18*0.6 || b.Width > 20*18*0.6+60 {
		t.Errorf("A is %v wide: not sized from the evaluated title", b.Width)
	}
}

func TestBuildCreatesContextUnlessNoEvaluate(t *testing.T) {
	el := parseDiagram(t, evaluatedSizeSrc)
	s := buildWithTimeout(t, el, testOpts())
	if got := titleOf(s, el.Items[0]); got != "WWWWWWWWWWWWWWWWWWWW" {
		t.Errorf("evaluated title %q", got)
	}
	o := testOpts()
	o.NoEvaluate = true
	if got := titleOf(Build(el, o), el.Items[0]); got != `"W".repeat(20)` {
		t.Errorf("NoEvaluate title %q", got)
	}
}

func TestBuildRoundLimitGivesErrorValue(t *testing.T) {
	el := parseDiagram(t, `element Root {
    item A {
        pos 0 0
        title ${while(true){}}
    }
}`)
	o := testOpts()
	o.ExecLimit = 50 * time.Millisecond
	s := buildWithTimeout(t, el, o)
	if got := titleOf(s, el.Items[0]); got != "Error: Execution time limit exceeded" {
		t.Errorf("title %q", got)
	}
}
