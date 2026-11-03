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
		if rot, _, _ := animValues(spin, c.at, false); math.Abs(rot-c.want) > 1e-6 {
			t.Errorf("rotate at %v = %v, want %v", c.at, rot, c.want)
		}
	}

	// a finite animation keeps its last frame
	once := *spin
	once.Repeat, once.Duration = 1, time.Second
	if rot, _, _ := animValues(&once, 10*time.Second, false); math.Abs(rot-360) > 1e-6 {
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
	if _, scale, opacity := animValues(fade, 500*time.Millisecond, false); math.Abs(scale-1.5) > 1e-6 || math.Abs(opacity-0.6) > 1e-6 {
		t.Errorf("half way scale %v opacity %v, want 1.5 and 0.6", scale, opacity)
	}
}

// TestAnimLoop: the editor plays a finite animation again, while the export keeps its repeat.
func TestAnimLoop(t *testing.T) {
	fade := &scene.Animation{
		Duration: time.Second,
		Easing:   "linear",
		Repeat:   1, // once, as written
		Frames: []scene.Frame{
			frame(0, map[string]string{"opacity": "0"}),
			frame(1, map[string]string{"opacity": "1"}),
		},
	}
	// as written: the last frame stays after a second
	if _, _, opacity := animValues(fade, 2500*time.Millisecond, false); math.Abs(opacity-1) > 1e-6 {
		t.Errorf("after the end opacity = %v, want 1", opacity)
	}
	// in the editor: it starts again, so a quarter into the second turn is a quarter of the fade
	if _, _, opacity := animValues(fade, 1250*time.Millisecond, true); math.Abs(opacity-0.25) > 1e-6 {
		t.Errorf("looped opacity = %v, want 0.25", opacity)
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

// animScreen builds the element A of src into a scene and returns the scene with the screen.
func animScreen(t *testing.T, src string) (*gfx.Scene, *scene.Screen) {
	t.Helper()
	tree, err := scene.LoadSource(t.TempDir(), "a.tenn", src)
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.Build(tx, nil, "A", scene.Options{})
	})
	if err != nil {
		t.Fatal(err)
	}
	sc.Commit()
	return sc, screen
}

// animTurn is the element A with body as its items, playing turn forever.
func animTurn(body string) string {
	return `element A {
	animation turn {
		from { rotate 0 }
		to { rotate 360 }
	}
	` + body + `
}
`
}

// turn applies the transform of the turn, deg degrees and scale, to the only animated node of the screen
// and returns the center of what it draws, in scene coordinates.
func turn(t *testing.T, sc *gfx.Scene, screen *scene.Screen, deg, scale float64) gfx.Point {
	t.Helper()
	nodes := screen.AnimatedNodes()
	if len(nodes) != 1 {
		t.Fatalf("%d animated nodes, want 1", len(nodes))
	}
	a := nodes[0]
	sc.Update(func(tx *gfx.Tx) { tx.SetTransform(a.Node, animMatrix(a, deg, scale)) })
	sc.Commit()
	pic := a.Node.Picture()
	if pic == nil {
		return gfx.Point{X: a.Box.X + a.Box.W/2, Y: a.Box.Y + a.Box.H/2}
	}
	return a.Node.World().Bounds(pic.Bounds).Center()
}

// TestAnimTurn: a turn keeps the center of what the item draws where the layout put it and turns its
// corners, the same for an item at the top level and one inside an auto-layout.
func TestAnimTurn(t *testing.T) {
	for _, c := range []struct {
		name, body string
		nested     bool
	}{
		{"top level", `item dot { width 24; height 8; color #ffffff; pos 40 30; animation turn 1s linear infinite }`, false},
		{"auto-layout", `item row { auto-layout row; gap 8; padding 16
		item dot { width 24; height 8; color #ffffff; animation turn 1s linear infinite }
	}`, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			sc, screen := animScreen(t, animTurn(c.body))
			a := screen.AnimatedNodes()[0]
			if c.nested && (a.Box.X == 0 || a.Box.Y == 0) {
				t.Fatalf("box %+v: the item is not laid out inside its parent", a.Box)
			}
			rest := turn(t, sc, screen, 0, 1)
			want := gfx.Point{X: a.Box.X + a.Box.W/2, Y: a.Box.Y + a.Box.H/2}
			if dist(rest, want) > 1e-9 {
				t.Errorf("at rest the picture is centered at %v, the box at %v", rest, want)
			}
			// 90 degrees: the center holds, the bounding box of the box turns from 24x8 to 8x24
			turned := turn(t, sc, screen, 90, 1)
			if dist(turned, rest) > 1e-9 {
				t.Errorf("turned by 90 the center is at %v, at rest %v", turned, rest)
			}
			m := animMatrix(a, 90, 1)
			local := gfx.R(0, 0, a.Box.W, a.Box.H)
			if r := m.Bounds(local); math.Abs(r.W()-a.Box.H) > 1e-9 || math.Abs(r.H()-a.Box.W) > 1e-9 {
				t.Errorf("turned by 90 the box is %vx%v, want %vx%v", r.W(), r.H(), a.Box.H, a.Box.W)
			}
			// scaled by 2: the center holds, the box doubles
			grown := turn(t, sc, screen, 0, 2)
			if dist(grown, rest) > 1e-9 {
				t.Errorf("scaled by 2 the center is at %v, at rest %v", grown, rest)
			}
			if r := animMatrix(a, 0, 2).Bounds(local); math.Abs(r.W()-2*a.Box.W) > 1e-9 {
				t.Errorf("scaled by 2 the box is %v wide, want %v", r.W(), 2*a.Box.W)
			}
			// at rest the transform is the placement the build gave: the still picture does not move
			placed := gfx.TranslateBy(a.Box.X, a.Box.Y)
			if m := animMatrix(a, 0, 1); m != placed {
				t.Errorf("at rest the transform is %v, the layout placed the node at %v", m, placed)
			}
		})
	}
}

// TestAnimTurnOffCenter: an icon whose ink sits off center in its box turns about its ink, as the fill
// box of the export does, so its picture stays where it was instead of swinging around the box.
func TestAnimTurnOffCenter(t *testing.T) {
	sc, screen := animScreen(t, animTurn(`item row { auto-layout row; padding 16
		item dot {
			width 40; height 40; pos 8 8; color #ffffff
			path "M0 0A10 10 0 0 1 20 0A10 10 0 0 1 0 0Z"; view-box 0 0 40 40
			animation turn 1s linear infinite
		}
	}`))
	a := screen.AnimatedNodes()[0]
	ink := a.Node.Picture().Bounds.Center()
	if dist(ink, gfx.Point{X: a.Box.W / 2, Y: a.Box.H / 2}) < 1 {
		t.Fatalf("the ink of the icon is centered in its box, the test needs it off center: %v", ink)
	}
	rest := turn(t, sc, screen, 0, 1)
	if got := turn(t, sc, screen, 90, 1); dist(got, rest) > 1e-6 {
		t.Errorf("turned by 90 the ink is centered at %v, at rest %v: it swung around the box", got, rest)
	}
}

// dist is the distance between two points.
func dist(a, b gfx.Point) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

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
				rot, scale, opacity := animValues(a, now, true)
				m := animMatrix(nodes[0], rot, scale)
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
