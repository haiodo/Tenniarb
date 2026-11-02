//go:build darwin || linux || windows

package app_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

// pagesDoc nests elements three levels deep: Chapter has Section, Section has Part, Chapter has Aside.
const pagesDoc = `element "Book" {
    element "Chapter" {
        item "A" { pos 0 0; width 40; height 20 }
        element "Section" {
            item "B" { pos 0 0; width 40; height 20 }
            element "Part" { item "C" { pos 0 0; width 40; height 20 } }
        }
        element "Aside" { item "D" { pos 0 0; width 40; height 20 } }
    }
    element "Appendix" { item "E" { pos 0 0; width 40; height 20 } }
}
`

// pdfPageCount is the page count of a PDF, read from the /Count of its page tree.
func pdfPageCount(data []byte) int {
	n := 0
	for i := 0; ; {
		j := bytes.Index(data[i:], []byte("/Count "))
		if j < 0 {
			return n
		}
		i += j + len("/Count ")
		k := i
		for k < len(data) && data[k] >= '0' && data[k] <= '9' {
			k++
		}
		if v, err := strconv.Atoi(string(data[i:k])); err == nil && v > n {
			n = v
		}
	}
}

func fileNames(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		out = append(out, e.Name())
	}
	return out
}

// TestExportPages drives Element > Export Pages and the same submenu of the outline: PDF is one
// document of a page per element, PNG a file per element in the chosen folder.
func TestExportPages(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	dir := t.TempDir()
	file := filepath.Join(dir, "Book.tenn")
	if err := os.WriteFile(file, []byte(pagesDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	// the later tests start on the fixture, as they find it
	t.Cleanup(func() {
		h.open(fixture)
		a.State("command", "name", "close-others")
	})
	h.open(file)
	a.State("command", "name", "close-others")
	a.Find(qa.Text("Chapter").In(qa.ID("outline"))).Click()
	a.Settle()

	// the item of the Element menu
	titles := menuTitles(a.Dump())
	for _, want := range []string{"Export Pages", "PNG...", "PDF..."} {
		if !slices.Contains(titles, want) {
			t.Fatalf("no menu item %q in %q", want, titles)
		}
	}

	// PDF: one file, a page per element
	pdf := filepath.Join(dir, "Book.pdf")
	a.Answer(qa.Dialog{Kind: "file", Reply: pdf})
	a.Answer(qa.Dialog{Kind: "message", Reply: "OK"})
	a.Find(qa.Text("PDF...").In(qa.Selector{Class: "MenuItem", Text: "Element"})).Click()
	a.Settle()
	data, err := os.ReadFile(pdf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) || len(data) < 1000 {
		t.Fatalf("pdf of %d bytes", len(data))
	}
	if n := pdfPageCount(data); n != 4 {
		t.Fatalf("pdf has %d pages, want 4", n)
	}
	if msg := lastMessage(a); !strings.Contains(msg, "4 pages written to "+pdf) {
		t.Fatalf("report after the PDF: %q", msg)
	}

	// PNG: the same submenu in the context menu of the outline, a file per element in a folder
	out := filepath.Join(dir, "pages")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	tree := a.Find(qa.ID("outline"))
	tree.RightClickAt(40, 12)
	a.Settle()
	if !a.Find(qa.Text("Export Pages").In(qa.ID("outline"))).Exists() {
		t.Fatal("no Export Pages in the context menu of the outline")
	}
	a.Answer(qa.Dialog{Kind: "dir", Reply: out})
	a.Answer(qa.Dialog{Kind: "message", Reply: "OK"})
	a.Find(qa.Text("PNG...").In(qa.ID("outline"))).Click()
	a.Settle()
	want := []string{"01-Chapter.png", "02-Section.png", "03-Part.png", "04-Aside.png"}
	if got := fileNames(t, out); !slices.Equal(got, want) {
		t.Fatalf("pages %v, want %v", got, want)
	}
	for _, n := range want {
		img, err := os.ReadFile(filepath.Join(out, n))
		if err != nil || !bytes.HasPrefix(img, []byte("\x89PNG")) || len(img) < 100 {
			t.Fatalf("%s: %d bytes, %v", n, len(img), err)
		}
	}
	if msg := lastMessage(a); !strings.Contains(msg, "4 pages written to "+out) {
		t.Fatalf("report after the PNG export: %q", msg)
	}
}

// lastMessage is the text of the last message box the app opened.
func lastMessage(a *qatest.Client) string {
	msg := ""
	for _, d := range a.Dialogs() {
		if d.Kind == "message" {
			msg = d.Message
		}
	}
	return msg
}
