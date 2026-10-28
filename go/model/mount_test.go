package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/scene"
)

const mountMain = "element \"kit\" --from-file \"lib.tenn\"\nelement \"S\" {\n    item \"A\" { pos 0 0 }\n}\n"

func mountedModel(t *testing.T) *ElementModel {
	t.Helper()
	root := t.TempDir()
	lib := "element \"Parts\" {\n    item \"B\" { pos 1 1 }\n}\nitem \"Card\" { width 10 }\n"
	for name, text := range map[string]string{"main.tenn": mountMain, "lib.tenn": lib} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tree, err := scene.Load(root, "main.tenn")
	if err != nil {
		t.Fatal(err)
	}
	m := ReadTenn(mountMain)
	MountFiles(m, tree, "main.tenn")
	return m
}

func TestMountFilesAndPrint(t *testing.T) {
	m := mountedModel(t)
	kit := m.Elements[0]
	if kit.FromFile != "lib.tenn" || len(kit.Elements) != 1 || kit.Elements[0].Name != "Parts" || kit.Elements[0].Parent != kit {
		t.Fatalf("mounted content: %+v", kit)
	}
	if kit.Properties.Count() != 1 {
		t.Errorf("root-level item of the file must be a property of the element, got %d", kit.Properties.Count())
	}
	out := ToTennStr(&m.Element, true, true)
	if strings.Contains(out, "Parts") || strings.Contains(out, "Card") || !strings.Contains(out, "--from-file") {
		t.Errorf("the printer must write the line only:\n%s", out)
	}
	if again := ReadTenn(out); again.Elements[0].FromFile != "lib.tenn" || len(again.Elements) != 2 {
		t.Errorf("round trip lost the line: %s", out)
	}
}

func TestMountedIsReadOnly(t *testing.T) {
	m := mountedModel(t)
	store := NewElementModelStore(m)
	kit := m.Elements[0]
	if err := store.Add(kit, NewElement("X"), nil, nil, nil); err != ErrMounted {
		t.Errorf("add into the mounted element: %v", err)
	}
	if err := store.Add(kit.Elements[0], NewElement("X"), nil, nil, nil); err != ErrMounted {
		t.Errorf("add deeper in the mount: %v", err)
	}
	if err := store.UpdateElementName(kit, "kit2", nil, nil); err != nil {
		t.Errorf("renaming the mounting element edits its line only: %v", err)
	}
	if err := store.Add(m.Elements[1], NewElement("Y"), nil, nil, nil); err != nil {
		t.Errorf("an ordinary element must stay editable: %v", err)
	}
}

func TestMountEditNeedsWriteBack(t *testing.T) {
	m := mountedModel(t)
	kit := m.Elements[0]
	store := NewElementModelStore(m)
	store.MountWritable = func(*Element) bool { return false }
	store.OnMountEdit = func(*Element) { t.Error("a refused edit must not write") }
	if err := store.Add(kit, NewElement("X"), nil, nil, nil); err != ErrMounted {
		t.Errorf("a mount that cannot be written back refuses: %v", err)
	}
}

// Edit, undo and redo inside a mount tell the store's owner which mount to write; the document's own
// elements and the rename of the mounting element do not.
func TestMountEditWritesBack(t *testing.T) {
	m := mountedModel(t)
	kit := m.Elements[0]
	var got []*Element
	store := NewElementModelStore(m)
	store.MountWritable = func(*Element) bool { return true }
	store.OnMountEdit = func(e *Element) { got = append(got, e) }
	um := &UndoManager{GroupsByEvent: true}
	if err := store.Add(kit.Elements[0], NewElement("X"), um, nil, nil); err != nil {
		t.Fatal(err)
	}
	um.CloseAutoGroup()
	if len(got) != 1 || got[0] != kit || len(kit.Elements[0].Elements) != 1 {
		t.Fatalf("edit: %v", got)
	}
	um.Undo()
	um.Redo()
	if len(got) != 3 || got[1] != kit || got[2] != kit {
		t.Errorf("undo and redo write too: %d", len(got))
	}
	if err := store.UpdateElementName(kit, "kit2", nil, nil); err != nil || len(got) != 3 {
		t.Errorf("renaming the mounting element edits its line: %v", err)
	}
	if err := store.Add(m.Elements[1], NewElement("Y"), nil, nil, nil); err != nil || len(got) != 3 {
		t.Errorf("an element of the document: %v", err)
	}
	if err := store.Move(m.Elements[1], kit, nil, nil, 0); err != ErrMounted {
		t.Errorf("moving between files: %v", err)
	}
}

