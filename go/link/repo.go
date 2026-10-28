package link

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// scpRemote is the "user@host:path" form of an ssh remote.
var scpRemote = regexp.MustCompile(`^(?:[^@/]+@)?([^:/]+):(.+)$`)

// NormalizeRemote makes "host/owner/name" of a git remote, so that the ssh and the https remotes
// of one repository are equal: no scheme, user, port or ".git", lower case. "" when raw is no remote.
func NormalizeRemote(raw string) string {
	raw = strings.TrimSpace(raw)
	var host, path string
	if i := strings.Index(raw, "://"); i >= 0 {
		u := raw[i+3:]
		host, path, _ = strings.Cut(u, "/")
		if j := strings.LastIndex(host, "@"); j >= 0 {
			host = host[j+1:]
		}
		host, _, _ = strings.Cut(host, ":")
	} else if m := scpRemote.FindStringSubmatch(raw); m != nil {
		host, path = m[1], m[2]
	}
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	if host == "" || path == "" {
		return ""
	}
	return strings.ToLower(host + "/" + path)
}

// FindRoot is the nearest folder above file (or file itself, for a folder) that holds a .git entry; a
// worktree or a submodule has a .git file. "" when there is none.
func FindRoot(file string) string {
	dir := filepath.Dir(file)
	if fi, err := os.Stat(file); err == nil && fi.IsDir() {
		dir = file
	}
	for {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		up := filepath.Dir(dir)
		if up == dir {
			return ""
		}
		dir = up
	}
}

// Repos asks git for the remote of a repository root and remembers the answer.
type Repos struct {
	mu      sync.Mutex
	remotes map[string]string
}

// Remote is the normalised remote of the repository at root: origin, else the first remote.
// "" when git is missing or the repository has none.
func (r *Repos) Remote(root string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if v, ok := r.remotes[root]; ok {
		return v
	}
	if r.remotes == nil {
		r.remotes = map[string]string{}
	}
	v := gitRemote(root)
	r.remotes[root] = v
	return v
}

func gitRemote(root string) string {
	if out, err := git(root, "remote", "get-url", "origin"); err == nil {
		return NormalizeRemote(out)
	}
	names, err := git(root, "remote")
	if err != nil {
		return ""
	}
	first, _, _ := strings.Cut(names, "\n")
	if first = strings.TrimSpace(first); first == "" {
		return ""
	}
	out, err := git(root, "remote", "get-url", first)
	if err != nil {
		return ""
	}
	return NormalizeRemote(out)
}

func git(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

// Root is a local clone: its folder and its normalised remote.
type Root struct{ Dir, Remote string }

// RootOf is the clone that holds file; ok is false outside a repository or when it has no remote.
func (r *Repos) RootOf(file string) (Root, bool) {
	dir := FindRoot(file)
	if dir == "" {
		return Root{}, false
	}
	remote := r.Remote(dir)
	return Root{dir, remote}, remote != ""
}

// ErrNoBase: the file is neither in a repository with a remote nor under the home folder, so no link
// that works on another machine can name it.
var ErrNoBase = errors.New("the file is not in a git repository with a remote and not under the home folder")

// Of is the link to the place in file. A file in a repository with a remote gets Repo and the path
// from the root; any other file under home gets Home and the path from there, and note says so.
func (r *Repos) Of(file, home string, element []string, node string) (l Link, note string, err error) {
	l = Link{Element: element, Node: node}
	if root, ok := r.RootOf(file); ok {
		l.Repo, l.Path = root.Remote, relSlash(root.Dir, file)
		return l, "", nil
	}
	if home == "" || relSlash(home, file) == "" {
		return Link{}, "", ErrNoBase
	}
	l.Home, l.Path = true, relSlash(home, file)
	note = "The file is not in a git repository with a remote"
	if FindRoot(file) == "" {
		note = "The file is not in a git repository"
	}
	return l, note + ": the link works only for files at the same place under home.", nil
}

// relSlash is file relative to dir with slashes; "" when file is not below dir.
func relSlash(dir, file string) string {
	rel, err := filepath.Rel(dir, file)
	if err != nil || !filepath.IsLocal(rel) {
		return ""
	}
	return filepath.ToSlash(rel)
}

// Resolve finds the file of l among the local clones, in the order given. A plain link (no Repo) takes
// the first clone that has the file. exists is os.Stat in the app, a fake in tests.
func Resolve(l Link, roots []Root, home string, exists func(string) bool) (string, bool) {
	if l.Home {
		p := filepath.Join(home, filepath.FromSlash(l.Path))
		return p, home != "" && exists(p)
	}
	for _, r := range roots {
		if l.Repo != "" && !strings.EqualFold(r.Remote, l.Repo) {
			continue
		}
		if p := filepath.Join(r.Dir, filepath.FromSlash(l.Path)); exists(p) {
			return p, true
		}
	}
	return "", false
}

// DocKey names a document the same in every clone of its repository: the remote and the path inside
// it; outside a repository with a remote it is the absolute path, which only that file has.
func (r *Repos) DocKey(file string) string {
	if root, ok := r.RootOf(file); ok {
		if rel := relSlash(root.Dir, file); rel != "" {
			return "repo:" + root.Remote + "#" + rel
		}
	}
	return "path:" + filepath.ToSlash(filepath.Clean(file))
}
