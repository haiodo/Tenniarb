package model

import (
	"github.com/haiodo/tenniarb/go/tenn"
)

// The TS toTennAsProps/fromTennProps take Element | DiagramItem; Go has one function per type.

// ElementToTennAsProps converts an element to a list of properties.
func ElementToTennAsProps(self *Element, kind tenn.NodeKind, reparse bool) *tenn.Node {
	items := tenn.NewNode(kind, nil)
	items.Add(tenn.NewCommand("name", tenn.NewStrNode(self.Name)))
	BuildElementData(self, items)
	return maybeReparse(items, reparse)
}

// ItemToTennAsProps converts an item or a link to a list of properties.
func ItemToTennAsProps(self *DiagramItem, kind tenn.NodeKind, reparse bool) *tenn.Node {
	items := tenn.NewNode(kind, nil)

	switch self.Kind {
	case ItemKindItem:
		if self.Name != "" {
			items.Add(tenn.NewCommand(cmd(PersistName), tenn.NewStrNode(self.Name)))
		}
		BuildItemData(self, items, true)
	case ItemKindLink:
		BuildLinkData(self, items, true)
	}
	return maybeReparse(items, reparse)
}

// maybeReparse converts the nodes to text and back to get proper positions.
func maybeReparse(items *tenn.Node, reparse bool) *tenn.Node {
	if !reparse {
		return items
	}
	tree, _ := tenn.Parse(tenn.ToStr(items, 0, false))
	return tree
}

func ElementFromTennProps(self *Element, node *tenn.Node) {
	self.Properties = NewModelProperties()

	var linkElements []LinkNode
	TraverseBlock(node, func(cmdName string, blChild *tenn.Node) {
		if cmdName == "name" {
			if newName, ok := blChild.GetIdent(1); ok {
				self.Name = newName
			}
			return
		}
		ParseElementData(self, cmdName, blChild, &linkElements)
	})
}

func ItemFromTennProps(self *DiagramItem, node *tenn.Node) {
	switch self.Kind {
	case ItemKindItem:
		resetItem(self)
		TraverseBlock(node, func(cmdName string, blChild *tenn.Node) {
			if cmdName == cmd(PersistName) {
				if newName, ok := blChild.GetIdent(1); ok {
					self.Name = newName
				}
				return
			}
			ParseItemData(self, cmdName, blChild)
		})
	case ItemKindLink:
		resetItem(self)
		self.Name = ""
		idx := &LinkIndexes{}
		TraverseBlock(node, func(cmdName string, blChild *tenn.Node) {
			if cmdName == cmd(PersistLabel) {
				if newName, ok := blChild.GetIdent(1); ok {
					self.Name = newName
				}
				return
			}
			ParseLinkData(self, cmdName, blChild, idx)
		})
	}
}

// resetItem clears position in case pos was deleted.
func resetItem(self *DiagramItem) {
	self.Properties = NewModelProperties()
	self.X = 0
	self.Y = 0
}
