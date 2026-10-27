package mcpserver

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
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

// maxWalk caps the entries files visits: a root opened at the home directory must fail fast, not
// walk it all.
const maxWalk = 50000

var errTooLarge = fmt.Errorf("the workspace has more than %d entries: start tenniarb-mcp with -root at the project folder", maxWalk)

// files lists the .tenn files of the workspace, relative with slashes. Hidden entries are skipped.
func (s *Server) files() ([]string, error) {
	var out []string
	n := 0
	err := filepath.WalkDir(s.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if n++; n > maxWalk {
			return errTooLarge
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

// liveTexts returns the text of every document open in an app under the root, by relative slash
// path, and the entries by real path. A document whose app moved to a newer service is read from disk.
func (s *Server) liveTexts() (map[string]string, map[string]liveDoc, error) {
	docs := s.host.live.within(s.root)
	if len(docs) == 0 {
		return nil, nil, nil
	}
	texts := map[string]string{}
	for abs, d := range docs {
		if d.moved {
			delete(docs, abs)
			continue
		}
		res, err := s.host.live.text(abs, false)
		if err != nil {
			return nil, nil, err
		}
		d.doc.Version = res.Version
		docs[abs] = d
		r, _ := filepath.Rel(s.root, abs)
		texts[filepath.ToSlash(r)] = res.Text
	}
	return texts, docs, nil
}

// diagnose loads rel and returns every problem found, with warnings. The error is set only when
// the file could not be loaded at all. Open documents count with their unsaved text.
func (s *Server) diagnose(rel string) (scene.Errors, *scene.Tree, error) {
	texts, docs, err := s.liveTexts()
	if err != nil {
		return nil, nil, err
	}
	if e, ok := s.host.cache.get(s.root, rel, docs); ok {
		return e.diags, e.tree, nil
	}
	tree, err := scene.LoadOverlay(s.root, rel, texts)
	if tree == nil {
		return loaded(tree, err)
	}
	deps := map[string]string{}
	for _, f := range loadedFiles(tree, rel) {
		st := stamp(s.root, f, docs)
		// A file that changed while it was read would get the new stamp with the old text.
		if src, _ := tree.Source(f); st[0] != 'v' && !sameSize(s.root, f, src) {
			st = "-"
		}
		deps[f] = st
	}
	e := s.host.cache.put(s.root, rel, tree, deps)
	return e.diags, e.tree, nil
}

func sameSize(root, rel, src string) bool {
	st, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil && st.Size() == int64(len(src))
}

// diagnoseText is diagnose for the text of rel given in memory, as it would be after a write.
func (s *Server) diagnoseText(rel, text string) (scene.Errors, *scene.Tree, error) {
	texts, _, err := s.liveTexts()
	if err != nil {
		return nil, nil, err
	}
	if texts == nil {
		texts = map[string]string{}
	}
	texts[rel] = text
	tree, err := scene.LoadOverlay(s.root, rel, texts)
	return loaded(tree, err)
}

// text returns the current text of a resolved file: the app's when it has the file open.
func (s *Server) text(abs string) (text string, live *liveDoc, err error) {
	if d := s.host.live.lookup(abs); d != nil {
		if d.moved {
			return "", nil, errMoved
		}
		res, err := s.host.live.text(abs, false)
		if err != nil {
			return "", nil, err
		}
		d.doc.Version, d.doc.Dirty = res.Version, res.Dirty
		return res.Text, d, nil
	}
	b, err := os.ReadFile(abs)
	return string(b), nil, err
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
