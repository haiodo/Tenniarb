package model

import (
	"errors"
	"hash/fnv"

	"github.com/haiodo/tenniarb/go/scene"
)

// ErrMounted is returned for an edit inside content that comes from a --from-file. Ceiling: no
// write-back to the source file from the model; edit that file as its own document.
var ErrMounted = errors.New("the content comes from --from-file: edit the source file")

// InMount reports whether an ancestor of e takes its content from a file, so e is part of it.
func (e *Element) InMount() bool {
	for p := e.Parent; p != nil; p = p.Parent {
		if p.FromFile != "" {
			return true
		}
	}
	return false
}

// MountFiles fills the elements of m that have FromFile with the content of their files, taken from
// tree (the scene tree of the document rel), which resolves the paths and rejects cycles. A mounted
// element keeps its content between calls until some file of the tree changes. Elements the tree
// could not mount stay empty.
func MountFiles(m *ElementModel, tree *scene.Tree, rel string) {
	root := tree.File(rel)
	if root == nil {
		return
	}
	h := fnv.New64a()
	for _, f := range tree.Files()[1:] { // the document itself changes with every edit
		src, _ := tree.Source(f)
		h.Write([]byte(f))
		h.Write([]byte{0})
		h.Write([]byte(src))
		h.Write([]byte{0})
	}
	mountElements(&m.Element, root, tree, h.Sum64())
}

// mountElements pairs the elements below el with the scene nodes of the same file by order.
func mountElements(el *Element, file *scene.Node, tree *scene.Tree, sig uint64) {
	if len(el.Elements) != len(file.Elems) {
		return
	}
	for i, c := range el.Elements {
		sn := file.Elems[i]
		if sn.Name != c.Name {
			continue
		}
		if c.FromFile == "" {
			mountElements(c, sn, tree, sig)
			continue
		}
		if sn.Mount == nil {
			continue
		}
		// The file name goes in too: pointing the element at another loaded file changes nothing else.
		esig := sig*31 + uint64(len(sn.Mount.Pos.File)) + fnvString(sn.Mount.Pos.File)
		if c.mountSig != esig {
			src, _ := tree.Source(sn.Mount.Pos.File)
			lib := ReadTenn(src)
			if lib == nil {
				continue
			}
			c.Elements, c.Items = nil, nil
			c.Properties = lib.Properties
			for _, k := range lib.Elements {
				c.AddElement(k)
			}
			c.mountSig = esig
		}
		mountElements(c, sn.Mount, tree, sig)
	}
}

func fnvString(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}
