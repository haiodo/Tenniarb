//go:build darwin || linux || windows

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	g "github.com/haiodo/gowt"
)

// attachSceneImage puts a picture into the scene: `image "path"` on the selected node, or a new item
// holding it when nothing is selected. The path is relative to the file that writes the node.
func (v *Viewer) attachSceneImage() {
	if !v.ensureSceneElement() || v.sv == nil || v.sv.node == nil {
		return
	}
	paths := v.win.FileDialog(g.FileDialog{
		Title:   "Attach image",
		Filters: []g.FileFilter{{Name: "Images", Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.webp"}},
	})
	if len(paths) != 1 {
		return
	}
	// The picture resolves from the file that writes the property, which is a mounted library for a
	// node that comes from one.
	target := v.sv.node
	if n, _ := v.selectedScene(); n != nil {
		target = n
	}
	file := v.ownFileOf(target)
	dir := filepath.Join(v.root, filepath.Dir(file))
	ref, err := imageRef(v.root, dir, paths[0])
	if err != nil {
		v.setBanner(err.Error())
		return
	}
	if n, _ := v.selectedScene(); n != nil {
		e, ok := SetPropEdit(v.fileText(file), n, "image", ref)
		if !ok {
			v.setBanner("Not applied: the node does not take an image")
			return
		}
		v.sceneEditIn(file, n, e)
		return
	}
	head := CommandText("item", FreeName(target, "Image"))
	v.insertAndSelect(target, head+" { "+CommandText("image", ref)+" }", head)
}

// imageRef is what `image` has to say for a picked picture: a path relative to dir when the file is
// already in the workspace, else a copy in dir/assets - a path outside the workspace root is an error.
func imageRef(root, dir, picked string) (string, error) {
	abs, err := filepath.Abs(picked)
	if err != nil {
		return "", err
	}
	if root != "" {
		if rel, err := filepath.Rel(root, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			if ref, err := filepath.Rel(dir, abs); err == nil {
				return filepath.ToSlash(ref), nil
			}
		}
	}
	assets := filepath.Join(dir, "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		return "", err
	}
	name, err := freeAssetName(assets, filepath.Base(abs))
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(assets, name), data, 0o644); err != nil {
		return "", err
	}
	return "assets/" + name, nil
}

// freeAssetName is name, or name-2, name-3... when the folder already has a file of that name, so
// attaching a second picture of the same name never replaces the first.
func freeAssetName(dir, name string) (string, error) {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		cand := name
		if i > 1 {
			cand = fmt.Sprintf("%s-%d%s", stem, i, ext)
		}
		if _, err := os.Stat(filepath.Join(dir, cand)); os.IsNotExist(err) {
			return cand, nil
		} else if err != nil {
			return "", err
		}
	}
}
