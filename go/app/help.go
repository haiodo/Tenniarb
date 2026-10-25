//go:build darwin || linux || windows

package app

import (
	_ "embed"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/swt"
	"github.com/haiodo/gowt/webview"
)

// readme is Swift's help page (Tenniarb/readme.html, a copy of docs/readme.html).
//
//go:embed help/readme.html
var readme string

type helpWindow struct {
	win *g.Window
	wv  *webview.WebView
}

// showHelp opens "Tenniarb Help": the readme in a web view, as Swift's HelpController does.
func (sh *Shell) showHelp() {
	if sh.helpWin != nil {
		sh.helpWin.win.Unwrap().ForceActive()
		return
	}
	hw := &helpWindow{}
	var host *g.Panel
	hw.win = sh.app.Window("Tenniarb Help", g.Padding(0), g.Gap(0), g.Fill(g.Grow(), g.Ref(&host)))
	hw.win.SetSize(800, 600)
	wv, err := webview.New(host, webview.Options{})
	if err != nil {
		hw.win.Close()
		if v := sh.active; v != nil {
			v.win.MessageBox(g.Message{Title: "Help", Text: "The web view is not available: " + err.Error(), Icon: g.IconError, Buttons: g.ButtonsOK})
		}
		return
	}
	hw.wv = wv
	sh.helpWin = hw
	hw.win.Unwrap().AddListener(swt.Dispose, listenerFunc(func(*swt.Event) { sh.helpWin = nil }))
	hw.win.Show()
	wv.SetHTML(readme, "")
}
