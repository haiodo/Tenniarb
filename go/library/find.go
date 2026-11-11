package library

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/haiodo/tenniarb/go/comments"
)

// EnvLibraries is a folder of .tenn libraries a document may mount by name. It is looked at before the
// libraries that ship with the application, so a checkout of a library shadows the shipped copy.
const EnvLibraries = "TENNIARB_LIBRARIES"

// findDepth bounds the walk up from a workspace: deep enough for a document anywhere in a repository,
// short enough never to climb out of the user's tree by accident.
const findDepth = 12

// Finder is where a scheme name is looked up: the installed folders first, an installed copy shadows a
// shipped one, then the shipped folders in order.
type Finder struct {
	Installed []string
	Shipped   []string
}

// Default is where a document at workspace finds the libraries it mounts with a scheme: the installed
// ones in the config folder, then TENNIARB_LIBRARIES, the libraries beside the executable (for an .app,
// Contents/Resources/libraries), and a libraries folder found going up from workspace, then from the
// working directory.
//
// The scene resolves a scheme with it, so the CLI, the window and the service all see the same files.
func Default(workspace string) Finder {
	return Finder{
		Installed: []string{filepath.Join(comments.ConfigDir(), "libraries")},
		Shipped:   Shipped(workspace),
	}
}

// Shipped lists the folders of the libraries that come with the application, in the order they are
// looked at: $TENNIARB_LIBRARIES, the one beside the executable, the resources of an .app, and a
// libraries folder of the repository the workspace or the working directory lies in.
func Shipped(workspace string) []string {
	var out []string
	add := func(dir string) {
		if dir == "" {
			return
		}
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		dir = filepath.Clean(dir)
		for _, seen := range out {
			if seen == dir {
				return
			}
		}
		out = append(out, dir)
	}
	add(os.Getenv(EnvLibraries))
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		add(filepath.Join(dir, "libraries"))
		add(filepath.Join(dir, "..", "Resources", "libraries")) // Tenniarb.app/Contents/Resources/libraries
	}
	add(repoLibraries(workspace))
	// The working directory is the checkout the command runs in: a document outside it (a design in
	// /tmp, a file opened by path) still finds the libraries of the repository.
	if cwd, err := os.Getwd(); err == nil {
		add(repoLibraries(cwd))
	}
	return out
}

// repoLibraries is the closest libraries folder with a .tenn file above workspace, or "" when there is
// none. The walk stops at the home folder and never runs longer than findDepth steps.
func repoLibraries(workspace string) string {
	dir := workspace
	if dir == "" {
		var err error
		if dir, err = os.Getwd(); err != nil {
			return ""
		}
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	home, _ := os.UserHomeDir()
	for i := 0; i < findDepth; i++ {
		cand := filepath.Join(dir, "libraries")
		if hasTenn(cand) {
			return cand
		}
		if dir == home {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
	return ""
}

// hasTenn reports whether dir holds at least one .tenn file, so that an unrelated folder named
// libraries is not taken for the one of the format.
func hasTenn(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".tenn") {
			return true
		}
	}
	return false
}

// dirs is every folder of the finder, installed first, each once.
func (f Finder) dirs() []string {
	var out []string
	for _, list := range [][]string{f.Installed, f.Shipped} {
		for _, dir := range list {
			if dir == "" {
				continue
			}
			dup := false
			for _, seen := range out {
				if seen == dir {
					dup = true
					break
				}
			}
			if !dup {
				out = append(out, dir)
			}
		}
	}
	return out
}

// Contains reports whether path is a file of one of the folders: a library the application ships or has
// installed, which a document reads but does not write.
func (f Finder) Contains(path string) bool {
	if path == "" {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, dir := range f.dirs() {
		if rel, err := filepath.Rel(dir, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// Resolve finds the file of p, an absolute path, or an error that names what was looked for and where.
func (f Finder) Resolve(p Path) (string, error) {
	if err := p.valid(); err != nil {
		return "", err
	}
	dirs := f.dirs()
	rel := p.Rel()
	for _, dir := range dirs {
		file := filepath.Join(dir, filepath.FromSlash(rel))
		if fi, err := os.Stat(file); err == nil && fi.Mode().IsRegular() {
			if abs, err := filepath.Abs(file); err == nil {
				return abs, nil
			}
			return file, nil
		}
	}
	if len(dirs) == 0 {
		return "", fmt.Errorf("library %s not found: no library folder to look in (set %s or install the application)", p.label(), EnvLibraries)
	}
	return "", fmt.Errorf("library %s not found: looked for %s in %s", p.label(), rel, strings.Join(dirs, ", "))
}

// label is the name a reader writes in a document, without the scheme.
func (p Path) label() string {
	if p.Owner == "" {
		return p.Name
	}
	return p.Owner + "/" + p.Name
}
