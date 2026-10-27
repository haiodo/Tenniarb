//go:build darwin || linux || windows

package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"time"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/swt"
)

// PollInterval is how often the open files are checked for changes.
const PollInterval = 500 * time.Millisecond

// Shell is the application: the documents, one window each, and what they share (settings, the
// recent list, the window frames in the config directory).
type Shell struct {
	app    *g.App
	dir    string
	exe    string
	prefs  Prefs
	recent []string

	viewers []*Viewer
	active  *Viewer
	// quitting is set while the application asks to quit: an edited untitled document is stashed
	// instead of asked about.
	quitting bool

	prefsWin *prefsWindow
	helpWin  *helpWindow
	// sysMenu is set when About and Preferences are in the macOS application menu.
	sysMenu bool

	subs    map[int]func(DocEvent) // the API's subscribers
	nextSub int

	ctx context.Context // ends when the application quits
}

// listenerFunc is a swt.Listener.
type listenerFunc func(*swt.Event)

func (f listenerFunc) HandleEvent(e *swt.Event) { f(e) }

// Run opens the application with the documents named in args and returns when the last window closes.
func Run(args []string) error { return RunIn(ConfigDir(), args) }

// RunIn is Run with the files of the application in dir.
func RunIn(dir string, args []string) error {
	g.SetAppName("Tenniarb")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return g.Run(func(app *g.App) {
		app.SetDensity(g.DensityNative)
		exe, _ := os.Executable()
		sh := &Shell{app: app, dir: dir, exe: exe, prefs: LoadPrefs(dir), recent: LoadRecent(dir), ctx: ctx}
		sh.start(args)
	})
}

func (sh *Shell) start(args []string) {
	sh.registerQA()
	prefs := sh.app.OnPreferences(sh.showPrefs)
	about := sh.app.OnAbout(sh.showAbout)
	sh.sysMenu = prefs && about
	disp := sh.app.Unwrap()
	// A filter runs before the listener that asks the windows, so OnClose already knows it is a quit.
	// Filters see the Close of every widget; the one of the application has no widget.
	disp.AddFilter(swt.Close, listenerFunc(func(e *swt.Event) {
		if e.Widget != nil {
			return
		}
		sh.quitting = true
		sh.app.Async(func() { sh.quitting = false })
	}))
	disp.AddListener(swt.OpenDocument, listenerFunc(func(e *swt.Event) {
		if e.Text != "" {
			sh.openPath(e.Text)
		}
	}))
	sh.app.Every(PollInterval, func() {
		for _, v := range slices.Clone(sh.viewers) {
			v.poll()
		}
	})
	sh.app.OnThemeChange(func(bool) {
		for _, v := range sh.viewers {
			v.themeChanged()
		}
	})

	sh.startProvider(sh.ctx)
	for _, a := range args {
		sh.openPath(a)
	}
	for _, id := range StashedUntitled(sh.dir) {
		if text, err := ReadUntitled(sh.dir, id); err == nil {
			sh.newViewer(viewerInit{id: id, text: text, stashed: true})
		}
	}
	if len(sh.viewers) == 0 {
		sh.newDocument()
	}
}

// newDocument is File > New.
func (sh *Shell) newDocument() *Viewer {
	return sh.newViewer(viewerInit{id: NewUntitledID(), text: NewDocumentText})
}

// openPath opens the file in a window: the one already showing it, the untitled window nothing was
// done in (as AppKit does), or a new one.
func (sh *Shell) openPath(path string) {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	for _, v := range sh.viewers {
		if v.state.Doc.Path == path {
			v.win.Unwrap().ForceActive()
			return
		}
	}
	if a := sh.active; a != nil && a.state.Doc.Path == "" && !a.isDirty() {
		a.openPath(path)
		return
	}
	sh.newViewer(viewerInit{path: path})
}

// addRecent remembers path in Open Recent.
func (sh *Shell) addRecent(path string) {
	list, err := AddRecent(sh.dir, path)
	if err != nil {
		list = PushRecent(sh.recent, path)
	}
	sh.setRecent(list)
}

func (sh *Shell) clearRecent() {
	_ = SaveRecent(sh.dir, nil)
	sh.setRecent(nil)
}

func (sh *Shell) setRecent(list []string) {
	sh.recent = list
	// The item that was chosen may be the one being replaced.
	sh.app.Async(func() {
		for _, v := range sh.viewers {
			v.fillRecent()
		}
	})
}

// setPrefs stores new settings and applies them to every window.
func (sh *Shell) setPrefs(p Prefs) {
	sh.prefs = p.sane()
	_ = sh.prefs.Save(sh.dir)
	for _, v := range sh.viewers {
		v.prefsChanged()
	}
}

func (sh *Shell) remove(v *Viewer) {
	sh.viewers = slices.DeleteFunc(sh.viewers, func(o *Viewer) bool { return o == v })
	sh.publish(DocEvent{Path: v.state.Doc.Path, Kind: EventClose, Version: v.version})
	if len(sh.viewers) == 0 { // as Swift: the application ends with the last document
		for _, w := range []*g.Window{sh.prefsWin.window(), sh.helpWin.window()} {
			if w != nil {
				w.Unwrap().Dispose()
			}
		}
	}
	if sh.active == v {
		sh.active = nil
		if n := len(sh.viewers); n > 0 {
			sh.active = sh.viewers[n-1]
		}
	}
}

func (p *prefsWindow) window() *g.Window {
	if p == nil {
		return nil
	}
	return p.win
}

func (h *helpWindow) window() *g.Window {
	if h == nil {
		return nil
	}
	return h.win
}

// Version is set by the packaging targets: -ldflags "-X github.com/haiodo/tenniarb/go/app.Version=...".
var Version string

// buildVersion is the module version, or the commit of a build from a checkout.
func buildVersion() string {
	if Version != "" {
		return Version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && len(s.Value) >= 7 {
			return s.Value[:7]
		}
	}
	return "dev"
}

// showAbout is the About panel; the text is what Swift's standard panel shows from Info.plist.
func (sh *Shell) showAbout() {
	v := sh.active
	if v == nil {
		return
	}
	v.win.MessageBox(g.Message{
		Title:   "About Tenniarb",
		Text:    fmt.Sprintf("Tenniarb\nVersion %s\nCopyright \u00a9 2026 Andrey Sobolev", buildVersion()),
		Icon:    g.IconInfo,
		Buttons: g.ButtonsOK,
	})
}
