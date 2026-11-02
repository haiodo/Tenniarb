//go:build darwin || linux || windows

package app

import (
	"errors"
	"fmt"
	"strings"

	g "github.com/haiodo/gowt"

	"github.com/haiodo/tenniarb/go/optimize"
)

// suggestUI is the Suggest Improvements list of a window: what the last analysis of the file found,
// the preview shown last, and the version the list was built from.
type suggestUI struct {
	findings []optimize.Finding
	preview  string
	version  int
}

// suggestRowWidth is the width the rows of the list wrap to.
func suggestRowWidth() int { return g.Scaled(360) }

// showSuggest lists what could be tidied in the scene of the window: every row can show the change or
// apply it as one edit. The list is rebuilt after an apply, so it shows what is left.
func (v *Viewer) showSuggest() {
	if v.stree == nil || !v.sceneMode() {
		v.notice("Suggestions are for a scene: open a screen or a component.")
		return
	}
	files := v.stree.Files()
	if len(files) == 0 {
		return
	}
	v.closePopover()
	v.sug.findings = optimize.Analyze(v.stree)
	v.sug.preview, v.sug.version = "", v.version
	p := v.win.Popover(g.Gap(g.Scaled(4)), g.Padding(g.Scaled(6)))
	var prev *g.LabelView
	p.Add(g.Label(fmt.Sprintf("%d suggestions in %s", len(v.sug.findings), files[0]), faint()))
	p.Add(g.Label("", g.Wrap(), g.Size(suggestRowWidth(), 0), g.Ref(&prev), g.ID("suggest-preview")))
	for i := range v.sug.findings {
		f := v.sug.findings[i]
		row := []g.Option{
			g.GrowX(), g.Padding(0), g.Gap(g.Scaled(2)),
			g.Label(f.Rule+": "+f.Detail+" ("+f.Saving+")", g.Wrap(), g.Size(suggestRowWidth(), 0)),
		}
		if f.Rule == optimize.RuleRepeat || f.Rule == optimize.RuleNoMerge {
			row = append(row, g.Label("Report only: nothing to apply here.", faint()))
		} else {
			row = append(row, g.HBox(g.GrowX(), g.Padding(0), g.Gap(g.Scaled(6)),
				g.Button("Preview", g.ID(fmt.Sprintf("suggest-preview-%d", i)), g.Init(func(b *g.ButtonView) {
					b.OnClick(func() {
						_, preview, err := v.suggestionEdits(f)
						if err != nil {
							preview = "Not applied: " + err.Error()
						}
						v.sug.preview = preview
						prev.SetText(preview)
					})
				})),
				g.Button("Apply", g.ID(fmt.Sprintf("suggest-apply-%d", i)), g.Init(func(b *g.ButtonView) {
					b.OnClick(func() {
						if v.applySuggestion(f) {
							p.Close()
							v.app.Async(v.showSuggest)
						}
					})
				})),
			))
		}
		p.Add(g.VBox(row...))
	}
	if len(v.sug.findings) == 0 {
		p.Add(g.Label("Nothing to tidy in this file.", faint()))
	}
	v.openPopover(p)
	width, _ := v.view.Canvas().Size()
	p.ShowAtPoint(v.view.Canvas(), width/2, 0, g.PopoverBelow)
}

// suggestionEdits builds the change of a finding of the list; a file that changed under the list is
// refused, so the offsets of the finding still mean what they said.
func (v *Viewer) suggestionEdits(f optimize.Finding) ([]optimize.Edit, string, error) {
	if v.stree == nil {
		return nil, "", errors.New("no scene is loaded")
	}
	if v.sug.version != v.version {
		return nil, "", errors.New("the file changed: open Suggest Improvements again")
	}
	return optimize.Apply(v.stree, f)
}

// applySuggestion writes one finding into the file through the ordinary text edit, so it is one undo
// step and a change that adds scene errors is refused before anything moves. It reports whether the
// file changed.
func (v *Viewer) applySuggestion(f optimize.Finding) bool {
	edits, _, err := v.suggestionEdits(f)
	if err != nil {
		v.sug.preview = "Not applied: " + err.Error()
		v.notice(v.sug.preview)
		return false
	}
	text := v.fileText(f.File)
	one, err := optimize.ApplyText(text, edits)
	if err != nil {
		v.sug.preview = "Not applied: " + err.Error()
		v.notice(v.sug.preview)
		return false
	}
	n := v.stree.File(f.File)
	if n == nil {
		v.notice("Not applied: " + f.File + " is not part of this document")
		return false
	}
	if !v.sceneEdit(n, TextEdit{Start: one.Start, End: one.End, New: one.New}) {
		v.sug.preview = "Not applied: the change adds scene errors"
		return false
	}
	switch d := strings.Count(text, "\n") - strings.Count(v.fileText(f.File), "\n"); {
	case d > 0:
		v.notice(fmt.Sprintf("Applied %s: %d lines fewer", f.Rule, d))
	case d < 0:
		v.notice(fmt.Sprintf("Applied %s: %d lines more", f.Rule, -d))
	default:
		v.notice(fmt.Sprintf("Applied %s: the same number of lines", f.Rule))
	}
	return true
}
