package model

import (
	"errors"
	"hash/fnv"
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

// MountFiles fills the elements of m that have FromFile with the content of their files, taken from
// tree (the scene tree of the document rel), which resolves the paths and rejects cycles. A mounted
// element keeps its content between calls until some file of the tree changes. Elements the tree
// could not mount stay empty. An element of keep already shows the new text of its file (it was
// edited here), so it is not read again.
func MountFiles(m *ElementModel, tree *scene.Tree, rel string, keep ...*Element) []Mounted {
	root := tree.File(rel)
	if root == nil {
		return nil
	}
	h := fnv.New64a()
	for _, f := range tree.Files()[1:] { // the document itself changes with every edit
		src, _ := tree.Source(f)
		h.Write([]byte(f))
		h.Write([]byte{0})
		h.Write([]byte(src))
		h.Write([]byte{0})
	}
	var out []Mounted
	mountElements(&m.Element, root, tree, h.Sum64(), keep, &out)
	return out
}

// mountElements pairs the elements below el with the scene nodes of the same file by order.
func mountElements(el *Element, file *scene.Node, tree *scene.Tree, sig uint64, keep []*Element, out *[]Mounted) {
	if len(el.Elements) != len(file.Elems) {
		return
	}
	for i, c := range el.Elements {
		sn := file.Elems[i]
		if sn.Name != c.Name {
			continue
		}
		if c.FromFile == "" {
			mountElements(c, sn, tree, sig, keep, out)
			continue
		}
		if sn.Mount == nil {
			continue
		}
		// The file name goes in too: pointing the element at another loaded file changes nothing else.
		esig := sig*31 + uint64(len(sn.Mount.Pos.File)) + fnvString(sn.Mount.Pos.File)
		fresh := false
		if c.mountSig != esig {
			if !slices.Contains(keep, c) {
				src, _ := tree.Source(sn.Mount.Pos.File)
				lib := ReadTenn(src)
				if lib == nil {
					continue
				}
				refill(c, &lib.Element)
				fresh = true
			}
			c.mountSig = esig
		}
		*out = append(*out, Mounted{El: c, File: sn.Mount.Pos.File, Fresh: fresh})
		mountElements(c, sn.Mount, tree, sig, keep, out)
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

func fnvString(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}
