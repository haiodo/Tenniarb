//go:build darwin || linux || windows

package app

import (
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/tenn"
)

// frame is a keyframe at offset with the properties name value.
func frame(offset float64, props map[string]string) scene.Frame {
	f := scene.Frame{Offset: offset}
	for _, name := range []string{"rotate", "scale", "opacity"} {
		v, ok := props[name]
		if !ok {
			continue
		}
		f.Props = append(f.Props, &scene.Prop{Name: name, Val: []string{v},
			Args: []*tenn.Node{{Kind: tenn.KindIntLit, Token: &tenn.Token{Literal: v}}}})
	}
	return f
}

// TestAnimValues: the rotate, the scale and the opacity of an animation at a moment, from its frames.
func TestAnimValues(t *testing.T) {
	spin := &scene.Animation{
		Duration: 2 * time.Second,
		Easing:   "linear",
		Repeat:   -1, // forever
		Frames: []scene.Frame{
			frame(0, map[string]string{"rotate": "0"}),
			frame(1, map[string]string{"rotate": "360"}),
		},
	}
	for _, c := range []struct {
		at   time.Duration
		want float64
	}{
		{0, 0},
		{time.Second, 180},
		{1500 * time.Millisecond, 270},
		{2 * time.Second, 0},   // the loop starts again
		{3 * time.Second, 180}, // and stays in step
	} {
		if rot, _, _ := animValues(spin, c.at); math.Abs(rot-c.want) > 1e-6 {
			t.Errorf("rotate at %v = %v, want %v", c.at, rot, c.want)
		}
	}

	// a finite animation keeps its last frame
	once := *spin
	once.Repeat, once.Duration = 1, time.Second
	if rot, _, _ := animValues(&once, 10*time.Second); math.Abs(rot-360) > 1e-6 {
		t.Errorf("after the end rotate = %v, want 360", rot)
	}

	// opacity and scale interpolate too, from their defaults when a frame leaves them out
	fade := &scene.Animation{
		Duration: time.Second,
		Easing:   "linear",
		Frames: []scene.Frame{
			frame(0, map[string]string{"opacity": "1", "scale": "1"}),
			frame(1, map[string]string{"opacity": "0.2", "scale": "2"}),
		},
	}
	if _, scale, opacity := animValues(fade, 500*time.Millisecond); math.Abs(scale-1.5) > 1e-6 || math.Abs(opacity-0.6) > 1e-6 {
		t.Errorf("half way scale %v opacity %v, want 1.5 and 0.6", scale, opacity)
	}
}

// TestEase: the easing curves start at 0, end at 1 and are monotone; ease-in starts slower than linear.
func TestEase(t *testing.T) {
	for _, name := range []string{"linear", "ease", "ease-in", "ease-out", "ease-in-out"} {
		prev := -1.0
		for i := 0; i <= 10; i++ {
			v := ease(name, float64(i)/10)
			if v < prev-1e-9 || v < -1e-9 || v > 1+1e-9 {
				t.Fatalf("%s: %v after %v", name, v, prev)
			}
			prev = v
		}
		if v := ease(name, 0); math.Abs(v) > 1e-9 {
			t.Errorf("%s(0) = %v", name, v)
		}
		if v := ease(name, 1); math.Abs(v-1) > 1e-9 {
			t.Errorf("%s(1) = %v", name, v)
		}
	}
	if ease("ease-in", 0.3) >= 0.3 {
		t.Error("ease-in is not slower than linear at the start")
	}
	if ease("ease-out", 0.3) <= 0.3 {
		t.Error("ease-out is not faster than linear at the start")
	}
}

// spinner is an animation of a full turn in a second, forever.
func spinner() *scene.Animation {
	return &scene.Animation{Duration: time.Second, Easing: "linear", Repeat: -1,
		Frames: []scene.Frame{
			frame(0, map[string]string{"rotate": "0"}),
			frame(1, map[string]string{"rotate": "360"}),
		}}
}

// BenchmarkAnimFrame is the cost of one animation frame: the transform of every animated node. Its
// repaint is the cost of the scene, measured by gfx itself.
func BenchmarkAnimFrame(b *testing.B) {
	for _, n := range []int{1, 10, 100} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			sc := gfx.NewScene()
			nodes := make([]scene.Animated, n)
			sc.Update(func(tx *gfx.Tx) {
				for i := range nodes {
					node := gfx.NewNode(i + 2)
					tx.Add(nil, node)
					nodes[i] = scene.Animated{Node: node, Box: scene.Box{X: float64(i) * 10, W: 10, H: 10},
						Anim: spinner(), Opacity: 1}
				}
			})
			a := nodes[0].Anim
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				now := time.Duration(i) * 16 * time.Millisecond
				rot, scale, opacity := animValues(a, now)
				cx, cy := 5.0, 5.0
				m := gfx.TranslateBy(cx, cy)
				if rot != 0 {
					m = m.Mul(gfx.RotateBy(rot * math.Pi / 180))
				}
				if scale != 1 {
					m = m.Mul(gfx.ScaleBy(scale, scale))
				}
				m = m.Mul(gfx.TranslateBy(-cx, -cy)).Mul(gfx.TranslateBy(0, 0))
				sc.Update(func(tx *gfx.Tx) {
					for _, n := range nodes {
						tx.SetTransform(n.Node, m)
						tx.SetOpacity(n.Node, n.Opacity*opacity)
					}
				})
			}
		})
	}
}
