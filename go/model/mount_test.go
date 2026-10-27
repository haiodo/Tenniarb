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
