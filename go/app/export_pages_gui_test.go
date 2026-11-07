//go:build darwin || linux || windows

package app_test

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

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

// pdfPageCount is the page count of a PDF, read from the /Count of its page tree, which cairo writes
// into a compressed object stream.
func pdfPageCount(data []byte) int {
	n := pdfMaxCount(data)
	if m := pdfMaxCount(pdfStreams(data)); m > n {
		n = m
	}
	return n
}

// pdfMaxCount is the largest /Count of the text.
func pdfMaxCount(data []byte) int {
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

// pdfStreams is what the FlateDecode streams of a PDF hold, joined.
func pdfStreams(data []byte) []byte {
	var out bytes.Buffer
	for i := 0; ; {
		j := bytes.Index(data[i:], []byte("stream"))
		if j < 0 {
			return out.Bytes()
		}
		j += i
		i = j + len("stream")
		if j > 0 && data[j-1] == 'd' { // endstream
			continue
		}
		for i < len(data) && (data[i] == '\r' || data[i] == '\n') {
			i++
		}
		k := bytes.Index(data[i:], []byte("endstream"))
		if k < 0 {
			return out.Bytes()
		}
		zr, err := zlib.NewReader(bytes.NewReader(data[i : i+k]))
		if err != nil {
			i += k + len("endstream")
			continue
		}
		if b, err := io.ReadAll(zr); err == nil {
			out.Write(b)
		}
		i += k + len("endstream")
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
	if msgs := messageDialogs(a); len(msgs) != 0 {
		t.Fatalf("message boxes after the PDF: %v", msgs)
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
	if msgs := messageDialogs(a); len(msgs) != 0 {
		t.Fatalf("message boxes after the PNG export: %v", msgs)
	}
}

// TestExportPagesQuiet is the pages export in the background: the progress item comes and goes with the
// export, nothing is left open once it is done, and only a file that cannot be written is a box.
func TestExportPagesQuiet(t *testing.T) {
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
	t.Cleanup(func() {
		h.open(fixture)
		a.State("command", "name", "close-others")
	})
	h.open(file)
	a.State("command", "name", "close-others")
	a.Find(qa.Text("Chapter").In(qa.ID("outline"))).Click()
	a.Settle()

	pdf := filepath.Join(dir, "Book.pdf")
	a.Answer(qa.Dialog{Kind: "file", Reply: pdf})
	a.Find(qa.Text("PDF...").In(qa.Selector{Class: "MenuItem", Text: "Element"})).Click()
	a.Settle()
	if data, err := os.ReadFile(pdf); err != nil || !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatalf("pdf: %d bytes, %v", len(data), err)
	}
	if msgs := messageDialogs(a); len(msgs) != 0 {
		t.Fatalf("message boxes after a good export: %v", msgs)
	}
	if a.Find(qa.ID("export-progress")).Exists() {
		t.Fatal("the progress item is still in the toolbar")
	}
	var st struct {
		Export struct {
			Active      bool
			Done, Total int
			Dest        string
		}
	}
	if err := json.Unmarshal(a.State("state"), &st); err != nil {
		t.Fatal(err)
	}
	if st.Export.Active || st.Export.Done != 4 || st.Export.Total != 4 || st.Export.Dest != pdf {
		t.Fatalf("export state %+v", st.Export)
	}

	// a file that cannot be written is the one message box, and the progress item goes with it
	missing := filepath.Join(dir, "missing", "Book.pdf")
	a.Answer(qa.Dialog{Kind: "file", Reply: missing})
	a.Answer(qa.Dialog{Kind: "message", Reply: "OK"})
	a.Find(qa.Text("PDF...").In(qa.Selector{Class: "MenuItem", Text: "Element"})).Click()
	a.Settle()
	if msgs := messageDialogs(a); len(msgs) != 1 || msgs[0].Title != "Export failed" || !strings.Contains(msgs[0].Message, "missing") {
		t.Fatalf("message boxes after the failed export: %v", msgs)
	}
	if a.Find(qa.ID("export-progress")).Exists() {
		t.Fatal("the progress item is still in the toolbar after the failure")
	}
}

// TestExportNoteGoesAway pins that a finished export says what it did for a few seconds, not forever:
// the note takes the banner, and the banner is the user's again after it.
func TestExportNoteGoesAway(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	dir := t.TempDir()
	// "Empty" has nothing to draw, so it is the page the export skips and the banner has to name.
	doc := strings.Replace(pagesDoc, `    element "Appendix" { item "E" { pos 0 0; width 40; height 20 } }`,
		"    element \"Appendix\" { item \"E\" { pos 0 0; width 40; height 20 } }\n    element \"Empty\" {}", 1)
	file := filepath.Join(dir, "Book.tenn")
	if err := os.WriteFile(file, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		h.open(fixture)
		a.State("command", "name", "close-others")
	})
	h.open(file)
	a.State("command", "name", "close-others")
	a.Find(qa.Text("Book").In(qa.ID("outline"))).Click()
	a.Settle()

	pdf := filepath.Join(dir, "Book.pdf")
	a.Answer(qa.Dialog{Kind: "file", Reply: pdf})
	a.Find(qa.Text("PDF...").In(qa.Selector{Class: "MenuItem", Text: "Element"})).Click()
	a.Settle()
	if !bannerHas(a, "pages written") {
		t.Fatalf("no note about the export in the banner: %v", bannerTexts(a))
	}
	a.Eventually(9*time.Second, func() (any, bool) {
		a.Settle()
		return bannerTexts(a), !bannerHas(a, "pages written")
	})
}

// bannerTexts is every label text of the window, for the banner assertions: the note lives in a label.
func bannerTexts(a *qatest.Client) []string {
	var out []string
	var walk func([]qa.Node)
	walk = func(list []qa.Node) {
		for _, n := range list {
			out = append(out, n.Text, n.Value)
			walk(n.Children)
		}
	}
	walk(a.Dump())
	return out
}

// bannerHas reports whether some label shows text with the part in it.
func bannerHas(a *qatest.Client, part string) bool {
	for _, s := range bannerTexts(a) {
		if strings.Contains(s, part) {
			return true
		}
	}
	return false
}

// messageDialogs are the message boxes the app opened since App(t).
func messageDialogs(a *qatest.Client) []qa.Dialog {
	var out []qa.Dialog
	for _, d := range a.Dialogs() {
		if d.Kind == "message" {
			out = append(out, d)
		}
	}
	return out
}
