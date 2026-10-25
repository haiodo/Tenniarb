package model

import (
	"fmt"
	"slices"
	"strings"

	"github.com/haiodo/tenniarb/go/tenn"
	"golang.org/x/text/unicode/norm"
)

type PersistenceItemKind string

const (
	PersistItem        PersistenceItemKind = "Item"
	PersistLink        PersistenceItemKind = "Link"
	PersistElement     PersistenceItemKind = "Element"
	PersistModel       PersistenceItemKind = "Model"
	PersistAnnontation PersistenceItemKind = "Annontation"
	PersistDescription PersistenceItemKind = "Description"
	PersistLabel       PersistenceItemKind = "Label"
	PersistSourceIndex PersistenceItemKind = "SourceIndex"
	PersistTargetIndex PersistenceItemKind = "TargetIndex"
	PersistName        PersistenceItemKind = "Name"
	PersistPosition    PersistenceItemKind = "Position"
)

var itemCommandNames = map[PersistenceItemKind]string{
	PersistItem:        "item",
	PersistLink:        "link",
	PersistElement:     "element",
	PersistModel:       "model",
	PersistAnnontation: "annotation",
	PersistDescription: "description",
	PersistSourceIndex: "source-index",
	PersistTargetIndex: "target-index",
	PersistPosition:    "pos",
	PersistName:        "name",
	PersistLabel:       "label",
}

func PersistenceItemCommandName(kind PersistenceItemKind) string {
	return itemCommandNames[kind]
}

type PersistenceStyleKind string

const (
	StyleColor        PersistenceStyleKind = "Color"
	StyleTextColor    PersistenceStyleKind = "TextColor"
	StyleFontSize     PersistenceStyleKind = "FontSize"
	StyleDisplay      PersistenceStyleKind = "Display"
	StyleLayout       PersistenceStyleKind = "Layout"
	StyleLineStyle    PersistenceStyleKind = "LineStyle"
	StyleWidth        PersistenceStyleKind = "Width"
	StyleHeight       PersistenceStyleKind = "Height"
	StyleBorderColor  PersistenceStyleKind = "BorderColor"
	StyleZoomLevel    PersistenceStyleKind = "ZoomLevel"
	StyleStyles       PersistenceStyleKind = "Styles"
	StyleGrid         PersistenceStyleKind = "Grid"
	StyleTitle        PersistenceStyleKind = "Title"
	StyleLabel        PersistenceStyleKind = "Label"
	StyleShadow       PersistenceStyleKind = "Shadow"
	StyleLineWidth    PersistenceStyleKind = "LineWidth"
	StyleMarker       PersistenceStyleKind = "Marker"
	StyleLayer        PersistenceStyleKind = "Layer"
	StyleInherit      PersistenceStyleKind = "Inherit"
	StyleUseStyle     PersistenceStyleKind = "UseStyle"
	StyleFieldName    PersistenceStyleKind = "FieldName"
	StyleCornerRadius PersistenceStyleKind = "CornerRadius"
	StyleLineSpacing  PersistenceStyleKind = "LineSpacing"
)

var styleNames = map[PersistenceStyleKind]string{
	StyleColor:        "color",
	StyleFontSize:     "font-size",
	StyleDisplay:      "display",
	StyleLayout:       "layout",
	StyleLineStyle:    "line-style",
	StyleWidth:        "width",
	StyleHeight:       "height",
	StyleBorderColor:  "border-color",
	StyleZoomLevel:    "zoom",
	StyleTextColor:    "text-color",
	StyleStyles:       "styles",
	StyleGrid:         "grid",
	StyleTitle:        "title",
	StyleLabel:        "label",
	StyleShadow:       "shadow",
	StyleLineWidth:    "line-width",
	StyleMarker:       "marker",
	StyleLayer:        "layer",
	StyleInherit:      "inherit",
	StyleUseStyle:     "use-style",
	StyleFieldName:    "field-name",
	StyleCornerRadius: "corner-radius",
	StyleLineSpacing:  "line-spacing",
}

func PersistenceStyleName(kind PersistenceStyleKind) string {
	return styleNames[kind]
}

var cmd = PersistenceItemCommandName

// Swift String equality is canonical equivalence, so names are NFC-normalised for map keys.
func nfc(s string) string {
	return norm.NFC.String(s)
}

type IndexedName struct {
	Name  string
	Index int
}

func (n IndexedName) Key() string {
	return fmt.Sprintf("%s\x00%d", nfc(n.Name), n.Index)
}

