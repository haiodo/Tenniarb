// Package wiretest has a fake desktop app for tests of the provider protocol: documents held in
// memory with versions and an undo stack.
package wiretest

import (
	"errors"
	"fmt"
	"sync"

	"github.com/haiodo/tenniarb/go/servicewire"
)

type doc struct {
	text    string
	version int64
	dirty   bool
	undo    []string
}

// Fake implements servicewire.Provider.
type Fake struct {
	Client *servicewire.ProviderClient // set after RunProvider; Open and Type report through it
	// BeforeApply runs at the start of Apply, outside the lock: a test types there to race the agent.
	BeforeApply func()

	mu       sync.Mutex
	docs     map[string]*doc
	sel      servicewire.Selection
	selected []servicewire.SelectArgs
	presence []servicewire.Presence
}

func New() *Fake { return &Fake{docs: map[string]*doc{}} }

// Open opens path with text as if loaded from disk.
func (f *Fake) Open(path, text string) error {
	f.mu.Lock()
	f.docs[path] = &doc{text: text, version: 1}
	f.mu.Unlock()
	return f.report()
}

// Close closes path, unsaved changes dropped.
func (f *Fake) Close(path string) error {
	f.mu.Lock()
	delete(f.docs, path)
	f.mu.Unlock()
	return f.report()
}

// Type changes the text as the user would, unsaved.
func (f *Fake) Type(path, text string) error {
	f.mu.Lock()
	d := f.docs[path]
	d.undo = append(d.undo, d.text)
	d.text, d.dirty = text, true
	d.version++
	f.mu.Unlock()
	return f.report()
}

// Undo reverts the last change.
func (f *Fake) Undo(path string) error {
	f.mu.Lock()
	d := f.docs[path]
	d.text = d.undo[len(d.undo)-1]
	d.undo = d.undo[:len(d.undo)-1]
	d.version++
	f.mu.Unlock()
	return f.report()
}

// TextOf returns the text and the undo depth of path.
func (f *Fake) TextOf(path string) (string, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := f.docs[path]
	if d == nil {
		return "", 0
	}
	return d.text, len(d.undo)
}

// Selected lists the Select calls received.
func (f *Fake) Selected() []servicewire.SelectArgs {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]servicewire.SelectArgs(nil), f.selected...)
}

// Presence records a notification from the service.
func (f *Fake) Presence(p servicewire.Presence) {
	f.mu.Lock()
	f.presence = append(f.presence, p)
	f.mu.Unlock()
}

// Presences lists the Presence notifications received, in arrival order.
func (f *Fake) Presences() []servicewire.Presence {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]servicewire.Presence(nil), f.presence...)
}

func (f *Fake) SetSelection(s servicewire.Selection) {
	f.mu.Lock()
	f.sel = s
	f.mu.Unlock()
}

func (f *Fake) report() error {
	if f.Client == nil {
		return nil
	}
	f.mu.Lock()
	var docs []servicewire.Doc
	for p, d := range f.docs {
		docs = append(docs, servicewire.Doc{Path: p, Version: d.version, Dirty: d.dirty})
	}
	f.mu.Unlock()
	return f.Client.SetDocuments(docs)
}

func (f *Fake) Text(path string) (servicewire.TextResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := f.docs[path]
	if d == nil {
		return servicewire.TextResult{}, fmt.Errorf("%s is not open", path)
	}
	return servicewire.TextResult{Text: d.text, Version: d.version, Dirty: d.dirty}, nil
}

func (f *Fake) Apply(path string, base int64, text string) (int64, error) {
	if f.BeforeApply != nil {
		f.BeforeApply()
	}
	f.mu.Lock()
	d := f.docs[path]
	if d == nil {
		f.mu.Unlock()
		return 0, fmt.Errorf("%s is not open", path)
	}
	if d.version != base {
		f.mu.Unlock()
		return 0, fmt.Errorf("document changed: version %d, edit was made for %d", d.version, base)
	}
	d.undo = append(d.undo, d.text)
	d.text, d.dirty = text, true
	d.version++
	v := d.version
	f.mu.Unlock()
	return v, f.report()
}

func (f *Fake) Selection() (servicewire.Selection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sel, nil
}

func (f *Fake) Select(path string, nodes []string, reveal bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.docs[path] == nil {
		return errors.New("not open")
	}
	f.selected = append(f.selected, servicewire.SelectArgs{Path: path, Nodes: nodes, Reveal: reveal})
	f.sel = servicewire.Selection{Path: path, Nodes: nodes}
	return nil
}

// PNG is a valid 1x1 image, returned by Render.
var PNG = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\rIDATx\x9cc\xf8\xff\xff?\x00\x05\xfe\x02\xfe\xa7\x35\x81\x84\x00\x00\x00\x00IEND\xaeB`\x82")

func (f *Fake) Render(path string, _ float64) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.docs[path] == nil {
		return nil, errors.New("not open")
	}
	return PNG, nil
}
