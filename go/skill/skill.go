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

// Install writes the skill into every place an agent reads it, and links the Claude Code one to the
// shared copy, as the other skills of this setup are linked. It returns what it wrote and what failed.
func Install() ([]string, []error) {
	agents := make([]string, 0, len(Places()))
	for _, pl := range Places() {
		agents = append(agents, pl.Agent)
	}
	return InstallFor(agents)
}

// InstallFor is Install limited to the agents named: the shared copy under ~/.agents is written always,
// since the Claude Code directory is a link to it.
func InstallFor(agents []string) ([]string, []error) {
	var done []string
	var errs []error
	shared := ""
	places := Places()
	for _, pl := range places {
		if pl.Agent == "agents" {
			if err := Write(pl.Dir); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", pl.Dir, err))
				continue
			}
			shared = pl.Dir
			done = append(done, pl.Dir)
			break
		}
	}
	for _, pl := range places {
		if pl.Agent == "agents" || !slices.Contains(agents, pl.Agent) {
			continue
		}
		var err error
		if pl.Link != "" {
			if shared == "" {
				continue // the shared copy failed already
			}
			err = link(pl.Dir, shared)
		} else {
			err = Write(pl.Dir)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", pl.Dir, err))
			continue
		}
		done = append(done, pl.Dir)
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

// Place is one directory the skill is written into and the agent that reads it.
type Place struct {
	Agent string // "agents" for the shared copy, then opencode, codex and claude
	Dir   string
	// Link is the directory Dir becomes a link to; empty when the files are written there.
	Link string
}

// Places are the directories the skill is written into: the shared ~/.agents/skills that every agent
// reads by default, the copy opencode and codex keep of their own, and the Claude Code link.
func Places() []Place {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	shared := filepath.Join(home, ".agents", "skills", Name)
	return []Place{
		{Agent: "agents", Dir: shared},
		{Agent: "opencode", Dir: filepath.Join(home, ".config", "opencode", "skill", Name)},
		{Agent: "codex", Dir: filepath.Join(home, ".codex", "skills", Name)},
		{Agent: "claude", Dir: filepath.Join(home, ".claude", "skills", Name), Link: shared},
	}
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