// ToTenn maps the element model to tenn nodes.
func ToTenn(self *Element, includeSubElements, includeItems bool) *tenn.Node {
	result := tenn.NewNode(tenn.KindStatements, nil)

	if self.Kind == KindRoot {
		// Top-level non-element statements go first; their original interleaving with elements is not kept.
		for _, p := range self.Properties.Nodes() {
			result.Add(p.Clone())
		}
		buildElements(result, self.Elements, includeSubElements, includeItems)
	} else {
		buildElements(result, []*Element{self}, includeSubElements, includeItems)
	}

	return result
}

func ToTennStr(self *Element, includeSubElements, includeItems bool) string {
	return tenn.ToStr(ToTenn(self, includeSubElements, includeItems), 0, false)
}

func addPosition(block *tenn.Node, item *DiagramItem, addPos bool) {
	if item.X != 0 || item.Y != 0 || addPos {
		block.Add(tenn.NewCommand(cmd(PersistPosition), tenn.NewFloatNode(item.X), tenn.NewFloatNode(item.Y)))
	}
}

func addDescription(block *tenn.Node, description *string) {
	if description != nil {
		block.Add(tenn.NewCommand(cmd(PersistDescription), tenn.NewStrNode(*description)))
	}
}

func BuildItemData(item *DiagramItem, itemBlock *tenn.Node, addPos bool) {
	addDescription(itemBlock, item.Description)
	addPosition(itemBlock, item, addPos)

	for _, p := range item.Properties.Nodes() {
		itemBlock.Add(p.Clone())
	}
}

func buildItem(item *DiagramItem, enodeBlock *tenn.Node) {
	itemRoot := tenn.NewCommand(cmd(PersistItem), tenn.NewStrNode(item.Name))

	enodeBlock.Add(itemRoot)

	itemBlock := tenn.NewBlockExpr()

	BuildItemData(item, itemBlock, false)

	if itemBlock.Count() > 0 {
		itemRoot.Add(itemBlock)
	}
}

func BuildLinkData(item *DiagramItem, linkDataBlock *tenn.Node, addPos bool) {
	addDescription(linkDataBlock, item.Description)

	if item.Name != "" {
		linkDataBlock.Add(tenn.NewCommand(cmd(PersistLabel), tenn.NewStrNode(item.Name)))
	}

	addPosition(linkDataBlock, item, addPos)

	for _, p := range item.Properties.Nodes() {
		linkDataBlock.Add(p.Clone())
	}
}

func buildLink(item *DiagramItem, enodeBlock *tenn.Node, indexes map[*DiagramItem]int) {
	if !item.isLink {
		return
	}
	sourceName, targetName := "", ""
	if item.Source != nil {
		sourceName = item.Source.Name
	}
	if item.Target != nil {
		targetName = item.Target.Name
	}
	linkCmd := tenn.NewCommand(cmd(PersistLink))
	linkCmd.Add(tenn.NewStrNode(sourceName))
	linkCmd.Add(tenn.NewStrNode(targetName))

	linkDataBlock := tenn.NewBlockExpr()

	if idx, ok := indexes[item.Source]; ok && idx != 0 {
		linkDataBlock.Add(tenn.NewCommand(cmd(PersistSourceIndex), tenn.NewIntNode(idx)))
	}
	if idx, ok := indexes[item.Target]; ok && idx != 0 {
		linkDataBlock.Add(tenn.NewCommand(cmd(PersistTargetIndex), tenn.NewIntNode(idx)))
	}

	BuildLinkData(item, linkDataBlock, false)

	if linkDataBlock.Count() > 0 {
		linkCmd.Add(linkDataBlock)
	}

	enodeBlock.Add(linkCmd)
}

func buildItems(items []*DiagramItem, enodeBlock *tenn.Node, itemIndexes map[*DiagramItem]int) {
	for _, item := range items {
		switch item.Kind {
		case ItemKindItem:
			buildItem(item, enodeBlock)
		case ItemKindLink:
			buildLink(item, enodeBlock, itemIndexes)
		}
	}
}

// PrepareItemRefs numbers items of the same name: the first gets 0, the next 1, and so on.
func PrepareItemRefs(items []*DiagramItem) map[*DiagramItem]int {
	itemRefNames := map[*DiagramItem]int{}
	strToIndex := map[string]int{}

	for _, item := range items {
		if item.Kind != ItemKindItem {
			continue
		}
		key := nfc(item.Name)
		index, seen := strToIndex[key]
		if !seen {
			index = -1
		}
		strToIndex[key] = index + 1
		itemRefNames[item] = index + 1
	}

	return itemRefNames
}

