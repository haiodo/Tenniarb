//go:build darwin || linux || windows

package app

import (
	"fmt"
	"os/exec"
	"runtime"
	"slices"
	"strings"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenniarb/go/update"
)

// updateAPI is where the check asks for the latest release; a QA test points it at its own server.
var updateAPI = update.DefaultAPI

// updOptions is a check of the latest release with the settings of this application.
func (sh *Shell) updOptions() update.Options {
	return update.Options{Dir: sh.dir, API: updateAPI}
}

// openBrowser opens url in the browser of the system.
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

// checkUpdatesCommand is Help > Check for Updates: the request runs off the UI thread and the answer
// comes back to it, so a slow network does not freeze the window.
func (v *Viewer) checkUpdatesCommand() {
	sh := v.sh
	go func() {
		r, err := update.Latest(sh.ctx, sh.updOptions())
		v.app.Async(func() {
			sh.setUpdate(r, err)
			if v.win.Unwrap().IsDisposed() {
				return
			}
			v.showUpdate(r, err)
		})
	}()
}

// updateStatus is the last check, as the QA state reports it.
type updateStatus struct {
	Checked bool   `json:"checked"`
	Version string `json:"version"`
	Error   string `json:"error"`
}

// setUpdate keeps the result of a check for the QA state.
func (sh *Shell) setUpdate(r update.Release, err error) {
	sh.update = updateStatus{Checked: true, Version: r.Version}
	if err != nil {
		sh.update.Error = err.Error()
	}
}

// showUpdate is the answer of an asked check: an error is a line in the banner, a release that is not
// newer is a box, a newer one is the dialog with its notes.
func (v *Viewer) showUpdate(r update.Release, err error) {
	if err != nil {
		v.setBanner("Update check failed: " + err.Error())
		return
	}
	current := buildVersion()
	if update.Compare(releaseVersion(), r.Version) >= 0 {
		v.win.MessageBox(g.Message{Title: "Check for Updates",
			Text: fmt.Sprintf("You have the latest version (%s).", current), Icon: g.IconInfo, Buttons: g.ButtonsOK})
		return
	}
	v.showUpdateWindow(r, current)
}

// notesLimit cuts a long changelog: the dialog shows the start, the release page has the rest.
const notesLimit = 2000

// showUpdateWindow shows the newer release with its notes and two answers: download the file for this
// machine (or the release page when the release has none for it), or later.
func (v *Viewer) showUpdateWindow(r update.Release, current string) {
	sh := v.sh
	if w := sh.updWin; w != nil {
		w.Unwrap().ForceActive()
		return
	}
	var w *g.Window
	w = v.app.Window("Update Available", g.Padding(16), g.Gap(12),
		g.Label(fmt.Sprintf("Version %s is available. You have %s.", r.Version, current), g.Wrap()),
		g.Text(g.Value(notesOf(r.Notes)), g.Multiline(), g.ReadOnly(), g.Border(), g.ID("update-notes"),
			g.Size(g.Scaled(520), g.Scaled(220))),
		g.HBox(g.Padding(0), g.Gap(8),
			g.Button("&Later", g.ID("update-later"), g.Init(func(b *g.ButtonView) {
				b.OnClick(func() { w.Close() })
			})),
			g.Button("&Download", g.ID("update-download"), g.Init(func(b *g.ButtonView) {
				b.OnClick(func() {
					url := r.Page()
					if a, ok := update.PickAsset(r, runtime.GOOS, runtime.GOARCH); ok {
						url = a.URL
					}
					if err := openBrowser(url); err != nil {
						v.setBanner("Could not open " + url + ": " + err.Error())
					}
					w.Close()
				})
			}))))
	sh.updWin = w
	w.Unwrap().AddListener(swt.Dispose, listenerFunc(func(*swt.Event) { sh.updWin = nil }))
	w.Show()
}

// notesOf is the release notes as the dialog shows them: kept whole while they fit.
func notesOf(s string) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= notesLimit {
		return s
	}
	cut := string(r[:notesLimit])
	if i := strings.LastIndex(cut, "\n"); i > 0 {
		cut = cut[:i]
	}
	return cut + "\n…"
}

// checkUpdatesOnStart is the quiet check of a start: a stale cache goes to the network, a newer
// release is a line in the banner, and nothing is said about a network that is not there.
func (sh *Shell) checkUpdatesOnStart() {
	if !sh.prefs.CheckUpdates {
		return
	}
	go func() {
		r, err := update.Latest(sh.ctx, sh.updOptions())
		if err != nil || update.Compare(releaseVersion(), r.Version) >= 0 {
			return
		}
		note := fmt.Sprintf("Tenniarb %s is available — Help > Check for Updates…", r.Version)
		sh.app.Async(func() {
			sh.setUpdate(r, nil)
			for _, v := range slices.Clone(sh.viewers) {
				if !v.win.Unwrap().IsDisposed() && v.bannerText() == "" {
					v.setNote(note)
				}
			}
		})
	}()
}
