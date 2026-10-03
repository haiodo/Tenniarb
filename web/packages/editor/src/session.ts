// Editor state without DOM: scene, selection, drag, undo. Points are scene coordinates (y up), as in the Swift SceneDrawView.
import { ElementModel, ElementModelStore, LinkItem, UndoManager, toTennStr } from "@tenniarb/core";
import type { DiagramItem, Element, ElementOperation, ExecutionContext } from "@tenniarb/core";
import { CircleBox, DrawableLine, EmptyBox, RoundBox, buildScene, createExecutionContext, setMeasureContext } from "@tenniarb/render";
import type { Canvas2D, DrawableScene, ImageDecoder, Point, Rect } from "@tenniarb/render";
import { drawSelection, hitTest, itemsInRect } from "./selection.ts";

export interface SessionOptions {
  darkMode?: boolean;
  evaluate?: boolean;
  decodeImage?: ImageDecoder;
  measureContext?: Pick<Canvas2D, "font" | "measureText">;
  /** No selection and no edits. */
  readonly?: boolean;
  /** Serialized document after each committed change (also undo / redo). */
  onChange?: (text: string) => void;
  /** The scene was rebuilt or drawables moved; repaint. */
  onRedraw?: () => void;
}

type Mode = "none" | "drag" | "band";

// Wide enough that DrawableContainer.layout never culls an item moved away from the original bounds.
const EVERYWHERE: Rect = { x: -1e9, y: -1e9, width: 2e9, height: 2e9 };
// Below this (scene units) a press is a click, not a drag.
const DRAG_SLOP = 2;

export class EditorSession {
  readonly store: ElementModelStore;
  readonly undoManager = new UndoManager();
  scene!: DrawableScene;
  selection: DiagramItem[] = [];
  band: Rect | null = null;

  private readonly root: ElementModel;
  private readonly exec: ExecutionContext;
  private mode: Mode = "none";
  private origin: Point = { x: 0, y: 0 };
  private moved = false;
  private starts = new Map<DiagramItem, Point>();

  readonly element: Element;
  private readonly opts: SessionOptions;

  constructor(element: Element, opts: SessionOptions = {}) {
    this.element = element;
    this.opts = opts;
    let root: Element = element;
    while (root.parent !== null) root = root.parent;
    if (!(root instanceof ElementModel)) throw new Error("element is not attached to a model");
    this.root = root;
    this.store = new ElementModelStore(root);
    this.undoManager.groupsByEvent = true;
    // The execution context measures text while it evaluates, before the first buildScene.
    if (opts.measureContext !== undefined) setMeasureContext(opts.measureContext);
    this.exec = createExecutionContext({ evaluate: opts.evaluate, decodeImage: opts.decodeImage });
    this.exec.setElement(element);
    this.store.executionContext = this.exec;
    this.rebuild();
  }

  text(): string {
    return toTennStr(this.root);
  }

  rebuild(): void {
    this.scene = buildScene(this.element, {
      darkMode: this.opts.darkMode,
      executionContext: this.exec,
      decodeImage: this.opts.decodeImage,
      measureContext: this.opts.measureContext,
    });
    this.selection = this.selection.filter((i) => this.scene.drawables.has(i));
  }

  private refresh = (): void => {
    this.rebuild();
    this.opts.onChange?.(this.text());
    this.opts.onRedraw?.();
  };

  undo(): void {
    if (!this.opts.readonly) this.undoManager.undo();
  }

  redo(): void {
    if (!this.opts.readonly) this.undoManager.redo();
  }