func BuildElementData(e *Element, enodeBlock *tenn.Node) {
	if e.Description != nil && *e.Description != "" {
		enodeBlock.Add(tenn.NewCommand(cmd(PersistDescription), tenn.NewStrNode(*e.Description)))
	}

	for _, p := range e.Properties.Nodes() {
		enodeBlock.Add(p.Clone())
	}
}

func buildElement(e *Element, topParent *tenn.Node, includeSubElements, includeItems bool) {
	enode := tenn.NewCommand(cmd(PersistElement), tenn.NewStrNode(e.Name))

	topParent.Add(enode)

	enodeBlock := tenn.NewBlockExpr()

	enode.Add(enodeBlock)

	BuildElementData(e, enodeBlock)

	if includeItems {
		buildItems(e.Items, enodeBlock, PrepareItemRefs(e.Items))
	}

	if len(e.Elements) > 0 && includeSubElements {
		buildElements(enodeBlock, e.Elements, includeSubElements, includeItems)
	}
}

func buildElements(topParent *tenn.Node, elements []*Element, includeSubElements, includeItems bool) {
	for _, e := range elements {
		buildElement(e, topParent, includeSubElements, includeItems)
	}
}

func StoreItems(items []*DiagramItem) *tenn.Node {
	block := tenn.NewNode(tenn.KindStatements, nil)

	buildItems(items, block, PrepareItemRefs(items))
	return block
}

// ParseTenn parses a tenn tree into an element model. Nodes it does not understand are kept in Properties.
func ParseTenn(node *tenn.Node) *ElementModel {
	result := NewElementModel()
	switch node.Kind {
	case tenn.KindStatements:
		for _, child := range node.Children {
			if el := parseCommand(child); el != nil {
				result.AddElement(el)
			} else {
				result.Properties.Append(child)
			}
		}
	case tenn.KindCommand:
		if el := parseCommand(node); el != nil {
			result.AddElement(el)
		}
	default:
		result.Properties.Append(node)
	}
	return result
}

// TraverseBlock calls visitor for each command of a Statements or BlockExpr node that starts with a name.
func TraverseBlock(block *tenn.Node, visitor func(cmdName string, node *tenn.Node)) {
	if block.Kind != tenn.KindStatements && block.Kind != tenn.KindBlockExpr {
		return
	}
	for _, blChild := range block.Children {
		if blChild.Kind != tenn.KindCommand || blChild.Count() == 0 {
			continue
		}
		if cmdName, ok := blChild.GetIdent(0); ok {
			visitor(cmdName, blChild)
		}
	}
}

func parseChildCommands(node *tenn.Node, blockIndex int, visitor func(cmdName string, child *tenn.Node)) {
	if node.Count() > blockIndex {
		if block := node.GetChild(blockIndex); block != nil {
			TraverseBlock(block, visitor)
		}
	}
}

func prepareRefs(items []*DiagramItem) map[string]*DiagramItem {
	itemRefNames := map[string]*DiagramItem{}

	for item, index := range PrepareItemRefs(items) {
		itemRefNames[IndexedName{item.Name, index}.Key()] = item
	}

	return itemRefNames
}

type LinkNode struct {
	Node *tenn.Node
	Link *DiagramItem
}

// ParseElementData unknown commands, including any not listed here, are kept verbatim in Properties.
func ParseElementData(el *Element, cmdName string, blChild *tenn.Node, linkElements *[]LinkNode) {
	switch cmdName {
	case cmd(PersistItem):
		if item := parseItem(blChild); item != nil {
			el.AddItem(item)
			return
		}
	case cmd(PersistLink):
		if item := parseLink(blChild); item != nil {
			el.AddItem(item)
			*linkElements = append(*linkElements, LinkNode{blChild, item})
			return
		}
	case cmd(PersistElement):
		if child := parseElement(blChild); child != nil {
			el.AddElement(child)
			return
		}
	}
	el.Properties.Append(blChild)
}

func parseElement(node *tenn.Node) *Element {
	// TS: need to report error
	if node.Count() < 2 {
		return nil
	}
	el := NewElement("")

	var linkElements []LinkNode

	if name, ok := node.GetIdent(1); ok {
		el.Name = name
	}

	parseChildCommands(node, 2, func(cmdName string, blChild *tenn.Node) {
		ParseElementData(el, cmdName, blChild, &linkElements)
	})

	refs := prepareRefs(el.Items)

	for _, ln := range linkElements {
		processLink(ln.Link, ln.Node, refs)
		// Keep unresolved links verbatim so a save does not lose their name/index references.
		if ln.Link.Source == nil || ln.Link.Target == nil {
			el.Items = slices.DeleteFunc(el.Items, func(i *DiagramItem) bool { return i == ln.Link })
			el.Properties.Append(ln.Node)
		}
	}

	return el
}

