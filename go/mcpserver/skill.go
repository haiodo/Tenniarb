package mcpserver

import "github.com/haiodo/tenniarb/go/skill"

// readSkill returns SKILL.md, or the skill file at p (such as "layout.md").
func readSkill(p string) (string, error) { return skill.Read(p) }

// skillFiles lists the files of the skill, for the tools that name them.
func skillFiles() []string { return skill.Names() }
