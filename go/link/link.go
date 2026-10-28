// Package link builds and reads links to an element or a node of a .tenn file that work on another
// machine: the file is named by its git repository and its path inside it, never by an absolute path.
// The format is in docs/links.md. The package uses the standard library only.
package link

import (
	"errors"
	"net/url"
	"path/filepath"
	"strings"
)

// Scheme is the URL scheme the app registers with the system.
const Scheme = "tenniarb"

// Link points to a place in a .tenn file.
type Link struct {
	// Repo is the normalised git remote of the repository that holds the file (see NormalizeRemote);
	// empty for a plain link and for a Home link.
	Repo string
	// Home says the file is in no repository: Path is then relative to the home folder of the user.
	Home bool
	// Path is the slash-separated path of the file, relative to the repository root (or to home).
	Path string
	// Element is the elements from the root down, as the app's PathOf writes them: names, "#n" for the
	// n-th repeat among siblings. Empty is the root.
	Element []string
	// Node is the selected node of the element (the app's NodePath, "a/b" reaches into an instance) or
	// the name of the selected item of a diagram; empty selects none.
	Node string
}

// escSeg keeps a name from being cut at the separators of a path.
var escSeg = strings.NewReplacer("%", "%25", ".", "%2E", "/", "%2F")

func (l Link) elementText() string {
	segs := make([]string, len(l.Element))
	for i, s := range l.Element {
		segs[i] = escSeg.Replace(s)
	}
	return strings.Join(segs, ".")
}

func (l Link) fragment() string {
	f := l.elementText()
	if l.Node != "" {
		f += "/" + l.Node
	}
	return f
}

// Text is the plain-text form for chats and agents: "dir/File.tenn#Element.Sub/node".
// Repo and Home are not part of it.
func (l Link) Text() string {
	if f := l.fragment(); f != "" {
		return l.Path + "#" + f
	}
	return l.Path
}

// esc is a query value with the slash left readable.
func esc(s string) string {
	s = url.QueryEscape(s)
	s = strings.ReplaceAll(s, "+", "%20")
	return strings.ReplaceAll(s, "%2F", "/")
}

// URL is the tenniarb://open link.
func (l Link) URL() string {
	var b strings.Builder
	b.WriteString(Scheme + "://open?")
	sep := ""
	add := func(k, v string) {
		if v != "" {
			b.WriteString(sep + k + "=" + esc(v))
			sep = "&"
		}
	}
	add("repo", l.Repo)
	if l.Home {
		add("home", "1")
	}
	add("path", l.Path)
	add("el", l.elementText())
	add("node", l.Node)
	return b.String()
}

// ErrNotLink is returned for text that is neither form.
var ErrNotLink = errors.New("not a tenniarb link")

// Parse reads the URL form or the plain-text form; surrounding white space is ignored.
// A plain link has no Repo: it names the file inside whichever repository holds it.
func Parse(s string) (Link, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(strings.ToLower(s), Scheme+":") {
		return parseURL(s)
	}
	file, frag, _ := strings.Cut(s, "#")
	if !strings.HasSuffix(strings.ToLower(file), ".tenn") {
		return Link{}, ErrNotLink
	}
	l := Link{Path: file}
	if err := l.setFragment(frag); err != nil {
		return Link{}, err
	}
	return l, l.check()
}

func (l *Link) setFragment(frag string) error {
	el, node, _ := strings.Cut(frag, "/")
	l.Node = node
	return l.setElement(el)
}

func (l *Link) setElement(el string) error {
	l.Element = nil
	if el == "" {
		return nil
	}
	for _, s := range strings.Split(el, ".") {
		if s == "" {
			return errors.New("link: empty element name")
		}
		if u, err := url.PathUnescape(s); err == nil {
			s = u
		}
		l.Element = append(l.Element, s)
	}
	return nil
}

func parseURL(s string) (Link, error) {
	u, err := url.Parse(s)
	if err != nil {
		return Link{}, err
	}
	if !strings.EqualFold(u.Host, "open") {
		return Link{}, errors.New("link: unknown action, only open is known")
	}
	q := u.Query()
	l := Link{Repo: q.Get("repo"), Home: q.Get("home") == "1", Path: q.Get("path"), Node: q.Get("node")}
	if err := l.setElement(q.Get("el")); err != nil {
		return Link{}, err
	}
	return l, l.check()
}

// check refuses a path that leaves the repository or names no .tenn file: a link comes from outside.
func (l Link) check() error {
	if l.Path == "" || !strings.HasSuffix(strings.ToLower(l.Path), ".tenn") {
		return errors.New("link: path must name a .tenn file")
	}
	if strings.HasPrefix(l.Path, "/") || strings.Contains(l.Path, "\\") || !filepath.IsLocal(filepath.FromSlash(l.Path)) {
		return errors.New("link: path must stay inside the repository")
	}
	return nil
}

// IsLink reports whether s reads as a link in either form.
func IsLink(s string) bool {
	_, err := Parse(s)
	return err == nil
}

// IsURL reports whether s starts like a tenniarb:// link, valid or not.
func IsURL(s string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), Scheme+":")
}
