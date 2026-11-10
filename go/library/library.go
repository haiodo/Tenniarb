// Package library names and finds the libraries a document mounts with --from-file: a .tenn file of
// components, shipped with the app or installed from git.
package library

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// scheme is the prefix a document writes in place of a path.
const scheme = "tenniarb-library://"

// nameRe is one segment of a library name.
var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Path is what a document writes after tenniarb-library://: a name, or owner/name for an installed library.
type Path struct{ Owner, Name string }

func IsScheme(s string) bool {
	return strings.HasPrefix(s, scheme)
}

// Parse reads "tenniarb-library://haiodo/c4"; a trailing ".tenn" is accepted and dropped.
func Parse(s string) (Path, error) {
	rest, ok := strings.CutPrefix(strings.ToLower(s), scheme)
	if !ok {
		return Path{}, fmt.Errorf("%q is not a %s name", s, scheme)
	}
	rest = strings.TrimSuffix(rest, ".tenn")
	parts := strings.Split(rest, "/")
	if len(parts) > 2 {
		return Path{}, fmt.Errorf("library %q: want name or owner/name, each matching %s", rest, nameRe)
	}
	for _, seg := range parts { // an empty owner or name fails here, so no "/x" is taken for a name
		if err := checkName(seg); err != nil {
			return Path{}, err
		}
	}
	p := Path{Name: parts[0]}
	if len(parts) == 2 {
		p = Path{Owner: parts[0], Name: parts[1]}
	}
	return p, nil
}

// String is the form a document writes.
func (p Path) String() string {
	if p.Owner == "" {
		return scheme + p.Name
	}
	return scheme + p.Owner + "/" + p.Name
}

// Rel is the file below a root.
func (p Path) Rel() string {
	if p.Owner == "" {
		return p.Name + ".tenn"
	}
	return p.Owner + "/" + p.Name + ".tenn"
}

// valid also guards a Path built by hand, so no name can leave its root.
func (p Path) valid() error {
	if p.Owner == "" {
		return checkName(p.Name)
	}
	return errors.Join(checkName(p.Name), checkName(p.Owner))
}

func checkName(seg string) error {
	if nameRe.MatchString(seg) {
		return nil
	}
	return fmt.Errorf("library name %q: must match %s, lowercase letters digits and dashes only", seg, nameRe)
}

// Roots are the two directories a scheme resolves in.
type Roots struct{ Installed, Shipped string }

// Resolve tries Installed first: an installed copy shadows a shipped one.
func (r Roots) Resolve(p Path) (string, error) {
	var f Finder
	if r.Installed != "" {
		f.Installed = []string{r.Installed}
	}
	if r.Shipped != "" {
		f.Shipped = []string{r.Shipped}
	}
	return f.Resolve(p)
}

// Entry is one line of the index of installed libraries.
type Entry struct {
	Owner string `json:"owner"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	Ref   string `json:"ref"`
}

// IndexName is the file name of the index of installed libraries.
func IndexName() string { return "libraries.json" }

// LoadIndex reads the index of dir; a missing file is an empty list, not an error.
func LoadIndex(dir string) ([]Entry, error) {
	file := filepath.Join(dir, IndexName())
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var list []Entry
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return list, nil
}

// SaveIndex writes the index of dir sorted by owner then name.
func SaveIndex(dir string, list []Entry) error {
	sorted := append([]Entry(nil), list...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Owner != sorted[j].Owner {
			return sorted[i].Owner < sorted[j].Owner
		}
		return sorted[i].Name < sorted[j].Name
	})
	data, err := json.MarshalIndent(sorted, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, IndexName()), data)
}

// writeAtomic writes through a temporary file in the same folder, so a reader sees the old file or the
// new one whole.
func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, err = tmp.Write(data)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		os.Remove(name)
	}
	return err
}
