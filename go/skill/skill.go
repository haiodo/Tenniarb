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

// Install writes the skill into every directory of Targets and links the Claude Code one to the shared
// copy, as the other skills of this setup are linked. It returns what it wrote and what failed.
func Install() ([]string, []error) {
	var done []string
	var errs []error
	for _, dir := range Targets() {
		if err := Write(dir); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", dir, err))
			continue
		}
		done = append(done, dir)
	}
	if len(done) > 0 {
		if err := link(claudeDir(), done[0]); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", claudeDir(), err))
		}
	}
	return done, errs
}

// Write writes every file of the skill into dir, replacing what is there. A link there, as an older
// `make install` left it, is replaced by the real files.
func Write(dir string) error {
	if fi, err := os.Lstat(dir); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(dir); err != nil {
			return err
		}
	}
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

// Targets are the directories the skill is written into: the shared ~/.agents/skills and opencode.
func Targets() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".agents", "skills", Name),
		filepath.Join(home, ".config", "opencode", "skill", Name),
	}
}

// claudeDir is where Claude Code reads its skills: a link to the shared copy, as the other skills there.
func claudeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "skills", Name)
}

// link makes dir a link to target, replacing a directory an older install wrote there.
func link(dir, target string) error {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	if fi, err := os.Lstat(dir); err == nil && fi.Mode()&os.ModeSymlink == 0 {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	if cur, err := os.Readlink(dir); err == nil && cur == target {
		return nil
	}
	_ = os.Remove(dir)
	return os.Symlink(target, dir)
}
