package mcpserver

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ignored answers whether a directory of the workspace is left out of the walk: the .gitignore files
// from the file system root down to that directory say so, the deeper file winning.
type ignored struct{ rules []ignoreRule }

// ignoreRule is one line of a .gitignore as path segments from the workspace root; "**" stands for any
// number of segments.
type ignoreRule struct {
	glob   []string
	negate bool
}

// loadIgnores reads .gitignore from the workspace root up to the file system root, the outermost first.
func loadIgnores(root string) *ignored {
	ig := &ignored{}
	var dirs []string
	for d := filepath.Clean(root); ; d = filepath.Dir(d) {
		dirs = append(dirs, d)
		if d == filepath.Dir(d) || len(dirs) > 32 {
			break
		}
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		rel, err := filepath.Rel(root, dirs[i])
		if err != nil {
			continue
		}
		ig.read(dirs[i], rel)
	}
	return ig
}

// read adds the .gitignore of dir, whose path from the workspace root is base.
func (ig *ignored) read(dir, base string) {
	b, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		return
	}
	var at []string
	if base != "." {
		at = strings.Split(filepath.ToSlash(base), "/")
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimRight(line, " \r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		r := ignoreRule{negate: strings.HasPrefix(line, "!")}
		line = strings.TrimPrefix(line, "!")
		if line == "" {
			continue
		}
		// A leading or inner slash anchors the pattern to the directory of the file; without one it
		// matches at any depth. Only directories are tested, so a trailing slash changes nothing.
		anchored := strings.HasPrefix(line, "/") || strings.Contains(strings.TrimSuffix(line, "/"), "/")
		line = strings.Trim(strings.TrimSuffix(line, "/"), "/")
		if line == "" || strings.HasPrefix(line, "!") {
			continue
		}
		segs := strings.Split(line, "/")
		switch {
		case !anchored:
			r.glob = append([]string{"**"}, segs...)
		case at == nil:
			r.glob = segs
		case at[0] == "..":
			continue // the file is above the workspace: an anchored pattern there names nothing inside it
		default:
			r.glob = append(append([]string{}, at...), segs...)
		}
		ig.rules = append(ig.rules, r)
	}
}

// match is true when the path from the workspace root, slashed, is ignored.
func (ig *ignored) match(p string) bool {
	segs := strings.Split(p, "/")
	skip := false
	for _, r := range ig.rules {
		if matchSegs(r.glob, segs) {
			skip = !r.negate
		}
	}
	return skip
}

// matchSegs matches a pattern of segments against a path of segments, "**" taking any number of them.
func matchSegs(pat, segs []string) bool {
	if len(pat) == 0 {
		return len(segs) == 0
	}
	if pat[0] == "**" {
		for i := 0; i <= len(segs); i++ {
			if matchSegs(pat[1:], segs[i:]) {
				return true
			}
		}
		return false
	}
	if len(segs) == 0 {
		return false
	}
	ok, err := path.Match(pat[0], segs[0])
	return err == nil && ok && matchSegs(pat[1:], segs[1:])
}
