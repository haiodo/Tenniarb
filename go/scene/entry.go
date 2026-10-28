package scene

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/haiodo/tenniarb/go/tenn"
)

// EntryProp is the top-level property that names the entry document of a part file: entry "../Design.tenn".
const EntryProp = "entry"

// maxEntryDepth is how many folders up FindEntry looks, the workspace root included.
const maxEntryDepth = 8

// fileInfo is what entry discovery needs of a file: the files it mounts and its entry property, absolute.
type fileInfo struct {
	mounts []string
	entry  string
}

type infoStamp struct {
	size int64
	mod  time.Time
	info fileInfo
}

// infos caches the parsed mounts of files by path; a changed size or mtime reads the file again.
var infos = struct {
	sync.Mutex
	m map[string]infoStamp
}{m: map[string]infoStamp{}}

// readInfo parses file (absolute) for its mounts and entry; it fails when the file cannot be read.
func readInfo(file string) (fileInfo, bool) {
	st, err := os.Stat(file)
	if err != nil {
		return fileInfo{}, false
	}
	infos.Lock()
	c, ok := infos.m[file]
	infos.Unlock()
	if ok && c.size == st.Size() && c.mod.Equal(st.ModTime()) {
		return c.info, true
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return fileInfo{}, false
	}
	info := infoOf(file, string(b))
	infos.Lock()
	infos.m[file] = infoStamp{st.Size(), st.ModTime(), info}
	infos.Unlock()
	return info, true
}

func infoOf(file, src string) fileInfo {
	ast, _ := tenn.Parse(src)
	dir := filepath.Dir(file)
	var info fileInfo
	for _, c := range ast.Children {
		if cmdName(c) == EntryProp {
			args, _ := split(c)
			if rel := argText(args, 0); rel != "" && info.entry == "" {
				info.entry = realPath(filepath.Join(dir, filepath.FromSlash(rel)))
			}
		}
	}
	info.mounts = mountsOf(ast.Children, dir, nil)
	return info
}

// mountsOf lists the files the --from-file elements of cmds name, nested elements included.
func mountsOf(cmds []*tenn.Node, dir string, out []string) []string {
	for _, c := range cmds {
		if cmdName(c) != "element" {
			continue
		}
		args, block := split(c)
		if rel, ok := fromFile(args); ok {
			out = append(out, realPath(filepath.Join(dir, filepath.FromSlash(rel))))
			continue
		}
		out = mountsOf(block, dir, out)
	}
	return out
}

// realPath cleans p and resolves symlinks when it exists, as the loader does.
func realPath(p string) string {
	p = filepath.Clean(p)
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// ExplicitEntry reads the entry property of file (absolute, or relative to root) from the text in
// overlay or on disk. It returns the absolute path and the position of the property.
func ExplicitEntry(root, file string, overlay map[string]string) (string, Pos, bool) {
	absRoot := realPath(mustAbs(root))
	abs := file
	if !filepath.IsAbs(file) {
		abs = filepath.Join(absRoot, file)
	}
	abs = realPath(abs)
	rel, err := filepath.Rel(absRoot, abs)
	if err != nil {
		return "", Pos{}, false
	}
	rel = filepath.ToSlash(rel)
	src, ok := overlay[rel]
	if !ok {
		b, err := os.ReadFile(abs)
		if err != nil {
			return "", Pos{}, false
		}
		src = string(b)
	}
	ast, _ := tenn.Parse(src)
	l := &loader{file: rel}
	for _, c := range ast.Children {
		if cmdName(c) != EntryProp {
			continue
		}
		args, _ := split(c)
		if p := argText(args, 0); p != "" {
			return filepath.Join(filepath.Dir(abs), filepath.FromSlash(p)), l.pos(c), true
		}
	}
	return "", Pos{}, false
}

func mustAbs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}

// FindEntry looks for the entry document of file: the .tenn files in its folder and the folders up
// to the workspace root (at most maxEntryDepth) that mount it, directly or through other files. The
// nearest wins, and the search goes on from it, so the top of a chain of mounts comes back. A file
// with an entry property gives that entry. The result is absolute.
func FindEntry(root, file string) (string, bool) {
	absRoot := realPath(mustAbs(root))
	if !filepath.IsAbs(file) {
		file = filepath.Join(absRoot, file)
	}
	file = realPath(file)
	return findEntry(absRoot, file, map[string]bool{file: true})
}

func findEntry(root, file string, seen map[string]bool) (string, bool) {
	if info, ok := readInfo(file); ok && info.entry != "" {
		return info.entry, true
	}
	cand := nearestMounter(root, file)
	if cand == "" || seen[cand] {
		return cand, cand != ""
	}
	seen[cand] = true
	if up, ok := findEntry(root, cand, seen); ok {
		return up, true
	}
	return cand, true
}

// nearestMounter is the first file, folder by folder up from file, that reaches file by mounts.
func nearestMounter(root, file string) string {
	dir := filepath.Dir(file)
	for depth := 0; depth < maxEntryDepth; depth++ {
		ents, _ := os.ReadDir(dir)
		for _, e := range ents {
			p := filepath.Join(dir, e.Name())
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".tenn") || p == file {
				continue
			}
			if reaches(realPath(p), file, map[string]bool{}) {
				return realPath(p)
			}
		}
		parent := filepath.Dir(dir)
		if dir == root || parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// reaches reports whether from is target or mounts it through some chain of files.
func reaches(from, target string, seen map[string]bool) bool {
	if from == target {
		return true
	}
	if seen[from] {
		return false
	}
	seen[from] = true
	info, _ := readInfo(from)
	for _, m := range info.mounts {
		if reaches(m, target, seen) {
			return true
		}
	}
	return false
}

// LoadAuto is LoadOverlay for a file that may be a part of a bigger document. With an entry property
// the file is loaded in that entry's context. Without one, a file that loads with errors is tried
// in the context of the entry FindEntry finds, and that tree is returned when it has fewer errors.
// Tree.Entry says which context was used.
func LoadAuto(root, file string, overlay map[string]string) (*Tree, error) {
	if entry, pos, ok := ExplicitEntry(root, file, overlay); ok {
		tree, err := LoadIn(root, file, entry, overlay)
		if tree != nil {
			return tree, err
		}
		plain, perr := LoadOverlay(root, file, overlay)
		if plain == nil {
			return nil, perr
		}
		plain.fail(pos, "%s: %v", EntryProp, err)
		plain.sortDiags()
		return plain, plain.diags.asError()
	}
	plain, err := LoadOverlay(root, file, overlay)
	if plain == nil || !plain.diags.Fatal() {
		return plain, err
	}
	entry, ok := FindEntry(root, file)
	if !ok {
		return plain, err
	}
	tree, terr := LoadIn(root, file, entry, overlay)
	if tree == nil || tree.diags.fatalCount() >= plain.diags.fatalCount() {
		return plain, err
	}
	return tree, terr
}
