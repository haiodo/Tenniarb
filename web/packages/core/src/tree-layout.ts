import type { ElementOperation } from "./element-operations.ts";
import type { LayoutAlgorithm, LayoutContext } from "./layout-base.ts";

// Swift: an empty stub (direction/bounds/leftSize/layerSize fields are unused there too).
export class TreeLayout implements LayoutAlgorithm {
  apply(_context: LayoutContext, _clean: boolean): ElementOperation[] {
    return [];
  }
}
