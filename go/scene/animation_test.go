package scene_test

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
)

// svgOfAnim builds element of animation.tenn and writes it as SVG, with the warnings of the build.
func svgOfAnim(t *testing.T, element string) (string, []string) {
	t.Helper()
	screen, sc := build(t, load(t, "animation.tenn"), element)
	var out bytes.Buffer
	if err := screen.WriteSVG(&out, sc, gfx.Rect{}, 1); err != nil {
		t.Fatal(err)
	}
	var msgs []string
	for _, e := range screen.Warnings {
		msgs = append(msgs, e.Error())
	}
	return out.String(), msgs
}

func TestAnimationKeyframesInSVG(t *testing.T) {
	svg, _ := svgOfAnim(t, "Spin")
	wantIn(t, svg,
		"@keyframes spin{from{rotate:0deg}to{rotate:360deg}}",
		"@keyframes fade{0%{opacity:1}100%{opacity:0.2}}",
		"animation:spin 1s linear infinite;transform-box:fill-box;transform-origin:center",
		"animation:fade 300ms ease-out 2;")
	// Icon names the animation itself, Tint takes it from the loading style.
	if n := strings.Count(svg, "animation:spin 1s linear infinite"); n != 2 {
		t.Errorf("animation:spin on %d groups, want 2", n)
	}
	if n := strings.Count(svg, "animation:fade "); n != 1 {
		t.Errorf("animation:fade on %d groups, want 1", n)
	}
	// Every item keeps its transform: the animation is a style on the group, not a replacement of it.
	if !strings.Contains(svg, `transform="matrix(1 0 0 1 36 8)"`) {
		t.Errorf("the group of Icon lost its place:\n%s", svg)
	}
}

// The still picture is the first frame: Dot is written red, the first frame of pulse is blue.
func TestAnimationFirstFrameIsStill(t *testing.T) {
	svg, msgs := svgOfAnim(t, "Still")
	if len(msgs) != 0 {
		t.Errorf("unexpected warnings: %v", msgs)
	}
	if n := strings.Count(svg, `fill="rgb(0,0,255)"`); n != 2 {
		t.Errorf("the first frame (blue) is drawn %d times, want 2:\n%s", n, svg)
	}
	if strings.Contains(svg, `fill="rgb(255,0,0)"`) {
		t.Errorf("the node's own red is drawn instead of the first frame:\n%s", svg)
	}
	// Faded takes opacity 0.5 from its style and plays the same first frame.
	if !strings.Contains(svg, `opacity="0.5"`) {
		t.Errorf("opacity of the style is not drawn:\n%s", svg)
	}
}

func TestAnimationTiming(t *testing.T) {
	screen, _ := build(t, load(t, "animation.tenn"), "Spin")
	var got []string
	for _, a := range screen.Animations {
		got = append(got, fmt.Sprintf("%s %v %s %d", a.Name, a.Duration, a.Easing, a.Repeat))
	}
	sort.Strings(got)
	want := []string{"fade 300ms ease-out 2", "spin 1s linear -1", "spin 1s linear -1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("animations = %v, want %v", got, want)
	}
}

func TestAnimationWarnings(t *testing.T) {
	_, msgs := svgOfAnim(t, "Warn")
	got := strings.Join(msgs, "\n")
	wantIn(t, got,
		`animation.tenn:42:9: warning: animation broken: frame "middle" is not from, to or a percentage`,
		"animation.tenn:43:16: warning: animation broken: width is a layout property and is not animated",
		"animation.tenn:43:26: warning: animation broken: shadow is not animated",
		`animation.tenn:46:35: warning: animation missing: no keyframes block with that name in the element`,
		"animation.tenn:47:35: warning: animation broken: the duration must be above zero",
		`animation.tenn:48:35: warning: animation broken: "sideways" is not an easing`,
		`animation.tenn:49:35: warning: animation broken: "-2" is not an easing`,
		"animation.tenn:52:5: warning: transition: width is a layout property and is not animated",
		`animation.tenn:51:5: warning: transition: "slow" is not a duration`,
		"animation.tenn:53:5: warning: transition takes properties, a duration and an easing",
	)
	// Load lays every element out, so `check` reports the same warnings without a render.
	tree, err := scene.Load("testdata", "animation.tenn")
	if err != nil {
		t.Fatal(err)
	}
	wantIn(t, tree.Diagnostics().Error(),
		`animation.tenn:46:35: warning: animation missing: no keyframes block with that name in the element`,
		"animation.tenn:47:35: warning: animation broken: the duration must be above zero",
		"animation.tenn:39:16: warning: the keyframes block goes in the element, not in a style")
}
