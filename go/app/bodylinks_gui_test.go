//go:build darwin || linux || windows

package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

type bodyLink struct {
	Box    []float64
	File   string
	Anchor string
}

type bodyLinkState struct {
	Path      string
	Element   []string
	Banner    string
	Zoom      float64
	OriginX   float64
	OriginY   float64
	BodyLinks []bodyLink
}

// TestBodyLinks clicks the [label](file#anchor) links of a body: the same text scrolls to its heading,
// another file opens on the element with the heading, and a missing file shows in the banner.
func TestBodyLinks(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	app := qatest.App(t)
	dir, err := os.MkdirTemp(os.Getenv("TENNIARB_GUI_DIR"), "bodylinks")
	if err != nil {
		t.Fatal(err)
	}
	// The heading Цель sits far below the link, so the scroll that follows it moves the camera.
	long := strings.Repeat("Строка текста страницы.\n\n", 40)
	page := func(name string, link string) string {
		return "element \"" + name + "\" {\n    item \"Doc\" {\n        auto-layout column\n        width 300\n" +
			"        body %{\n            # План\n\n            " + link + "\n\n            " + long +
			"            ## Цель\n\n            Финал.\n        }\n    }\n}\n"
	}
	fileA := filepath.Join(dir, "A.tenn")
	textA := page("One", "[К цели](#Цель)") + page("Two", "[Туда](B.tenn#Цель)") + page("Three", "[Нет](missing.md)")
	if err := os.WriteFile(fileA, []byte(textA), 0o644); err != nil {
		t.Fatal(err)
	}
	fileB := filepath.Join(dir, "B.tenn")
	textB := "element \"Target\" {\n    item \"Doc\" {\n        auto-layout column\n        width 300\n" +
		"        body %{\n            ## Цель\n\n            Текст.\n        }\n    }\n}\n"
	if err := os.WriteFile(fileB, []byte(textB), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		app.Answer(qa.Dialog{Kind: "message", Reply: "No"})
		app.State("command", "name", "open-document", "path", fixture)
		app.State("command", "name", "close-others")
	})

	read := func(t *testing.T) (s bodyLinkState) {
		t.Helper()
		if err := json.Unmarshal(app.State("state"), &s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	canvas := app.Find(qa.ID("canvas"))
	selectElement := func(t *testing.T, name string) bodyLinkState {
		t.Helper()
		app.Find(qa.Text(name).In(qa.ID("outline"))).Click()
		app.Settle()
		return read(t)
	}
	followLink := func(t *testing.T, s bodyLinkState) bodyLinkState {
		t.Helper()
		if len(s.BodyLinks) == 0 {
			t.Fatalf("no links in the body: %+v", s)
		}
		b := s.BodyLinks[0].Box
		canvas.ClickAt(b[0]+b[2]/2, b[1]+b[3]/2)
		app.Settle()
		return read(t)
	}

	app.State("command", "name", "open-document", "path", fileA)
	app.State("command", "name", "close-others")
	app.Settle()

	// the same text: the heading moves into the middle of the canvas
	before := selectElement(t, "One")
	if len(before.BodyLinks) != 1 || before.BodyLinks[0].File != "" || before.BodyLinks[0].Anchor != "цель" {
		t.Fatalf("link of One: %+v", before.BodyLinks)
	}
	if after := followLink(t, before); after.Path != fileA || (after.OriginY == before.OriginY && after.OriginX == before.OriginX) {
		t.Fatalf("the camera did not scroll to the heading: %v,%v -> %v,%v", before.OriginX, before.OriginY, after.OriginX, after.OriginY)
	}

	// a file that is not there: a banner, the page stays
	if after := followLink(t, selectElement(t, "Three")); after.Path != fileA || !strings.Contains(after.Banner, "no such file missing.md") {
		t.Fatalf("missing file: path %q banner %q", after.Path, after.Banner)
	}

	// another file: it opens on the element whose body has the heading
	if after := followLink(t, selectElement(t, "Two")); after.Path != fileB || len(after.Element) != 1 || after.Element[0] != "Target" {
		t.Fatalf("another file: path %q element %v banner %q", after.Path, after.Element, after.Banner)
	}
}
