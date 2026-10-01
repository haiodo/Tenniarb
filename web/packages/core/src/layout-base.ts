import type { DiagramItem, Element, LinkItem } from "./element-model.ts";
import type { ElementModelStore, ElementOperation } from "./element-operations.ts";

export interface Rect {
  x: number;
  y: number;
  width: number;
  height: number;
}

/** What the layouts need from the scene (Swift: DrawableScene); the renderer implements it. */
export interface LayoutScene {
  /** Bounds of the whole scene. */
  getBounds(): Rect;
  /** Bounds of the item's drawable, null when the item has none (treated as an empty rect). */
  getItemBounds(node: DiagramItem): Rect | null;
}

export interface LayoutAlgorithm {
  apply(context: LayoutContext, clean: boolean): ElementOperation[];
}

// Swift's generic max/min are `y >= x ? y : x` and `y < x ? y : x`, which differ from Math.max/min on NaN.
export function max(x: number, y: number): number {
  return y >= x ? y : x;
}

export function min(x: number, y: number): number {
  return y < x ? y : x;
}

// Swift's Int(Double) traps on NaN/infinity; an infinite loop in the callers would be worse.
export function toInt(x: number): number {
  if (!Number.isFinite(x)) {
    throw new RangeError(`not representable as Int: ${x}`);
  }
  return Math.trunc(x);
}

export class LayoutContext {
  // A container to perform layout on
  element: Element;

  preLayoutPass: (() => void)[] = [];
  postLayoutPass: (() => void)[] = [];

  layout: LayoutAlgorithm | null = null;
  readonly scene: LayoutScene;

  nodes: DiagramItem[] = [];
  edges: LinkItem[] = [];

  store: ElementModelStore;
  bounds: Rect;

  constructor(element: Element, scene: LayoutScene, store: ElementModelStore, bounds: Rect) {
    this.element = element;
    this.scene = scene;
    this.store = store;
    this.bounds = bounds;

    this.nodes = this.findNodes();
    this.edges = this.findEdges();
  }

  preLayout(): void {
    for (const pre of this.preLayoutPass) {
      pre();
    }
  }

  postLayout(): void {
    for (const post of this.postLayoutPass) {
      post();
    }
  }

  getBounds(): Rect;
  getBounds(node: DiagramItem): Rect;
  getBounds(node?: DiagramItem): Rect {
    if (node === undefined) {
      return this.scene.getBounds();
    }
    return this.scene.getItemBounds(node) ?? { x: 0, y: 0, width: 0, height: 0 };
  }

  getViewBounds(): Rect {
    return this.bounds;
  }

  apply(clean: boolean): ElementOperation[] {
    if (this.layout === null) {
      return [];
    }
    this.preLayout();
    const ops = this.layout.apply(this, clean);
    this.postLayout();
    return ops;
  }

  findNodes(): DiagramItem[] {
    return this.element.items.filter((itm) => itm.kind === "Item");
  }

  findEdges(): LinkItem[] {
    return this.element.items.filter((itm) => itm.kind === "Link") as LinkItem[];
  }

  isMovable(_node: DiagramItem): boolean {
    // For now any item are movable.
    return true;
  }

  getWeight(_link: LinkItem): number {
    return 0;
  }
}
