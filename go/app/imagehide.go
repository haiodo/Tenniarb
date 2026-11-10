package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haiodo/tenniarb/go/tenn"
)

// imageDataMark starts the literal the Code section shows in place of the base64 of a classic
// `image "name" @(base64)` of an old diagram. The space keeps a stub apart from the data it hides:
// base64 has none, so a stub can always be told from data.
const imageDataMark = "image data "

// imageDigestMark ends a stub: the digest of the data it stands for, which finds that data again
// when the text of the pane is written back to the document.
const imageDigestMark = ", hidden "

// imageLit is one `@(data)` literal of an image command in a text of the Code section, by bytes.
type imageLit struct {
	start, end int    // the literal between @( and )
	name       string // the name the command gives the picture
	data       string
}

// hideImageData replaces the base64 of every classic inline image of text by a short stub, so the
// Code section shows that a picture of so many bytes is there and does not hold it. An image from a
// file has no data and stays. A text that does not parse is left as typed, so a draft is never
// mangled.
func hideImageData(text string) string {
	if !strings.Contains(text, "image") || !strings.Contains(text, "@(") {
		return text // no image command, no literal: the common case does not parse
	}
	lits := imageLits(text)
	first := -1
	for i, l := range lits {
		if !strings.HasPrefix(l.data, imageDataMark) { // a stub already: hiding it again would hide the stub
			first = i
			break
		}
	}
	if first < 0 {
		return text
	}
	var b strings.Builder
	b.Grow(1024)
	at := 0
	for _, l := range lits[first:] {
		if strings.HasPrefix(l.data, imageDataMark) {
			continue
		}
		b.WriteString(text[at:l.start])
		b.WriteString(imageStub(l.data))
		at = l.end
	}
	b.WriteString(text[at:])
	return b.String()
}

// restoreImageData puts the data of the inline images of src back in place of the stubs of text, so
// a thing the Code section hid is written to the document whole. A stub that finds neither its data
// nor a picture of its name is left as typed: the text of the pane is the user's, and it stays
// visible instead of silently taking a neighbour's data.
func restoreImageData(text, src string) string {
	if !strings.Contains(text, imageDataMark) {
		return text
	}
	var originals []imageLit
	for _, l := range imageLits(src) {
		if !strings.HasPrefix(l.data, imageDataMark) {
			originals = append(originals, l)
		}
	}
	if len(originals) == 0 {
		return text
	}
	var stubs []imageLit
	for _, l := range imageLits(text) {
		if strings.HasPrefix(l.data, imageDataMark) {
			stubs = append(stubs, l)
		}
	}
	if len(stubs) == 0 {
		return text
	}
	byDigest := map[string]string{}
	digests := make([]string, len(originals))
	for i, o := range originals {
		digests[i] = imageDigest(o.data)
		byDigest[digests[i]] = o.data
	}
	data := map[int]string{} // stub index to the data it stands for
	used := make([]bool, len(originals))
	var left []int
	for i, s := range stubs {
		d := stubDigest(s.data)
		if v, ok := byDigest[d]; ok { // the stub knows what it hides, wherever the line moved
			data[i] = v
			for j := range originals {
				if digests[j] == d {
					used[j] = true
				}
			}
			continue
		}
		left = append(left, i)
	}
	// A stub whose digest was retyped pairs by name and order when the numbers agree, as a renamed
	// kid pairs with the stub that stands for it. Bigger counts mean the user added or deleted lines,
	// and guessing would write one picture's data over another.
	byName := map[string][]int{}
	for j, o := range originals {
		if !used[j] {
			byName[o.name] = append(byName[o.name], j)
		}
	}
	leftByName := map[string][]int{}
	for _, i := range left {
		leftByName[stubs[i].name] = append(leftByName[stubs[i].name], i)
	}
	for name, ids := range leftByName {
		js := byName[name]
		if len(ids) != len(js) {
			continue
		}
		for k, i := range ids {
			data[i] = originals[js[k]].data
		}
	}
	var b strings.Builder
	b.Grow(1024)
	found := false
	at := 0
	for i, s := range stubs {
		v, ok := data[i]
		if !ok {
			continue
		}
		b.WriteString(text[at:s.start])
		b.WriteString(v)
		at = s.end
		found = true
	}
	if !found {
		return text
	}
	b.WriteString(text[at:])
	return b.String()
}

// imageLits lists the `image "name" @(data)` commands of text in file order. Only an image command
// is read: an @(...) of another command is not the data of a picture and stays.
func imageLits(text string) []imageLit {
	ast, p := tenn.Parse(text)
	if p.Errors.HasErrors() {
		return nil
	}
	var out []imageLit
	var walk func(n *tenn.Node)
	walk = func(n *tenn.Node) {
		if n.Kind == tenn.KindCommand {
			if name, ok := n.GetIdent(0); ok && name == "image" {
				pic, _ := n.GetIdent(1)
				for _, a := range n.Children[1:] {
					if a.Kind != tenn.KindImage || a.Token == nil {
						continue
					}
					start, end := a.Token.Pos, a.Token.Pos+a.Token.Size
					if start < 0 || end > len(text) || text[start:end] != a.Token.Literal {
						continue
					}
					out = append(out, imageLit{start: start, end: end, name: pic, data: a.Token.Literal})
				}
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, c := range ast.Children {
		walk(c)
	}
	return out
}

// imageStub is the literal the Code section shows in place of data: how much hides behind it, and
// what finds it again.
func imageStub(data string) string {
	return imageDataMark + dataSize(len(data)) + imageDigestMark + imageDigest(data)
}

// stubDigest is the digest a stub ends with, "" when it is no stub.
func stubDigest(stub string) string {
	i := strings.LastIndex(stub, imageDigestMark)
	if i < 0 {
		return ""
	}
	return strings.TrimSpace(stub[i+len(imageDigestMark):])
}

// dataSize is a byte count as a person reads it, in binary units.
func dataSize(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// imageDigest is FNV-1a of the data, hex: a short name for a picture that a stub keeps, so the same
// bytes are found again even when the line moved. Base64 data is far larger than the digest.
func imageDigest(data string) string {
	const offset64, prime64 = 14695981039346656037, 1099511628211
	h := uint64(offset64)
	for i := 0; i < len(data); i++ {
		h ^= uint64(data[i])
		h *= prime64
	}
	return strconv.FormatUint(h, 16)
}
