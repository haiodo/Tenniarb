// Package model is a port of the element model and its persistence from web/packages/core
// (element-model.ts, persistence.ts, element-properties.ts), which in turn port ElementModel.swift,
// ElementPersistence.swift and ElementProperties.swift. Identity (Swift Hashable by id) is pointer
// identity here; ids are kept for parity.
package model

import (
	"crypto/rand"
	"fmt"
	"slices"

	"github.com/haiodo/tenniarb/go/tenn"
)

type ElementKind string

const (
	KindRoot    ElementKind = "Root"
	KindElement ElementKind = "Element"
)

// ItemKindAnnontation keeps the Swift spelling.
type ItemKind string

const (
	ItemKindItem        ItemKind = "Item"
	ItemKindLink        ItemKind = "Link"
	ItemKindAnnontation ItemKind = "Annontation"
)

func ElementCommandName(kind ElementKind) string {
	if kind == KindRoot {
		return "model"
	}
	return "element"
}

func ItemCommandName(kind ItemKind) string {
	switch kind {
	case ItemKindLink:
		return "link"
	case ItemKindAnnontation:
		return "annotation"
	}
	return "item"
}

// ModelProperties holds the nodes the model does not understand, as a BlockExpr so that
// Get finds them by command name (the last one wins, as in the parser's lookup table).
type ModelProperties struct {
	node *tenn.Node
}

func NewModelProperties(props ...*tenn.Node) *ModelProperties {
	p := &ModelProperties{node: tenn.NewBlockExpr()}
	p.node.Add(props...)
	return p
}

func (p *ModelProperties) Nodes() []*tenn.Node {
	return p.node.Children
}

func (p *ModelProperties) Append(itm *tenn.Node) {
	p.node.Add(itm)
}

func (p *ModelProperties) AppendContentsOf(nodes []*tenn.Node) {
	p.node.Add(nodes...)
}

func (p *ModelProperties) Get(name string) *tenn.Node {
	return p.node.GetNamedElement(name)
}

func (p *ModelProperties) Count() int {
	return p.node.Count()
}

func (p *ModelProperties) Clone() *ModelProperties {
	return &ModelProperties{node: p.node.Clone()}
}

func (p *ModelProperties) AsNode() *tenn.Node {
	return p.node
}

func cloneNodes(nodes []*tenn.Node) []*tenn.Node {
	result := make([]*tenn.Node, 0, len(nodes))
	for _, n := range nodes {
		result = append(result, n.Clone())
	}
	return result
}

// newID makes a random version 4 UUID.
func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// Element Description is nil when absent, which differs from an empty string.
type Element struct {
	ID          string
	Kind        ElementKind
	Name        string
	Elements    []*Element
	Parent      *Element
	Model       *ElementModel
	Items       []*DiagramItem
	Description *string
	Properties  *ModelProperties // Extra nodes not supported directly by model.

	// FromFile is the path of "element ... --from-file": the content below is read from that file by
	// MountFiles, is never printed and cannot be edited here. mountSig is the state it was read in.
	FromFile string
	mountSig uint64

	// Transient data values
	Ox float64
	Oy float64
}

func NewElement(name string) *Element {
	return &Element{ID: newID(), Kind: KindElement, Name: name, Properties: NewModelProperties()}
}

func (e *Element) Count() int {
	return len(e.Elements)
}

func (e *Element) ItemCount() int {
	return len(e.Items)
}

// Clone does a full clone of this element with all children.
func (e *Element) Clone(cloneItems, cloneElement bool) *Element {
	cloneEl := NewElement(e.Name)

	cloneEl.Ox = e.Ox
	cloneEl.Oy = e.Oy
	cloneEl.Description = e.Description
	cloneEl.FromFile = e.FromFile

	cloneEl.Properties.AppendContentsOf(cloneNodes(e.Properties.Nodes()))

	if cloneItems {
		cloneEl.cloneItemsFrom(e)
	}

	if cloneElement {
		for _, el := range e.Elements {
			cp := el.Clone(true, true)
			cp.Parent = cloneEl
			cloneEl.Elements = append(cloneEl.Elements, cp)
		}
	}

	return cloneEl
}

func (e *Element) cloneItemsFrom(src *Element) {
	itemsMap := map[*DiagramItem]*DiagramItem{}
	var links []*DiagramItem

	for _, itm := range src.Items {
		cp := itm.Clone()
		cp.Parent = e
		itemsMap[itm] = cp
		if cp.isLink {
			links = append(links, cp)
		}
		e.Items = append(e.Items, cp)
	}

	// We need to process links with right items
	for _, link := range links {
		if link.Source != nil {
			link.Source = itemsMap[link.Source]
		}
		if link.Target != nil {
			link.Target = itemsMap[link.Target]
		}
	}
}

// assignModel hands the element's model to el and its descendants. A root has Model set to itself
// (see NewElementModel), so its own Add reaches ElementModel.assignModel without virtual dispatch.
func (e *Element) assignModel(el *Element) {
	if e.Model != nil {
		e.Model.assignModel(el)
	}
}

func (e *Element) assignItem(item *DiagramItem) {
	item.Parent = e
}

// AddItem appends a diagram item to the current diagram.
func (e *Element) AddItem(item *DiagramItem) {
	e.assignItem(item)
	e.Items = append(e.Items, item)
}

// InsertItem fails before touching state: Swift Array.insert traps on a bad index.
func (e *Element) InsertItem(at int, item *DiagramItem) error {
	if at < 0 || at > len(e.Items) {
		return fmt.Errorf("add: index %d out of range", at)
	}
	e.assignItem(item)
	e.Items = slices.Insert(e.Items, at, item)
	return nil
}

