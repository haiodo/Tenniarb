package model

import "math"

// GridLayout is a port of web/packages/core/src/grid-layout.ts.
type GridLayout struct {
	PaddingPercentage float64
	MinEntitySize     float64

	// AspectRatio is the width/height ratio.
	AspectRatio float64
	// RowPadding is the padding around rows.
	RowPadding float64

	Rows        int
	Cols        int
	NumChildren int
	ColWidth    float64
	RowHeight   float64
	OffsetX     float64
	OffsetY     float64
	// ChildrenHeight is the height of a single node.
	ChildrenHeight float64
	// ChildrenWidth is the width of a single node.
	ChildrenWidth float64
}

func NewGridLayout() *GridLayout {
	return &GridLayout{PaddingPercentage: 0.95, MinEntitySize: 5, AspectRatio: 1.0}
}

func (g *GridLayout) Apply(context *LayoutContext, clean bool) ([]ElementOperation, error) {
	if !clean {
		return nil, nil
	}
	g.NumChildren = len(context.Nodes)
	bounds := context.GetBounds()
	if err := g.CalculateGrid(bounds); err != nil {
		return nil, err
	}

	var operations []ElementOperation

	index := 0
	for i := 0; i < g.Rows; i++ {
		for j := 0; j < g.Cols; j++ {
			if i*g.Cols+j < g.NumChildren {
				node := context.Nodes[index]
				index++
				size := context.GetNodeBounds(node)
				// float64() around a product stops the compiler fusing it into an FMA, which would break bit parity with Swift/TS.
				xmove := bounds.X + float64(float64(j)*g.ColWidth) + g.OffsetX
				ymove := bounds.Y + bounds.Height - float64(float64(i)*g.RowHeight) + g.OffsetY - size.Height
				if context.IsMovable(node) {
					operations = append(operations, context.Store.CreateUpdatePosition(node, Point{xmove, ymove}))
				}
			}
		}
	}
	return operations, nil
}

func (g *GridLayout) CalculateGrid(bounds Rect) error {
	cols, rows, err := g.CalculateNumberOfRowsAndCols(g.NumChildren, bounds.X, bounds.Y, bounds.Width, bounds.Height)
	if err != nil {
		return err
	}
	g.Cols, g.Rows = cols, rows

	g.ColWidth = bounds.Width / float64(g.Cols)
	g.RowHeight = bounds.Height / float64(g.Rows)

	g.ChildrenWidth, g.ChildrenHeight = g.CalculateNodeSize(g.ColWidth, g.RowHeight)
	g.OffsetX = (g.ColWidth - g.ChildrenWidth) / 2.0   // half of the space between columns
	g.OffsetY = (g.RowHeight - g.ChildrenHeight) / 2.0 // half of the space between rows
	return nil
}

// CalculateNumberOfRowsAndCols returns cols, rows.
func (g *GridLayout) CalculateNumberOfRowsAndCols(numChildren int, boundX, boundY, boundWidth, boundHeight float64) (int, int, error) {
	// Both loops below only shrink or grow towards numChildren, so zero never terminates them.
	if numChildren <= 0 {
		return 1, 1, nil
	}
	if g.AspectRatio == 1.0 {
		return g.calculateNumberOfRowsAndColsSquare(numChildren, boundX, boundY, boundWidth, boundHeight)
	}
	return g.calculateNumberOfRowsAndColsRectangular(numChildren)
}

func (g *GridLayout) calculateNumberOfRowsAndColsRectangular(numChildren int) (int, int, error) {
	rows := swiftMax(1, math.Ceil(math.Sqrt(float64(numChildren))))
	cols := swiftMax(1, math.Ceil(math.Sqrt(float64(numChildren))))
	r, err := toInt(rows)
	if err != nil {
		return 0, 0, err
	}
	c, err := toInt(cols)
	if err != nil {
		return 0, 0, err
	}
	return c, r, nil
}

func (g *GridLayout) calculateNumberOfRowsAndColsSquare(numChildren int, _, _, boundWidth, boundHeight float64) (int, int, error) {
	n := float64(numChildren)
	rows, err := toInt(swiftMax(1, math.Sqrt(n*boundHeight/boundWidth)))
	if err != nil {
		return 0, 0, err
	}
	cols, err := toInt(swiftMax(1, math.Sqrt(n*boundWidth/boundHeight)))
	if err != nil {
		return 0, 0, err
	}

	// if space is taller than wide, adjust rows first
	if boundWidth <= boundHeight {
		// decrease number of rows and columns until just enough or not enough
		for rows*cols > numChildren {
			if rows > 1 {
				rows--
			}
			if rows*cols > numChildren {
				if cols > 1 {
					cols++
				}
			}
		}
		// increase number of rows and columns until just enough
		for rows*cols < numChildren {
			rows++
			if rows*cols < numChildren {
				cols++
			}
		}
	} else {
		for rows*cols > numChildren {
			if cols > 1 {
				cols--
			}
			if rows*cols > numChildren {
				if rows > 1 {
					rows--
				}
			}
		}
		for rows*cols < numChildren {
			cols++
			if rows*cols < numChildren {
				rows++
			}
		}
	}
	return cols, rows, nil
}

// CalculateNodeSize returns the width and height of a node.
func (g *GridLayout) CalculateNodeSize(colWidth, rowHeight float64) (float64, float64) {
	childW := swiftMax(g.MinEntitySize, g.PaddingPercentage*colWidth)
	childH := swiftMax(g.MinEntitySize, g.PaddingPercentage*(rowHeight-g.RowPadding))
	whRatio := colWidth / rowHeight
	if whRatio < g.AspectRatio {
		childH = childW / g.AspectRatio
	} else {
		childW = childH * g.AspectRatio
	}
	return childW, childH
}
