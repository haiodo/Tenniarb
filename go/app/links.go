package app

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/haiodo/tenniarb/go/link"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

// reposFile maps a normalised git remote to the folder where this user cloned it; it is filled when an
// opened link names a repository nothing known holds.
const reposFile = "repos.json"

// maxLinkRoots caps the clones looked at for a link; each costs one git call the first time.
const maxLinkRoots = 50

// LoadRepoDirs reads repos.json; it is empty when the file is missing or malformed.
func LoadRepoDirs(dir string) map[string]string {
	m := map[string]string{}
	readJSON(dir, reposFile, &m)
	return m
}

// SaveRepoDir remembers that the repository remote is cloned in local.
func SaveRepoDir(dir, remote, local string) error {
	m := LoadRepoDirs(dir)
	m[remote] = local
	return writeJSON(dir, reposFile, m)
}

// linkRoots are the clones a link may be in, most likely first: the remembered one, then those of the
// open, recent and framed documents. A plain link compares no remote, so none is asked for.
func linkRoots(l link.Link, repos *link.Repos, dir string, open, recent []string) []link.Root {
	var roots []link.Root
	seen := map[string]bool{}
	add := func(d string) {
		if d == "" || seen[d] || len(roots) >= maxLinkRoots {
			return
		}
		seen[d] = true
		r := link.Root{Dir: d}
		if l.Repo != "" {
			r.Remote = repos.Remote(d)
		}
		roots = append(roots, r)
	}
	// The user named this folder for the repository: its own remote does not have to match.
	if d, ok := LoadRepoDirs(dir)[l.Repo]; ok && l.Repo != "" {
		if _, err := os.Stat(d); err == nil {
			seen[d] = true
			roots = append(roots, link.Root{Dir: d, Remote: l.Repo})
		}
	}
	var framed []string
	if data, err := os.ReadFile(filepath.Join(dir, framesFile)); err == nil {
		for p := range ParseFrames(data) {
			framed = append(framed, p)
		}
		slices.Sort(framed)
	}
	for _, list := range [][]string{open, recent, framed} {
		for _, p := range list {
			add(link.FindRoot(p))
		}
	}
	return roots
}

// linkElement finds the element a link names: its names are the ones PathOf writes, else the names a
// scene path uses (see scene.NormalizeName), as an agent that read them from MCP has them.
func linkElement(root *model.Element, segs []string) *model.Element {
	if e := Resolve(root, segs); e != nil {
		return e
	}
	cur := root
	for _, seg := range segs {
		name, skip := splitIndex(seg)
		key := scene.NormalizeName(name)
		var next *model.Element
		for _, c := range cur.Elements {
			if scene.NormalizeName(c.Name) != key {
				continue
			}
			if skip == 0 {
				next = c
				break
			}
			skip--
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

// linkItem is the name of the item of e that a link node names, exactly or as a scene path writes it.
func linkItem(e *model.Element, node string) (string, bool) {
	for _, same := range []func(string) bool{
		func(n string) bool { return n == node },
		func(n string) bool { return scene.NormalizeName(n) == scene.NormalizeName(node) },
	} {
		for _, it := range e.Items {
			if !it.IsLink() && same(it.Name) {
				return it.Name, true
			}
		}
	}
	return "", false
}
