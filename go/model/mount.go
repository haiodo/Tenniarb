package model

import (
	"errors"
	"hash/maphash"
	"slices"

	"github.com/haiodo/tenniarb/go/scene"
)

// ErrMounted is returned for an edit inside content that comes from a --from-file when the store has
// no way to write it back: set ElementModelStore.MountWritable and OnMountEdit.
var ErrMounted = errors.New("the content comes from --from-file: edit the source file")

// MountOf is the element whose file holds the content n edits: n or its nearest ancestor with
// FromFile, nil when n is in the document itself. Renaming an element with FromFile edits its line, so
// rename looks from its parent.
func MountOf(n *Element, rename bool) *Element {
	if rename && n.FromFile != "" {
		n = n.Parent
	}
	for ; n != nil; n = n.Parent {
		if n.FromFile != "" {
			return n
		}
	}
	return nil
}

// Mounted is an element with FromFile after MountFiles: the file it shows, relative to the workspace
// root, and whether its content was read just now.
type Mounted struct {
	El    *Element
	File  string
	Fresh bool
}

// readTenn is ReadTenn; tests count the calls.
var readTenn = ReadTenn

// MountFiles fills the elements of m that have FromFile with the content of their files, taken from
// tree (the scene tree of the document rel), which resolves the paths and rejects cycles. A mounted
// element keeps its content between calls until its file changes. Elements the tree could not mount
// stay empty. An element of keep already shows the new text of its file (it was edited here), so it is
// not read again. A file shown by several elements is parsed once.
func MountFiles(m *ElementModel, tree *scene.Tree, rel string, keep ...*Element) []Mounted {
	root := tree.File(rel)
	if root == nil {
		return nil
	}
	mp := &mountPass{tree: tree, keep: keep, libs: map[string]*ElementModel{}}
	mountElements(&m.Element, root, mp)
	return mp.out
}

type mountPass struct {
	tree *scene.Tree
	keep []*Element
	libs map[string]*ElementModel // files parsed in this pass
	out  []Mounted
}

// read is the content of file, a new copy for each element: the first caller takes the parsed tree.
func (mp *mountPass) read(file string) *Element {
	if lib := mp.libs[file]; lib != nil {
		return lib.Element.Clone(true, true)
	}
	src, _ := mp.tree.Source(file)
	lib := readTenn(src)
	if lib == nil {
		return nil
	}
	mp.libs[file] = lib
	return &lib.Element
}

// mountElements pairs the elements below el with the scene nodes of the same file by order.
func mountElements(el *Element, file *scene.Node, mp *mountPass) {
	if len(el.Elements) != len(file.Elems) {
		return
	}
	for i, c := range el.Elements {
		sn := file.Elems[i]
		if sn.Name != c.Name {
			continue
		}
		if c.FromFile == "" {
			mountElements(c, sn, mp)
			continue
		}
		if sn.Mount == nil {
			continue
		}
		// The content is the file's alone, so the file name and its text are the signature: pointing the
		// element at another file changes nothing else, and a nested mount has a signature of its own.
		src, _ := mp.tree.Source(sn.Mount.Pos.File)
		esig := hashString(sn.Mount.Pos.File)*31 + hashString(src)
		fresh := false
		if c.mountSig != esig {
			if !slices.Contains(mp.keep, c) {
				lib := mp.read(sn.Mount.Pos.File)
				if lib == nil {
					continue
				}
				refill(c, lib)
				fresh = true
			}
			c.mountSig = esig
		}
		mp.out = append(mp.out, Mounted{El: c, File: sn.Mount.Pos.File, Fresh: fresh})
		mountElements(c, sn.Mount, mp)
	}
}

// refill puts the content of src into e. The elements keep their identity while the names pair up, so
// the canvas element and the undo steps of the editor stay attached after a text edit of the file.
func refill(e, src *Element) {
	e.Items, e.Description, e.Properties = src.Items, src.Description, src.Properties
	for _, it := range e.Items {
		it.Parent = e
	}
	if len(e.Elements) == len(src.Elements) && slices.EqualFunc(e.Elements, src.Elements, func(a, b *Element) bool {
		return a.Name == b.Name && a.FromFile == b.FromFile
	}) {
		for i, c := range e.Elements {
			if c.FromFile == "" { // a nested mount keeps the content it has
				refill(c, src.Elements[i])
			}
		}
		return
	}
	e.Elements = nil
	for _, k := range src.Elements {
		e.AddElement(k)
	}
}

var sigSeed = maphash.MakeSeed()

func hashString(s string) uint64 { return maphash.String(sigSeed, s) }
