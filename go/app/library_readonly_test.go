//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
)

const builtinLibDoc = "item \"zap\" {\n\twidth 24; height 24\n}\n"

// builtinLibEnv writes the library the scheme resolves and points the lookup at it, so no library
// installed on the machine can stand in.
func builtinLibEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "icons.tenn"), []byte(builtinLibDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TENNIARB_LIBRARIES", dir)
	t.Setenv("TENNIARB_CONFIG_DIR", t.TempDir())
	return dir
}

// A library mounted by scheme is read-only: an edit of its content is refused with a reason, nothing is
// held as unsaved, and no copy of it lands in the workspace.
func TestBuiltinLibraryMountIsReadOnly(t *testing.T) {
	builtinLibEnv(t)
	dir := t.TempDir()
	text := "element \"icons\" --from-file \"tenniarb-library://icons\"\n" +
		"element \"S\" {\n\tuse icons.zap as Z { width 24; height 24 }\n}\n"
	v := newDoc(t, &Shell{}, filepath.Join(dir, "main.tenn"), text)
	if v.stree == nil {
		t.Fatalf("no scene: %s", v.sceneErr)
	}
	const file = ".tenniarb-library/icons.tenn"
	if v.stree.File(file) == nil {
		t.Fatalf("the library is not mounted: %v", v.stree.Files())
	}
	zap := NodeAt(v.stree.File(file), "zap")
	if zap == nil {
		t.Fatal("no zap in the mounted library")
	}
	e, ok := SetPropEdit(v.fileText(file), zap, "width", "30")
	if !ok {
		t.Fatal("no edit of the width")
	}
	if err := v.applyFileText(file, e.Start, e.End, e.New); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("an edit of a built-in library was not refused: %v", err)
	}
	if v.text != text || v.libsDirty() {
		t.Fatalf("a refused edit changed the document: dirty %v\n%s", v.libsDirty(), v.text)
	}
	// a classic edit inside the mount is refused before the model changes
	kit := v.session.Root().Elements[0]
	if err := v.session.Store().Add(kit, model.NewElement("X"), nil, nil, nil); err != model.ErrMounted {
		t.Fatalf("add into a built-in library: %v", err)
	}
	if v.libErr == nil || !strings.Contains(v.libErr.Error(), "read-only") {
		t.Fatalf("the refusal does not name the library: %v", v.libErr)
	}
	if err := v.saveLibs(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(file))); !os.IsNotExist(err) {
		t.Fatalf("a save put a copy into the workspace: %v", err)
	}
}

// A library file opened as a document reads, but its window is read-only and says why: no edit is
// taken and Save writes nothing.
func TestBuiltinLibraryDocumentIsReadOnly(t *testing.T) {
	libs := builtinLibEnv(t)
	path := filepath.Join(libs, "icons.tenn")
	v := newDoc(t, &Shell{}, path, builtinLibDoc)
	if !v.builtin || !v.session.Readonly() {
		t.Fatalf("a built-in library as a document: builtin %v, readonly %v", v.builtin, v.session.Readonly())
	}
	if got := v.bannerText(); !strings.Contains(got, "read-only") {
		t.Fatalf("the banner does not say why: %q", got)
	}
	if err := v.applyText(0, len(v.text), builtinLibDoc+"\n"); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("a text edit of a built-in library was not refused: %v", err)
	}
	if err := v.writeFile(path); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("a save of a built-in library was not refused: %v", err)
	}
	if got := fileText(t, path); got != builtinLibDoc {
		t.Fatalf("the library file changed:\n%s", got)
	}
}