// A mount read again after its file changed keeps its elements while the names pair up, and one that
// was edited here is not read at all.
func TestMountRefillKeepsElements(t *testing.T) {
	root := t.TempDir()
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main.tenn", mountMain)
	write("lib.tenn", "element \"Parts\" {\n    item \"B\" { pos 1 1 }\n}\n")
	load := func(m *ElementModel, keep ...*Element) []Mounted {
		tree, err := scene.Load(root, "main.tenn")
		if err != nil {
			t.Fatal(err)
		}
		return MountFiles(m, tree, "main.tenn", keep...)
	}
	m := ReadTenn(mountMain)
	if got := load(m); len(got) != 1 || !got[0].Fresh || got[0].File != "lib.tenn" {
		t.Fatalf("first read: %+v", got)
	}
	kit := m.Elements[0]
	parts, item := kit.Elements[0], kit.Elements[0].Items[0]
	write("lib.tenn", "element \"Parts\" {\n    item \"B\" { pos 2 2 }\n}\n")
	if got := load(m); !got[0].Fresh || kit.Elements[0] != parts || parts.Items[0] == item || parts.Items[0].X != 2 {
		t.Fatalf("a changed file refills the elements it has: %+v", got)
	}
	item = parts.Items[0]
	write("lib.tenn", "element \"Parts\" {\n    item \"B\" { pos 3 3 }\n}\n")
	if got := load(m, kit); got[0].Fresh || parts.Items[0] != item || item.X != 2 {
		t.Fatalf("an edited mount is not read again: %+v", got)
	}
	write("lib.tenn", "element \"Other\" {\n}\n")
	load(m)
	if kit.Elements[0] == parts || kit.Elements[0].Name != "Other" {
		t.Errorf("other names replace the elements")
	}
}

// A file shown by two elements is parsed once, and a change of one file leaves the other mounts alone.
func TestMountParsesEachFileOnce(t *testing.T) {
	root := t.TempDir()
	main := "element \"a\" --from-file \"x.tenn\"\nelement \"b\" --from-file \"x.tenn\"\nelement \"c\" --from-file \"y.tenn\"\n"
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main.tenn", main)
	write("x.tenn", "element \"P\" {\n    item \"X\" { pos 1 1 }\n}\n")
	write("y.tenn", "element \"P\" {\n    item \"Y\" { pos 2 2 }\n}\n")
	reads := 0
	readTenn = func(src string) *ElementModel { reads++; return ReadTenn(src) }
	defer func() { readTenn = ReadTenn }()
	load := func(m *ElementModel) []Mounted {
		tree, err := scene.Load(root, "main.tenn")
		if err != nil {
			t.Fatal(err)
		}
		return MountFiles(m, tree, "main.tenn")
	}
	m := ReadTenn(main)
	got := load(m)
	a, b, c := m.Elements[0], m.Elements[1], m.Elements[2]
	pa, pb := a.Elements[0], b.Elements[0]
	if reads != 2 || len(got) != 3 || len(pa.Items) != 1 || len(pb.Items) != 1 || pa.Items[0] == pb.Items[0] || pb.Items[0].Parent != pb {
		t.Fatalf("reads=%d mounted=%d: each element has its own copy of the items", reads, len(got))
	}
	reads = 0
	if got = load(m); reads != 0 || got[0].Fresh {
		t.Errorf("unchanged files are not read again: reads=%d", reads)
	}
	write("y.tenn", "element \"P\" {\n    item \"Y\" { pos 3 3 }\n}\n")
	got = load(m)
	if reads != 1 || got[0].Fresh || got[1].Fresh || !got[2].Fresh || c.Elements[0].Items[0].X != 3 {
		t.Errorf("only the changed file is read: reads=%d %+v", reads, got)
	}
}
