//go:build darwin || linux || windows

package app

import (
	"strings"

	g "github.com/haiodo/gowt"

	"github.com/haiodo/tenniarb/go/skill"
)

// installSkills writes the design skill where the agents read it (Claude Code, opencode) and says where,
// or what went wrong.
func (v *Viewer) installSkills() {
	done, problems := skill.Install()
	var failed []string
	for _, err := range problems {
		failed = append(failed, err.Error())
	}
	title, icon := "Agent skill installed", g.IconInfo
	text := strings.Join(done, "\n")
	switch {
	case len(done) == 0:
		title, icon, text = "Agent skill not installed", g.IconError, strings.Join(failed, "\n")
	case len(failed) > 0:
		title, icon = "Agent skill installed in part", g.IconWarning
		text += "\n\nFailed:\n" + strings.Join(failed, "\n")
	}
	v.win.MessageBox(g.Message{Title: title, Text: text, Icon: icon, Buttons: g.ButtonsOK})
}
