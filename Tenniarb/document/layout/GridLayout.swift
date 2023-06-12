//
//  GridLayout.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 12.12.2019.
//  Copyright © 2019 Andrey Sobolev. All rights reserved.
//

import Foundation

public class GridLayout: LayoutAlgorithm {
    var PADDING_PERCENTAGE: CGFloat = 0.95
    var MIN_ENTITY_SIZE: CGFloat = 5

    /**
     * The width/height ratio.
     */
    var aspectRatio: CGFloat = 1.0

    /**
     * The padding around rows.
     */
    var rowPadding: CGFloat = 0

    var resize = false
    /**
     * The number of rows.
     */
    var rows: Int = 0
    /**
     * The number of columns.
     */
    var cols: Int = 0
    /**
     * The number of nodes.
     */
    var numChildren: Int = 0
    /**
     * The column width.
     */
    var colWidth: CGFloat = 0;
    /**
     * The row height.
     */
    var rowHeight: CGFloat = 0;
    /**
     * The horizontal offset.
     */
    var offsetX: CGFloat = 0;
    /**
     * The vertical offset.
     */
    var offsetY: CGFloat = 0;
    /**
     * The height of a single node.
     */
    var childrenHeight: CGFloat = 0;
    /**
     * The width of a single node.
     */
    var childrenWidth: CGFloat = 0;

    func apply(context: LayoutContext, clean: Bool) -> [ElementOperation] {
        if !clean {
            return []
        }
        self.numChildren = context.nodes.count
        let bounds = context.getBounds()
        self.calculateGrid(bounds)
        
        var operations: [ElementOperation] = []

        var index = 0
        for i  in  0 ..< rows {
            for j in 0 ..< cols {
                if (i * cols + j) < numChildren {
                    let node = context.nodes[index]
                    index += 1
                    let isResizable = false // LayoutProperties.isResizable(node)
                    if resize && isResizable {
//                        LayoutProperties.setSize(node, new Dimension(
//                                Math.max(childrenWidth, MIN_ENTITY_SIZE),
//                                Math.max(childrenHeight, MIN_ENTITY_SIZE)));
                    }
                    let size = context.getBounds(node: node)
                    let xmove = bounds.origin.x + CGFloat(j) * colWidth + offsetX ;//- size.width / 2;
                    let ymove = bounds.origin.y + bounds.size.height - CGFloat(i) * rowHeight + offsetY - size.height;
                    if context.isMovable(node) {
                        operations.append(context.store.createUpdatePosition(item: node, newPos: CGPoint(x: xmove, y: ymove)))
                    }
                }
            }
        }
        return operations
    }
    func calculateGrid(_ bounds: CGRect ) {
        (cols, rows) = self.calculateNumberOfRowsAndCols(numChildren, bounds.origin.x, bounds.origin.y, bounds.width, bounds.height)

        colWidth = bounds.width / CGFloat(cols)
        rowHeight = bounds.height / CGFloat(rows)

        let nodeSize = self.calculateNodeSize(colWidth, rowHeight)
        childrenWidth = nodeSize.width
        childrenHeight = nodeSize.height
        offsetX = (colWidth - childrenWidth) / 2.0; // half of the space between
                                                    // columns
        offsetY = (rowHeight - childrenHeight) / 2.0; // half of the space
                                                        // between rows
    }
    func calculateNumberOfRowsAndCols(_ numChildren: Int, _ boundX: CGFloat, _ boundY: CGFloat, _ boundWidth: CGFloat, _ boundHeight: CGFloat) -> (Int, Int) {
        if (aspectRatio == 1.0) {
            return calculateNumberOfRowsAndCols_square(numChildren, boundX, boundY, boundWidth, boundHeight);
        } else {
            return calculateNumberOfRowsAndCols_rectangular(numChildren)
        }
    }
    func calculateNumberOfRowsAndCols_rectangular(_ numChildren: Int) -> (Int, Int) {
        let rows = max(1, ceil(sqrt(CGFloat(numChildren))))
        let cols = max(1, ceil(sqrt(CGFloat(numChildren))))
        return (Int(rows), Int(cols))
    }
    func calculateNumberOfRowsAndCols_square(_ numChildren: Int,
                                             _ boundX: CGFloat, _ boundY: CGFloat, _ boundWidth: CGFloat,
                                             _ boundHeight: CGFloat) -> (Int, Int) {
        var rows = Int(max(1, sqrt(CGFloat(numChildren) * boundHeight / boundWidth)))
        var cols = Int(max(1, sqrt(CGFloat(numChildren) * boundWidth / boundHeight)))

        // if space is taller than wide, adjust rows first
        if boundWidth <= boundHeight {
            // decrease number of rows and columns until just enough or not
            // enough
            while rows * cols > numChildren {
                if rows > 1 {
                    rows -= 1
                }
                if rows * cols > numChildren {
                    if cols > 1 {
                        cols += 1
                    }
                }
            }
            // increase number of rows and columns until just enough
            while rows * cols < numChildren {
                rows += 1
                if rows * cols < numChildren {
                    cols += 1
                }
            }
        } else {
            // decrease number of rows and columns until just enough or not
            // enough
            while rows * cols > numChildren {
                if cols > 1 {
                    cols -= 1
                }
                if rows * cols > numChildren {
                    if rows > 1 {
                        rows -= 1
                    }
                }
            }
            // increase number of rows and columns until just enough
            while rows * cols < numChildren {
                cols += 1
                if rows * cols < numChildren {
                    rows += 1
                }
            }
        }
        return (cols, rows)
    }
    func calculateNodeSize(_ colWidth: CGFloat, _ rowHeight: CGFloat) -> CGSize {
        var childW = max(MIN_ENTITY_SIZE, PADDING_PERCENTAGE * colWidth)
        var childH = max(MIN_ENTITY_SIZE, PADDING_PERCENTAGE * (rowHeight - rowPadding))
        let whRatio = colWidth / rowHeight;
        if whRatio < aspectRatio {
            childH = childW / aspectRatio;
        } else {
            childW = childH * aspectRatio;
        }
        return CGSize(width: childW, height: childH);
    }
}
