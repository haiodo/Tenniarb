//go:build darwin || linux || windows

package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
)

const shadowDoc = `element "D" {
  styles {
    item {
      shadow 0 -4 12 #00000010
      shadow 0 -10 10 #00000010
    }
  }
  item "A" { pos 0 0 }
}`

// An item that writes no shadow of its own is drawn with the item defaults of the element: the list
// shows those layers, and changing one writes them all as the item's own, an override.
func TestShownShadowsFallBackToItemDefaults(t *testing.T) {
	v := newDoc(t, &Shell{}, filepath.Join(t.TempDir(), "s.tenn"), shadowDoc)
	el := v.session.Root().Elements[0]
	v.session.SetElement(el)
	v.session.Select([]*model.DiagramItem{el.Items[0]})
	if !v.shadowsInherited() {
		t.Fatal("the item writes no shadow: the list shows the defaults")
	}
	layers := v.shownShadows()
	if len(layers) != 2 {
		t.Fatalf("%d layers, want 2", len(layers))
	}
	if !v.setShadows(layers) {
		t.Fatal("refused")
	}
	if v.shadowsInherited() {
		t.Fatal("the item writes its own shadows now")
	}
	if n := strings.Count(v.text, "shadow 0 -"); n != 4 {
		t.Fatalf("%d shadow commands, want 2 in the item and 2 in the defaults:\n%s", n, v.text)
	}
}