func ParseItems(node *tenn.Node) []*DiagramItem {
	el := NewElement("")
	var linkElements []LinkNode

	TraverseBlock(node, func(cmdName string, blChild *tenn.Node) {
		ParseElementData(el, cmdName, blChild, &linkElements)
	})

	refs := prepareRefs(el.Items)

	for _, ln := range linkElements {
		processLink(ln.Link, ln.Node, refs)
	}

	return el.Items
}

func identPtr(node *tenn.Node, index int) *string {
	if s, ok := node.GetIdent(index); ok {
		return &s
	}
	return nil
}

// parsePosition keeps a pos command that is not two numbers in Properties.
func parsePosition(item *DiagramItem, blChild *tenn.Node) {
	if blChild.Count() == 3 {
		x, okX := blChild.GetFloat(1)
		y, okY := blChild.GetFloat(2)
		if okX && okY {
			item.X, item.Y = x, y
			return
		}
	}
	item.Properties.Append(blChild)
}

func ParseItemData(el *DiagramItem, cmdName string, blChild *tenn.Node) {
	switch cmdName {
	case cmd(PersistPosition):
		parsePosition(el, blChild)
	case cmd(PersistDescription), "desription":
		el.Description = identPtr(blChild, 1)
	default:
		el.Properties.Append(blChild)
	}
}

func parseItem(node *tenn.Node) *DiagramItem {
	el := NewDiagramItem(ItemKindItem, "")

	if node.Count() >= 2 {
		if name, ok := node.GetIdent(1); ok {
			el.Name = name
		}

		parseChildCommands(node, 2, func(cmdName string, blChild *tenn.Node) {
			ParseItemData(el, cmdName, blChild)
		})
	}
	return el
}

func parseLink(node *tenn.Node) *DiagramItem {
	if node.Count() >= 2 {
		return NewLinkItem(ItemKindLink, "", nil, nil)
	}
	return nil
}

type LinkIndexes struct {
	Source int
	Target int
}

func parseIndex(link *DiagramItem, blChild *tenn.Node, dst *int) {
	if index, ok := blChild.GetInt(1); ok {
		*dst = index
		return
	}
	link.Properties.Append(blChild)
}

func ParseLinkData(link *DiagramItem, cmdName string, blChild *tenn.Node, idx *LinkIndexes) {
	switch cmdName {
	case cmd(PersistDescription), "desription":
		link.Description = identPtr(blChild, 1)
	case cmd(PersistLabel):
		link.Name, _ = blChild.GetIdent(1)
	case cmd(PersistPosition):
		parsePosition(link, blChild)
	case cmd(PersistSourceIndex):
		parseIndex(link, blChild, &idx.Source)
	case cmd(PersistTargetIndex):
		parseIndex(link, blChild, &idx.Target)
	default:
		link.Properties.Append(blChild)
	}
}

func processLink(link *DiagramItem, node *tenn.Node, links map[string]*DiagramItem) {
	idx := &LinkIndexes{}
	parseChildCommands(node, 3, func(cmdName string, blChild *tenn.Node) {
		ParseLinkData(link, cmdName, blChild, idx)
	})

	source, okS := node.GetIdent(1)
	target, okT := node.GetIdent(2)
	if okS && okT {
		if s, ok := links[IndexedName{source, idx.Source}.Key()]; ok {
			link.Source = s
		}
		if t, ok := links[IndexedName{target, idx.Target}.Key()]; ok {
			link.Target = t
		}
	}
}

func parseCommand(node *tenn.Node) *Element {
	if node.Kind != tenn.KindCommand {
		return nil
	}
	cmdName, ok := node.GetIdent(0)
	if !ok {
		return nil
	}
	if cmdName == cmd(PersistElement) || cmdName == cmd(PersistModel) {
		return parseElement(node)
	}
	return nil
}

// ReadTenn mirrors Document.read: a leading BOM is dropped, and nil is returned when the parser reports errors.
func ReadTenn(text string) *ElementModel {
	tree, parser := tenn.Parse(strings.TrimPrefix(text, "\xef\xbb\xbf"))
	if parser.Errors.HasErrors() {
		return nil
	}
	return ParseTenn(tree)
}
