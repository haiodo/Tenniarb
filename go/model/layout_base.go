package model

import (
	"fmt"
	"math"
)

// Port of web/packages/core/src/layout-base.ts (LayoutBase.swift).

type Rect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// LayoutScene is what the layouts need from the scene (Swift: DrawableScene); the renderer implements it.
type LayoutScene interface {
	// GetBounds is the bounds of the whole scene.
	GetBounds() Rect
	// GetItemBounds is the bounds of the item's drawable, false when the item has none (treated as an empty rect).
	GetItemBounds(node *DiagramItem) (Rect, bool)
}

type LayoutAlgorithm interface {
	Apply(context *LayoutContext, clean bool) ([]ElementOperation, error)
}

// Swift's generic max/min are `y >= x ? y : x` and `y < x ? y : x`, which differ from the builtins on NaN.
func swiftMax(x, y float64) float64 {
	if y >= x {
		return y
	}
	return x
}

func swiftMin(x, y float64) float64 {
	if y < x {
		return y
	}
	return x
}

// toInt: Swift's Int(Double) traps on NaN/infinity; an infinite loop in the callers would be worse.
func toInt(x float64) (int, error) {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0, fmt.Errorf("not representable as Int: %v", x)
	}
	return int(math.Trunc(x)), nil
}

type LayoutContext struct {
	// Element is a container to perform layout on.
	Element *Element

	PreLayoutPass  []func()
	PostLayoutPass []func()

	Layout LayoutAlgorithm
	Scene  LayoutScene

	Nodes []*DiagramItem
	Edges []*DiagramItem // links

	Store  *ElementModelStore
	Bounds Rect
}

func NewLayoutContext(element *Element, scene LayoutScene, store *ElementModelStore, bounds Rect) *LayoutContext {
	c := &LayoutContext{Element: element, Scene: scene, Store: store, Bounds: bounds}
	c.Nodes = c.FindNodes()
	c.Edges = c.FindEdges()
	return c
}

func (c *LayoutContext) PreLayout() {
	for _, pre := range c.PreLayoutPass {
		pre()
	}
}

func (c *LayoutContext) PostLayout() {
	for _, post := range c.PostLayoutPass {
		post()
	}
}

// GetBounds is the bounds of the whole scene.
func (c *LayoutContext) GetBounds() Rect {
	return c.Scene.GetBounds()
}

// GetNodeBounds is the bounds of the node, empty when the scene has none.
func (c *LayoutContext) GetNodeBounds(node *DiagramItem) Rect {
	r, _ := c.Scene.GetItemBounds(node)
	return r
}

func (c *LayoutContext) GetViewBounds() Rect {
	return c.Bounds
}

func (c *LayoutContext) Apply(clean bool) ([]ElementOperation, error) {
	if c.Layout == nil {
		return nil, nil
	}
	c.PreLayout()
	ops, err := c.Layout.Apply(c, clean)
	c.PostLayout()
	return ops, err
}

func (c *LayoutContext) FindNodes() []*DiagramItem {
	var nodes []*DiagramItem
	for _, itm := range c.Element.Items {
		if itm.Kind == ItemKindItem {
			nodes = append(nodes, itm)
		}
	}
	return nodes
}

// FindEdges: TS casts every Kind "Link" item to LinkItem, so Source and Target may be nil.
func (c *LayoutContext) FindEdges() []*DiagramItem {
	var edges []*DiagramItem
	for _, itm := range c.Element.Items {
		if itm.Kind == ItemKindLink {
			edges = append(edges, itm)
		}
	}
	return edges
}

func (c *LayoutContext) IsMovable(_ *DiagramItem) bool {
	// For now any item are movable.
	return true
}

func (c *LayoutContext) GetWeight(_ *DiagramItem) float64 {
	return 0
}
