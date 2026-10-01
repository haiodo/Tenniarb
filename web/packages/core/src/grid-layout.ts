import type { ElementOperation } from "./element-operations.ts";
import { max, toInt } from "./layout-base.ts";
import type { LayoutAlgorithm, LayoutContext, Rect } from "./layout-base.ts";

export class GridLayout implements LayoutAlgorithm {
  PADDING_PERCENTAGE = 0.95;
  MIN_ENTITY_SIZE = 5;

  /** The width/height ratio. */
  aspectRatio = 1.0;
  /** The padding around rows. */
  rowPadding = 0;

  resize = false;
  rows = 0;
  cols = 0;
  numChildren = 0;
  colWidth = 0;
  rowHeight = 0;
  offsetX = 0;
  offsetY = 0;
  /** The height of a single node. */
  childrenHeight = 0;
  /** The width of a single node. */
  childrenWidth = 0;

  apply(context: LayoutContext, clean: boolean): ElementOperation[] {
    if (!clean) {
      return [];
    }
    this.numChildren = context.nodes.length;
    const bounds = context.getBounds();
    this.calculateGrid(bounds);

    const operations: ElementOperation[] = [];

    let index = 0;
    for (let i = 0; i < this.rows; i++) {
      for (let j = 0; j < this.cols; j++) {
        if (i * this.cols + j < this.numChildren) {
          const node = context.nodes[index++]!;
          const size = context.getBounds(node);
          const xmove = bounds.x + j * this.colWidth + this.offsetX;
          const ymove = bounds.y + bounds.height - i * this.rowHeight + this.offsetY - size.height;
          if (context.isMovable(node)) {
            operations.push(context.store.createUpdatePosition(node, { x: xmove, y: ymove }));
          }
        }
      }
    }
    return operations;
  }

  calculateGrid(bounds: Rect): void {
    [this.cols, this.rows] = this.calculateNumberOfRowsAndCols(this.numChildren, bounds.x, bounds.y, bounds.width, bounds.height);

    this.colWidth = bounds.width / this.cols;
    this.rowHeight = bounds.height / this.rows;

    const nodeSize = this.calculateNodeSize(this.colWidth, this.rowHeight);
    this.childrenWidth = nodeSize.width;
    this.childrenHeight = nodeSize.height;
    this.offsetX = (this.colWidth - this.childrenWidth) / 2.0; // half of the space between columns
    this.offsetY = (this.rowHeight - this.childrenHeight) / 2.0; // half of the space between rows
  }

  /** Returns [cols, rows]. */
  calculateNumberOfRowsAndCols(numChildren: number, boundX: number, boundY: number, boundWidth: number, boundHeight: number): [number, number] {
    // Both loops below only shrink or grow towards numChildren, so zero never terminates them.
    if (numChildren <= 0) {
      return [1, 1];
    }
    if (this.aspectRatio === 1.0) {
      return this.calculateNumberOfRowsAndCols_square(numChildren, boundX, boundY, boundWidth, boundHeight);
    }
    return this.calculateNumberOfRowsAndCols_rectangular(numChildren);
  }

  calculateNumberOfRowsAndCols_rectangular(numChildren: number): [number, number] {
    const rows = max(1, Math.ceil(Math.sqrt(numChildren)));
    const cols = max(1, Math.ceil(Math.sqrt(numChildren)));
    return [toInt(rows), toInt(cols)];
  }

  calculateNumberOfRowsAndCols_square(
    numChildren: number,
    _boundX: number,
    _boundY: number,
    boundWidth: number,
    boundHeight: number,
  ): [number, number] {
    let rows = toInt(max(1, Math.sqrt((numChildren * boundHeight) / boundWidth)));
    let cols = toInt(max(1, Math.sqrt((numChildren * boundWidth) / boundHeight)));

    // if space is taller than wide, adjust rows first
    if (boundWidth <= boundHeight) {
      // decrease number of rows and columns until just enough or not enough
      while (rows * cols > numChildren) {
        if (rows > 1) {
          rows -= 1;
        }
        if (rows * cols > numChildren) {
          if (cols > 1) {
            cols += 1;
          }
        }
      }
      // increase number of rows and columns until just enough
      while (rows * cols < numChildren) {
        rows += 1;
        if (rows * cols < numChildren) {
          cols += 1;
        }
      }
    } else {
      while (rows * cols > numChildren) {
        if (cols > 1) {
          cols -= 1;
        }
        if (rows * cols > numChildren) {
          if (rows > 1) {
            rows -= 1;
          }
        }
      }
      while (rows * cols < numChildren) {
        cols += 1;
        if (rows * cols < numChildren) {
          rows += 1;
        }
      }
    }
    return [cols, rows];
  }

  calculateNodeSize(colWidth: number, rowHeight: number): { width: number; height: number } {
    let childW = max(this.MIN_ENTITY_SIZE, this.PADDING_PERCENTAGE * colWidth);
    let childH = max(this.MIN_ENTITY_SIZE, this.PADDING_PERCENTAGE * (rowHeight - this.rowPadding));
    const whRatio = colWidth / rowHeight;
    if (whRatio < this.aspectRatio) {
      childH = childW / this.aspectRatio;
    } else {
      childW = childH * this.aspectRatio;
    }
    return { width: childW, height: childH };
  }
}
