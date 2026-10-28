package mcpserver

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/haiodo/tenniarb/go/link"
	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/servicewire"
)

// linkRepos remembers the git remotes of the clones the links name.
var linkRepos link.Repos

// stripIndex drops the "#n" that tells the n-th of equal names; a scene path cannot carry it (spec 4.2).
func stripIndex(seg string) string {
	if i := strings.LastIndex(seg, "#"); i >= 0 {
		if n, err := strconv.Atoi(seg[i+1:]); err == nil && n > 0 {
			return seg[:i]
		}
	}
	return seg
}

// scenePath is the dotted path (spec 4.2) of the place a link names; "" for the file itself. Reaching
// into an instance ("a/b") selects the use that holds it.
func scenePath(l link.Link) string {
	var segs []string
	for _, s := range l.Element {
		segs = append(segs, scene.NormalizeName(stripIndex(s)))
	}
	if l.Node != "" {
		anchor, _, _ := strings.Cut(l.Node, "/")
		for _, s := range strings.Split(anchor, ".") {
			segs = append(segs, stripIndex(s))
		}
	}
	return strings.Join(segs, ".")
}

// expandLink turns a link in in.File ("dir/File.tenn#Element/node" or a tenniarb:// URL) into the file
// relative to the workspace and, when no nodes were given, the node the link names.
func (s *Server) expandLink(in selectIn) (selectIn, error) {
	if !link.IsURL(in.File) && !strings.Contains(in.File, "#") {
		return in, nil
	}
	l, err := link.Parse(in.File)
	if err != nil {
		return in, err
	}
	base, _ := os.UserHomeDir()
	if !l.Home {
		base = link.FindRoot(s.root)
		if remote := linkRepos.Remote(base); l.Repo != "" && base != "" && remote != "" && !strings.EqualFold(remote, l.Repo) {
			return in, fmt.Errorf("the link is for the repository %s, this workspace is in %s", l.Repo, remote)
		}
	}
	if base == "" {
		base = s.root
	}
	rel, err := filepath.Rel(s.root, filepath.Join(base, filepath.FromSlash(l.Path)))
	if err != nil || !filepath.IsLocal(rel) {
		return in, fmt.Errorf("%s is outside the workspace", l.Path)
	}
	in.File = filepath.ToSlash(rel)
	if p := scenePath(l); len(in.Nodes) == 0 && p != "" {
		in.Nodes = []string{p}
	}
	return in, nil
}

// linkOf are the links to the selection of the front window: the URL and the plain text; "" when the
// file can be named by neither a repository nor the home folder.
func linkOf(abs string, sel servicewire.Selection) (url, text string) {
	var element []string
	if sel.Element != "" {
		element = strings.Split(sel.Element, ".")
	}
	node := ""
	if len(sel.Nodes) > 0 {
		node = strings.TrimPrefix(sel.Nodes[0], sel.Element+".")
	}
	home, _ := os.UserHomeDir()
	l, _, err := linkRepos.Of(abs, home, element, node)
	if err != nil {
		return "", ""
	}
	if l.Home {
		return l.URL(), ""
	}
	return l.URL(), l.Text()
}
