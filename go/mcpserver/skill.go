package mcpserver

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// skillFS is the design skill: SKILL.md and the files it names. TestSkillExamples loads and renders
// every tenn block of it.
//
//go:embed skill
var skillFS embed.FS

const skillMain = "SKILL.md"

// readSkill returns SKILL.md, or the skill file at p (such as "layout.md").
func readSkill(p string) (string, error) {
	p = strings.TrimPrefix(strings.TrimSpace(p), "/")
	if p == "" {
		p = skillMain
	}
	p = path.Clean(p)
	b, err := fs.ReadFile(skillFS, path.Join("skill", p))
	if err != nil || !fs.ValidPath(p) {
		return "", fmt.Errorf("no skill file %q; files: %s", p, strings.Join(skillFiles(), ", "))
	}
	return string(b), nil
}

func skillFiles() []string {
	var out []string
	fs.WalkDir(skillFS, "skill", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, strings.TrimPrefix(p, "skill/"))
		}
		return nil
	})
	return out
}