  /** false: nothing was grabbed, the caller may pan. */
  down(p: Point, o: { toggle?: boolean; band?: boolean } = {}): boolean {
    this.origin = p;
    this.moved = false;
    this.mode = "none";
    if (this.opts.readonly) return false;
    const hits = hitTest(this.scene, p);
    if (o.band) {
      this.mode = "band";
      return true;
    }
    if (hits.length === 0) {
      this.selection = [];
      this.opts.onRedraw?.();
      return false;
    }
    const top = hits[hits.length - 1]!;
    if (o.toggle) {
      this.toggle(top);
      return true;
    }
    // Pressing on an already selected item keeps the whole selection so it can be dragged together.
    if (!hits.some((h) => this.selection.includes(h))) this.selection = [top];
    this.mode = "drag";
    // Links move only alone, as in Swift.
    this.starts = new Map(this.selection.filter((i) => i.kind === "Item" || this.selection.length === 1).map((i) => [i, { x: i.x, y: i.y }]));
    this.opts.onRedraw?.();
    return true;
  }

  move(p: Point): void {
    const dx = p.x - this.origin.x;
    const dy = p.y - this.origin.y;
    if (!this.moved && Math.hypot(dx, dy) < DRAG_SLOP) return;
    this.moved = true;
    if (this.mode === "band") {
      this.band = { x: Math.min(this.origin.x, p.x), y: Math.min(this.origin.y, p.y), width: Math.abs(dx), height: Math.abs(dy) };
      this.selection = itemsInRect(this.scene, this.band);
    } else if (this.mode === "drag") {
      this.moveDrawables(new Map([...this.starts].map(([i, s]) => [i, { x: s.x + dx, y: s.y + dy }])));
    }
    this.opts.onRedraw?.();
  }

  up(p: Point): void {
    this.move(p);
    const mode = this.mode;
    this.mode = "none";
    this.band = null;
    if (mode === "band" && !this.moved) {
      const hit = hitTest(this.scene, p).at(-1);
      if (hit === undefined) this.selection = [];
      else this.toggle(hit);
    }
    if (mode === "drag" && this.moved) this.commitDrag(p);
    this.opts.onRedraw?.();
  }

  private toggle(item: DiagramItem): void {
    this.selection = this.selection.includes(item) ? this.selection.filter((i) => i !== item) : [...this.selection, item];
  }

  private commitDrag(p: Point): void {
    const dx = p.x - this.origin.x;
    const dy = p.y - this.origin.y;
    const ops: ElementOperation[] = [...this.starts]
      .map(([i, s]) => ({ i, pos: { x: s.x + dx, y: s.y + dy } }))
      .filter(({ i, pos }) => pos.x !== i.x || pos.y !== i.y)
      .map(({ i, pos }) => this.store.createUpdatePosition(i, pos));
    // One composite = one undo step.
    if (ops.length > 0) this.store.compositeOperation(this.element, this.undoManager, this.refresh, ops);
  }

  /** Port of DrawableScene.updateLayout(newPositions): move the drawables and re-route attached lines. */
  moveDrawables(positions: Map<DiagramItem, Point>): void {
    const { drawables, itemToLink } = this.scene;
    for (const [item, pos] of positions) {
      const d = drawables.get(item);
      if (d instanceof RoundBox || d instanceof CircleBox || d instanceof EmptyBox) {
        const b = d.getSelectorBounds();
        d.setPath({ x: pos.x, y: pos.y - b.height, width: b.width, height: b.height });
      } else if (d instanceof DrawableLine) {
        d.control = pos;
        d.updateLayout(d.sourceRect, d.targetRect);
      }
      for (const l of itemToLink.get(item) ?? []) {
        const line = drawables.get(l);
        const sr = l instanceof LinkItem && l.source !== null ? drawables.get(l.source)?.getSelectorBounds() : undefined;
        const tr = l instanceof LinkItem && l.target !== null ? drawables.get(l.target)?.getSelectorBounds() : undefined;
        if (line instanceof DrawableLine && sr !== undefined && tr !== undefined) line.updateLayout(sr, tr);
      }
    }
    this.scene.layout(this.scene.getBounds(), EVERYWHERE);
  }

  /** Scene plus selection, in scene space. */
  draw(ctx: Canvas2D): void {
    this.scene.draw(ctx);
    drawSelection(ctx, this.scene, this.selection, this.band);
  }
}
