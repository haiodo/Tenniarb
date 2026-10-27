package mcpserver

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/servicewire"
)

// EditOp is one find/replace of the edit tool.
type EditOp struct {
	Find    string `json:"find" jsonschema:"exact text to find; must occur exactly once unless all is true"`
	Replace string `json:"replace" jsonschema:"replacement text, may be empty"`
	All     bool   `json:"all,omitempty" jsonschema:"replace every occurrence"`
}

var posDigits = regexp.MustCompile(`:\d+(:\d+)?`)

// errKey identifies an error independent of where it sits: edits shift line numbers inside messages.
func errKey(e scene.Error) string {
	return e.Pos.File + "\x00" + posDigits.ReplaceAllString(e.Msg, ":N")
}

func applyEdits(text string, edits []EditOp) (string, error) {
	for i, ed := range edits {
		if ed.Find == "" {
			return "", fmt.Errorf("edit %d: find is empty", i+1)
		}
		n := strings.Count(text, ed.Find)
		switch {
		case n == 0:
			return "", fmt.Errorf("edit %d: find text not found (it must match the file exactly, whitespace included)", i+1)
		case n > 1 && !ed.All:
			return "", fmt.Errorf("edit %d: find text matches %d times (lines %s); add context to make it unique or set all", i+1, n, matchLines(text, ed.Find))
		}
		text = strings.ReplaceAll(text, ed.Find, ed.Replace)
	}
	return text, nil
}

func matchLines(text, find string) string {
	var ls []string
	for off := 0; len(ls) < 6; {
		i := strings.Index(text[off:], find)
		if i < 0 {
			break
		}
		ls = append(ls, fmt.Sprint(1+strings.Count(text[:off+i], "\n")))
		off += i + len(find)
	}
	return strings.Join(ls, ", ")
}

// edit applies the edits to file in memory, validates the result and replaces the file only if no
// new error appeared. The result text lists the warnings that remain.
func (s *Server) edit(file string, edits []EditOp) (string, error) {
	abs, rel, err := s.resolve(file)
	if err != nil {
		return "", err
	}
	if len(edits) == 0 {
		return "", errors.New("edits is empty")
	}
	s.host.editMu.Lock()
	defer s.host.editMu.Unlock()

	if d := s.host.live.lookup(abs); d != nil {
		return s.editLive(abs, rel, d, edits)
	}
	old, err := os.ReadFile(abs)
	creating := errors.Is(err, fs.ErrNotExist)
	if err != nil && !creating {
		return "", err
	}
	var text string
	if creating {
		// A new file takes one edit with empty find: its replace is the content.
		if len(edits) != 1 || edits[0].Find != "" {
			return "", fmt.Errorf("%s does not exist: to create it send one edit with empty find", rel)
		}
		text = edits[0].Replace
	} else if text, err = applyEdits(string(old), edits); err != nil {
		return "", err
	}
	if e, bad := parseError(text); bad {
		return "", fmt.Errorf("nothing written: parse error at %s:%d:%d: %s", rel, e.Line+1, e.Col, e.Message)
	}
	var before scene.Errors
	if !creating {
		if before, _, err = s.diagnose(rel); err != nil {
			return "", err
		}
	}
	return s.commit(abs, rel, text, before)
}

// editLive changes a document open in the app: the app applies the new text as one undo step, if
// it is still at the version the edits were made against. The file on disk is not touched, so
// unsaved changes in the app are never overwritten.
func (s *Server) editLive(abs, rel string, d *liveDoc, edits []EditOp) (string, error) {
	if d.moved {
		return "", errMoved
	}
	cur, err := s.host.live.text(abs, true)
	if err != nil {
		return "", err
	}
	text, err := applyEdits(cur.Text, edits)
	if err != nil {
		return "", err
	}
	if e, bad := parseError(text); bad {
		return "", fmt.Errorf("nothing changed: parse error at %s:%d:%d: %s", rel, e.Line+1, e.Col, e.Message)
	}
	before, _, err := s.diagnose(rel)
	if err != nil {
		return "", err
	}
	after, err := s.check(rel, text, before)
	if err != nil {
		return "", err
	}
	var res servicewire.ApplyResult
	if err := d.prov.call(servicewire.MethodApply, servicewire.ApplyArgs{Path: d.path, Base: cur.Version, Text: text}, &res); err != nil {
		return "", fmt.Errorf("nothing changed: Tenniarb did not apply the edit: %v", err)
	}
	return fmt.Sprintf("ok: %s changed in Tenniarb as one undo step, not saved (%d lines)\n%s", rel, strings.Count(text, "\n")+1, summary(after, true, rel)), nil
}

// check validates text as the content of rel and fails when it adds an error to before.
func (s *Server) check(rel, text string, before scene.Errors) (scene.Errors, error) {
	after, _, err := s.diagnoseText(rel, text)
	if err != nil {
		return nil, err
	}
	if bad := newErrors(before, after); len(bad) > 0 {
		return nil, fmt.Errorf("nothing written: the edit adds errors:\n%s", bad)
	}
	return after, nil
}

// commit validates text as the content of rel and, when it adds no error, stages it next to the
// target and renames it over the target.
func (s *Server) commit(abs, rel, text string, before scene.Errors) (string, error) {
	after, err := s.check(rel, text, before)
	if err != nil {
		return "", err
	}
	if err := writeAtomic(abs, text); err != nil {
		return "", err
	}
	return fmt.Sprintf("ok: %s written (%d lines)\n%s", rel, strings.Count(text, "\n")+1, summary(after, true, rel)), nil
}

func writeAtomic(abs, text string) error {
	tmp, err := os.CreateTemp(filepath.Dir(abs), ".tenn-edit-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(text); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	mode := os.FileMode(0o644)
	if st, err := os.Stat(abs); err == nil {
		mode = st.Mode().Perm()
	}
	os.Chmod(tmpName, mode)
	if err := os.Rename(tmpName, abs); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// newErrors returns the errors of after that before did not have; warnings are ignored.
func newErrors(before, after scene.Errors) scene.Errors {
	budget := map[string]int{}
	for _, e := range before {
		if !e.Warn {
			budget[errKey(e)]++
		}
	}
	var out scene.Errors
	for _, e := range after {
		if e.Warn {
			continue
		}
		if k := errKey(e); budget[k] > 0 {
			budget[k]--
		} else {
			out = append(out, e)
		}
	}
	return out
}

// summary lists errors and warnings, or only warnings when warnOnly. With own set, warnings of
// other files (imported libraries) are only counted: they repeat on every call that loads them.
func summary(es scene.Errors, warnOnly bool, own string) string {
	var b strings.Builder
	var nerr, nwarn, other int
	for _, e := range es {
		if e.Warn {
			nwarn++
		} else {
			nerr++
		}
		switch {
		case !e.Warn && warnOnly:
			continue
		case e.Warn && own != "" && e.Pos.File != own:
			other++
			continue
		}
		b.WriteString(e.Error() + "\n")
	}
	if other > 0 {
		fmt.Fprintf(&b, "%d warnings of imported files not listed: validate that file to see them\n", other)
	}
	fmt.Fprintf(&b, "%d errors, %d warnings", nerr, nwarn)
	return b.String()
}
