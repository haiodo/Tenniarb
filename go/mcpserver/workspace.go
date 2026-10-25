package mcpserver

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/haiodo/tenniarb/go/scene"
)

const tennExt = ".tenn"

func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// evalExisting resolves symlinks of the longest existing prefix of p and appends the rest, so a file
// that does not exist yet is still checked through the real location of its directory.
func evalExisting(p string) (string, error) {
	r, err := filepath.EvalSymlinks(p)
	if err == nil {
		return r, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	dir := filepath.Dir(p)
	if dir == p {
		return "", err
	}
	rd, err := evalExisting(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(rd, filepath.Base(p)), nil
}

// resolve maps a client path to a real file inside the root. It returns the absolute path with
// symlinks resolved and the slash path relative to the root.
func (s *Server) resolve(rel string) (abs, relSlash string, err error) {
	if rel == "" {
		return "", "", errors.New("file is required")
	}
	if filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" || strings.HasPrefix(rel, "/") {
		return "", "", fmt.Errorf("path %q must be relative to the workspace root", rel)
	}
	p := filepath.Join(s.root, filepath.FromSlash(rel))
	if !within(s.root, p) {
		return "", "", fmt.Errorf("path %q leaves the workspace root", rel)
	}
	if filepath.Ext(p) != tennExt {
		return "", "", fmt.Errorf("path %q is not a %s file", rel, tennExt)
	}
	real, err := evalExisting(p)
	if err != nil {
		return "", "", fmt.Errorf("path %q: %v", rel, err)
	}
	if !within(s.root, real) {
		return "", "", fmt.Errorf("path %q leaves the workspace root through a symlink", rel)
	}
	r, _ := filepath.Rel(s.root, real)
	return real, filepath.ToSlash(r), nil
}

// files lists the .tenn files of the workspace, relative with slashes. Hidden entries are skipped.
func (s *Server) files() ([]string, error) {
	var out []string
	err := filepath.WalkDir(s.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != s.root && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || filepath.Ext(p) != tennExt {
			return nil
		}
		rel, _ := filepath.Rel(s.root, p)
		if _, r, err := s.resolve(filepath.ToSlash(rel)); err == nil {
			out = append(out, r)
		}
		return nil
	})
	return out, err
}

// diagnose loads rel and returns every problem found, with warnings. The error is set only when
// the file could not be loaded at all.
func (s *Server) diagnose(rel string) (scene.Errors, *scene.Tree, error) {
	tree, err := scene.Load(s.root, rel)
	return loaded(tree, err)
}

// diagnoseText is diagnose for the text of rel given in memory, as it would be after a write.
func (s *Server) diagnoseText(rel, text string) (scene.Errors, *scene.Tree, error) {
	tree, err := scene.LoadSource(s.root, rel, text)
	return loaded(tree, err)
}

func loaded(tree *scene.Tree, err error) (scene.Errors, *scene.Tree, error) {
	if tree != nil {
		return tree.Diagnostics(), tree, nil
	}
	var es scene.Errors
	if errors.As(err, &es) {
		return es, nil, nil
	}
	return nil, nil, err
}

func fileRoot(tree *scene.Tree, rel string) *scene.Node {
	if tree == nil {
		return nil
	}
	return tree.File(rel)
}
