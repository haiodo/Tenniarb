// Package skill is the tenniarb design skill: what an agent reads before drawing in a .tenn file. The
// files are embedded, so the app and the CLI install them without the source tree.
package skill

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

//go:embed files
var files embed.FS

// Name is the directory the skill is installed under, and the name the MCP server gives it.
const Name = "tenniarb-design"

// Read returns the file name of the skill, SKILL.md when name is empty. The error lists the files.
func Read(name string) (string, error) {
	name = strings.TrimPrefix(strings.TrimSpace(name), "/")
	if name == "" {
		name = "SKILL.md"
	}
	name = path.Clean(name)
	b, err := fs.ReadFile(files, path.Join("files", name))
	if err != nil || !fs.ValidPath(name) {
		return "", fmt.Errorf("no skill file %q; files: %s", name, strings.Join(Names(), ", "))
	}
	return string(b), nil
}

// Names lists the files of the skill, SKILL.md first.
func Names() []string {
	var out []string
	_ = fs.WalkDir(files, "files", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, strings.TrimPrefix(p, "files/"))
		}
		return nil
	})
	slices.Sort(out)
	return out
}

// Install writes every file of the skill into dir, replacing what is there.
func Install(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, name := range Names() {
		text, err := Read(name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(text), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Targets are the directories the skill is installed into: Claude Code and opencode, each when its config
// directory exists or always for Claude Code, which every agent setup here has.
func Targets() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".claude", "skills", Name),
		filepath.Join(home, ".config", "opencode", "skill", Name),
	}
}
