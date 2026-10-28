//go:build darwin || linux || windows

package app

import (
	"runtime"
	"slices"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenniarb/go/render"
)

type prefsWindow struct {
	win *g.Window
	bg  *g.TextView
	dbg *g.TextView
}

// showPrefs opens the settings window, or brings the open one forward.
func (sh *Shell) showPrefs() {
	if sh.prefsWin != nil {
		sh.prefsWin.win.Unwrap().ForceActive()
		return
	}
	pw := &prefsWindow{}
	sh.prefsWin = pw
	p := sh.prefs
	change := func(f func(*Prefs)) {
		n := sh.prefs
		f(&n)
		sh.setPrefs(n)
	}
	check := func(id, label string, on bool, f func(*Prefs, bool)) g.Option {
		return g.Button(label, g.Check(), g.ID(id), g.Checked(on), g.Init(func(b *g.ButtonView) {
			b.OnClick(func() { change(func(p *Prefs) { f(p, b.Checked()) }) })
		}))
	}
	color := func(id string, ref **g.TextView, val string, set func(*Prefs, string), def string) g.Option {
		return g.HBox(g.Padding(0), g.Gap(6),
			g.Text(g.ID(id), g.Value(val), g.Ref(ref), g.Size(110, 0), g.Init(func(t *g.TextView) {
				t.OnChange(func(s string) {
					if _, ok := render.ParseColor(s); ok && s != "" {
						change(func(p *Prefs) { set(p, s) })
					}
				})
			})),
			g.Button("Reset", g.ID(id+"-reset"), g.Init(func(b *g.ButtonView) {
				b.OnClick(func() { (*ref).SetText(def) })
			})))
	}
	d := DefaultPrefs()
	modes := []string{AppearanceSystem, AppearanceLight, AppearanceDark}
	appearance := g.HBox(g.Padding(0), g.Gap(6),
		g.Label("Appearance"),
		g.Combo([]string{"System", "Light", "Dark"}, g.ReadOnly(), g.ID("appearance"), g.Selected(max(0, slices.Index(modes, p.Appearance))),
			g.Init(func(c *g.ComboView) {
				c.OnSelect(func(i int) { change(func(p *Prefs) { p.Appearance = modes[i] }) })
			})))
	pw.win = sh.app.Window("Preferences", g.Padding(16), g.Gap(12),
		g.Group("Theme", append([]g.Option{g.Gap(6)}, themeOpts(appearance)...)...),
		g.Group("Outline", g.Gap(6),
			check("auto-expand", "Expand the outline when a document opens", p.AutoExpand, func(p *Prefs, v bool) { p.AutoExpand = v }),
			g.HBox(g.Padding(0), g.Gap(6),
				g.Label("Expand levels"),
				g.Spinner(0, 20, g.ID("expand-level"), g.Value(p.ExpandLevel), g.Init(func(s *g.SpinnerView) {
					s.OnChange(func(n int) { change(func(p *Prefs) { p.ExpandLevel = n }) })
				})))),
		g.Group("Canvas background",
			g.Fields(
				g.Field("Light", color("background", &pw.bg, p.Background, func(p *Prefs, s string) { p.Background = s }, d.Background)),
				g.Field("Dark", color("background-dark", &pw.dbg, p.BackgroundDark, func(p *Prefs, s string) { p.BackgroundDark = s }, d.BackgroundDark)))),
		g.Group("Export", g.Gap(6),
			check("export-background", "Render background", p.ExportBackground, func(p *Prefs, v bool) { p.ExportBackground = v }),
			check("export-native", "Use native resolution", p.ExportNativeScale, func(p *Prefs, v bool) { p.ExportNativeScale = v })),
		g.Group("UI", g.Gap(6),
			check("quick-panel", "Show popup quick panel", p.QuickPanel, func(p *Prefs, v bool) { p.QuickPanel = v })))
	pw.win.Unwrap().AddListener(swt.Dispose, listenerFunc(func(*swt.Event) { sh.prefsWin = nil }))
	pw.win.Show()
}

// themeOpts adds a note where the controls theme does not switch live (gowt SetAppearance, Windows).
func themeOpts(appearance g.Option) []g.Option {
	if runtime.GOOS != "windows" {
		return []g.Option{appearance}
	}
	return []g.Option{appearance, g.Label("Some controls change after restart")}
}
