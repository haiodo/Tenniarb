// Package app is the Tenniarb window: a document loaded from a .tenn file, its element selection and
// viewport, and the gowt UI on top. The logic that needs no window lives in document.go, state.go and
// viewport.go.
package app

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/tenn"
)

// ParseError is one parser message with its position in the file.
type ParseError struct {
	Line, Col int
	Message   string
}

func (e ParseError) String() string { return fmt.Sprintf("%d:%d: %s", e.Line, e.Col, e.Message) }

// Document is a .tenn file as read from disk. Model is nil when the file has parse errors.
type Document struct {
	Path   string
	Model  *model.ElementModel
	Errors []ParseError
	Stamp  Stamp
}

// Load reads and parses path. The error is for I/O only; parse errors come back in the Document.
func Load(path string) (*Document, error) {
	stamp, err := StampOf(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc := ParseText(string(data))
	doc.Path, doc.Stamp = path, stamp
	return doc, nil
}

// ParseText parses file contents; a leading BOM is dropped as Document.read does.
func ParseText(text string) *Document {
	tree, parser := tenn.Parse(strings.TrimPrefix(text, "\xef\xbb\xbf"))
	if parser.Errors.HasErrors() {
		doc := &Document{}
		for _, e := range parser.Errors.Errors {
			doc.Errors = append(doc.Errors, ParseError{e.Line, e.Col, e.Message})
		}
		return doc
	}
	return &Document{Model: model.ParseTenn(tree)}
}

// Stamp identifies a version of a file on disk.
type Stamp struct {
	ModTime time.Time
	Size    int64
}

// StampOf stats path.
func StampOf(path string) (Stamp, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return Stamp{}, err
	}
	return Stamp{fi.ModTime(), fi.Size()}, nil
}

// Watcher decides when a file has to be read again: it compares the stamp of the last version seen.
type Watcher struct {
	Path string
	Last Stamp
}

// Poll reports whether the file differs from the last version seen and remembers the new one. A file
// that cannot be stat-ed (an editor replacing it) is not a change; the next poll sees the new file.
func (w *Watcher) Poll() bool {
	s, err := StampOf(w.Path)
	if err != nil || s == w.Last {
		return false
	}
	w.Last = s
	return true
}

// PathOf names e by the names of its ancestors; a repeated name among siblings gets "#n", n counting
// from 1 for the second one. Resolve finds the element again after the file is read anew.
func PathOf(e *model.Element) []string {
	var rev []string
	for ; e != nil && e.Parent != nil; e = e.Parent {
		seg := e.Name
		n := 0
		for _, s := range e.Parent.Elements {
			if s == e {
				break
			}
			if s.Name == e.Name {
				n++
			}
		}
		if n > 0 {
			seg += "#" + strconv.Itoa(n)
		}
		rev = append(rev, seg)
	}
	path := make([]string, len(rev))
	for i, s := range rev {
		path[len(rev)-1-i] = s
	}
	return path
}

// Resolve finds the element of path under root; the root for an empty path, nil when it is gone.
func Resolve(root *model.Element, path []string) *model.Element {
	cur := root
	for _, seg := range path {
		name, n := seg, 0
		// "#n" is ours only if what precedes it is a name; a name may itself contain '#'.
		if i := strings.LastIndex(seg, "#"); i >= 0 {
			if k, err := strconv.Atoi(seg[i+1:]); err == nil && k > 0 {
				name, n = seg[:i], k
			}
		}
		next := nth(cur, name, n)
		if next == nil && name != seg {
			next = nth(cur, seg, 0)
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

func nth(parent *model.Element, name string, n int) *model.Element {
	for _, c := range parent.Elements {
		if c.Name == name {
			if n == 0 {
				return c
			}
			n--
		}
	}
	return nil
}
