//go:build darwin || linux || windows

package app

import (
	"fmt"
	"strings"

	g "github.com/haiodo/gowt"

	"github.com/haiodo/tenniarb/go/skill"
)

// installSkills writes the design skill where the agents read it (Claude Code, opencode) and says where,
// or what went wrong.
func (v *Viewer) installSkills() {
	var done, failed []string
	for _, dir := range skill.Targets() {
		if err := skill.Install(dir); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", dir, err))
			continue
		}
		done = append(done, dir)
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