func (e *Element) AddElement(item *Element) {
	item.Parent = e
	e.assignModel(item)
	e.Elements = append(e.Elements, item)
}

// InsertElement appends when at is not below the current count, as the Swift original does.
func (e *Element) InsertElement(at int, item *Element) error {
	if at < 0 {
		return fmt.Errorf("add: index %d out of range", at)
	}
	item.Parent = e
	e.assignModel(item)
	if len(e.Elements) > at {
		e.Elements = slices.Insert(e.Elements, at, item)
	} else {
		e.Elements = append(e.Elements, item)
	}
	return nil
}

// AddGet adds a diagram item to the current diagram.
func (e *Element) AddGet(item *DiagramItem) *DiagramItem {
	e.AddItem(item)
	return item
}

// AddMakeItem adds a child element to the current diagram together with an item for it.
func (e *Element) AddMakeItem(el *Element) *DiagramItem {
	e.AddElement(el)
	item := NewDiagramItem(ItemKindItem, el.Name)
	e.Items = append(e.Items, item)
	e.assignItem(item)

	return item
}

// AddSourceTarget adds a link between two items; items missing from the diagram are added after the link.
func (e *Element) AddSourceTarget(source, target *DiagramItem) {
	link := NewLinkItem(ItemKindLink, "", source, target)
	e.Items = append(e.Items, link)
	e.assignItem(link)

	if !slices.Contains(e.Items, source) {
		e.Items = append(e.Items, source)
		e.assignItem(source)
	}
	if !slices.Contains(e.Items, target) {
		e.Items = append(e.Items, target)
		e.assignItem(target)
	}
}

// RemoveItem returns the removed index, -1 when not found.
func (e *Element) RemoveItem(item *DiagramItem) int {
	index := slices.Index(e.Items, item)
	if index != -1 {
		e.Items = slices.Delete(e.Items, index, index+1)
	}
	return index
}

// RemoveElement returns the removed index, -1 when not found.
func (e *Element) RemoveElement(item *Element) int {
	index := slices.Index(e.Elements, item)
	if index != -1 {
		e.Elements = slices.Delete(e.Elements, index, index+1)
	}
	return index
}

func (e *Element) FindLinks(item *DiagramItem) []*DiagramItem {
	var result []*DiagramItem
	for _, itm := range e.Items {
		if itm.touches(item, true, true) {
			result = append(result, itm)
		}
	}
	return result
}

func (e *Element) GetRelatedItems(item *DiagramItem, source, target bool) []*DiagramItem {
	var result []*DiagramItem
	for _, it := range e.Items {
		if it.touches(item, source, target) || it.ID == item.ID {
			result = append(result, it)
		}
	}
	return result
}

// ElementModel is the root of the element map.
type ElementModel struct {
	Element
	ModelName string
}

func NewElementModel() *ElementModel {
	m := &ElementModel{Element: *NewElement("Root")}
	m.Kind = KindRoot
	// TS has root.model === null; here the root points at itself so that the embedded Add dispatches to assignModel below.
	m.Element.Model = m
	return m
}

func (m *ElementModel) assignModel(el *Element) {
	el.Model = m

	// Assign all childs a proper model
	for _, child := range el.Elements {
		m.assignModel(child)
	}
}

// DiagramItem is an item, or a link when made by NewLinkItem (TS LinkItem extends DiagramItem).
type DiagramItem struct {
	ID          string
	Kind        ItemKind
	Name        string
	Parent      *Element
	Description *string
	Properties  *ModelProperties // Extra nodes not supported directly by model.
	X           float64
	Y           float64

	Source *DiagramItem // A direct link to source, links only
	Target *DiagramItem // A direct link to target, links only
	isLink bool
}

func NewDiagramItem(kind ItemKind, name string) *DiagramItem {
	return &DiagramItem{ID: newID(), Kind: kind, Name: name, Properties: NewModelProperties()}
}

func NewLinkItem(kind ItemKind, name string, source, target *DiagramItem) *DiagramItem {
	item := NewDiagramItem(kind, name)
	item.Source = source
	item.Target = target
	item.isLink = true
	return item
}

// IsLink reports whether the item was made by NewLinkItem; Kind alone is not enough.
func (d *DiagramItem) IsLink() bool {
	return d.isLink
}

// Clone of a link keeps its source and target; Element.Clone remaps them.
func (d *DiagramItem) Clone() *DiagramItem {
	cp := NewDiagramItem(d.Kind, d.Name)
	cp.Source, cp.Target, cp.isLink = d.Source, d.Target, d.isLink

	cp.X = d.X
	cp.Y = d.Y
	cp.Description = d.Description

	cp.Properties.AppendContentsOf(cloneNodes(d.Properties.Nodes()))

	return cp
}

func (d *DiagramItem) touches(item *DiagramItem, source, target bool) bool {
	if d.Kind != ItemKindLink || !d.isLink {
		return false
	}
	return (source && d.Source != nil && d.Source.ID == item.ID) ||
		(target && d.Target != nil && d.Target.ID == item.ID)
}

// ElementModelFactory is the default element factory.
type ElementModelFactory struct {
	ElementModel *ElementModel
}

func NewElementModelFactory() *ElementModelFactory {
	m := NewElementModel()
	m.AddElement(NewElement("Unnamed diagram"))
	return &ElementModelFactory{ElementModel: m}
}
