package main

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// fixtures is the corpus the scene golden and range stands read too.
var fixtures = filepath.Join("..", "..", "..", "web", "fixtures", "scene")

// TestLibrariesOfScreen: the screen that mounts design-kit.tenn lists the kit under the name its
// element gives it, and the paths are the ones the screen writes as use kit....
func TestLibrariesOfScreen(t *testing.T) {
	out, err := all(fixtures, fixtures, "", 200)
	if err != nil {
		t.Fatal(err)
	}
	var docs []document
	if err := json.Unmarshal(out, &docs); err != nil {
		t.Fatal(err)
	}
	var screen *document
	for i := range docs {
		if docs[i].File == "library-screen.tenn" {
			screen = &docs[i]
		}
	}
	if screen == nil {
		t.Fatalf("library-screen.tenn is not listed")
	}
	if len(screen.Mounts) != 1 || screen.Mounts[0].Alias != "kit" {
		t.Fatalf("mounts = %+v, want the kit library", screen.Mounts)
	}
	kit := screen.Mounts[0]
	if kit.File != "design-kit.tenn" {
		t.Errorf("mount file = %q, want design-kit.tenn", kit.File)
	}
	for _, c := range kit.Components {
		if c.Path == "kit.Button" && c.Name == "Button" {
			return
		}
	}
	t.Errorf("kit.Button is not listed: %+v", kit.Components)
}

// TestQueryAndLimit: q narrows to the components whose name holds it, limit caps each library.
func TestQueryAndLimit(t *testing.T) {
	plain, err := listing(fixtures, "library-screen.tenn", "", 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(plain.Mounts) != 1 || len(plain.Mounts[0].Components) < 2 {
		t.Fatalf("components = %+v, want more than one", plain.Mounts)
	}
	full := plain.Mounts[0].Components

	q, err := listing(fixtures, "library-screen.tenn", "badge", 200)
	if err != nil {
		t.Fatal(err)
	}
	got := q.Mounts[0].Components
	if len(got) != 1 || got[0].Path != "kit.Badge" || len(got) >= len(full) {
		t.Errorf("q badge = %+v, want only kit.Badge of %d", got, len(full))
	}

	capped, err := listing(fixtures, "library-screen.tenn", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	got = capped.Mounts[0].Components
	if len(got) != 1 || got[0] != full[0] {
		t.Errorf("limit 1 = %+v, want the first of %d", got, len(full))
	}
}
