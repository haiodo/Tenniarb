// Editor state without DOM: scene, selection, drag, undo. Points are scene coordinates (y up), as in the Swift SceneDrawView.
import {
  DiagramItem,
  Element as ModelElement,
  ElementModel,
  ElementModelStore,
  LinkItem,
  TennParser,
  UndoManager,
  newBlockExpr,
  newCommand,
  newIdent,
  newIntNode,
  newMarkdownNode,
  newStrNode,
  parseItems,
  storeItems,
  toStr,
  toTennAsProps,
  toTennStr,
} from "@tenniarb/core";
import type { Element, ElementOperation, ExecutionContext } from "@tenniarb/core";
import { CircleBox, DrawableLine, EmptyBox, RoundBox, buildScene, createExecutionContext, getString, prepareBodyText, setMeasureContext } from "@tenniarb/render";
import type { Canvas2D, DrawableScene, DrawableStyle, ImageDecoder, Point, Rect } from "@tenniarb/render";
import { optionNodes } from "./styles.ts";
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
  /** The edited element changed (setElement, or the old one was removed by an undo / redo). */
  onElement?: () => void;
}

type Mode = "none" | "drag" | "band";
export type EditMode = "name" | "body";

// Wide enough that DrawableContainer.layout never culls an item moved away from the original bounds.
const EVERYWHERE: Rect = { x: -1e9, y: -1e9, width: 2e9, height: 2e9 };
// Below this (scene units) a press is a click, not a drag.
const DRAG_SLOP = 2;

/** Port of SceneDrawView.getBodyText: `body "text"` or `body { text "..." }`, unprocessed. */
function bodyText(item: DiagramItem): string {
  const block = item.properties.get("body")?.getChild(1) ?? null;
  const node = block?.kind === "BlockExpr" ? (block.getNamedElement("text")?.getChild(1) ?? null) : block;
  return getString(node, new Map()) ?? "";
}

export class EditorSession {
  readonly store: ElementModelStore;
  readonly undoManager = new UndoManager();
  scene!: DrawableScene;
  selection: DiagramItem[] = [];
  band: Rect | null = null;

  readonly root: ElementModel;
  private elementCount = 0;
  private createIndex = 1;
  // Where the next new item goes: the last click on empty space, or right of the last selected item.
  private pivot: Point = { x: 0, y: 0 };
  private readonly exec: ExecutionContext;
  private mode: Mode = "none";
  private origin: Point = { x: 0, y: 0 };
  private moved = false;
  private starts = new Map<DiagramItem, Point>();

  element: Element;
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
    if (!this.attached(this.element)) this.setElement(this.fallback());
    else this.rebuild();
    this.opts.onChange?.(this.text());
    this.opts.onRedraw?.();
  };

  private attached(e: Element): boolean {
    for (let c = e; c.parent !== null; c = c.parent) if (!c.parent.elements.includes(c)) return false;
    return true;
  }

  // Nearest ancestor that is still in the model, else the first top-level element.
  private fallback(): Element {
    let c = this.element;
    while (c.parent !== null && !this.attached(c)) c = c.parent;
    return c.kind === "Root" ? (this.root.elements[0] ?? this.element) : c;
  }

  /** Edit another element of the same document; the selection is dropped, undo history is kept. */
  setElement(element: Element): void {
    if (element === this.element) return;
    this.element = element;
    this.exec.setElement(element);
    this.selection = [];
    this.pivot = { x: 0, y: 0 };
    this.rebuild();
    this.opts.onElement?.();
    this.opts.onRedraw?.();
  }

  /** Show `item` in its element and select it. */
  reveal(item: DiagramItem): void {
    this.setElement(item.parent!);
    this.selection = [item];
    this.opts.onRedraw?.();
  }

  /** New child element (Swift handleAddElement); readonly: null. */
  addElement(parent: Element = this.root): Element | null {
    if (this.opts.readonly) return null;
    const el = new ModelElement(`Unnamed element: ${this.elementCount++}`);
    this.store.add(parent, el, this.undoManager, this.refresh);
    return el;
  }

  /** Copy with items, next to the original (Swift duplicateItem for the outline). */
  duplicateElement(element: Element): Element | null {
    if (this.opts.readonly || element.parent === null) return null;
    const copy = element.clone();
    this.store.add(element.parent, copy, this.undoManager, this.refresh);
    return copy;
  }

  renameElement(element: Element, name: string): void {
    if (this.opts.readonly || name === element.name) return;
    this.store.updateElementName(element, name, this.undoManager, this.refresh);
  }

  /** False: readonly, or it is the last top-level element (the editor needs one to show). */
  removeElement(element: Element): boolean {
    const parent = element.parent;
    if (this.opts.readonly || parent === null || (parent === this.root && parent.elements.length === 1)) return false;
    this.store.remove(parent, element, this.undoManager, this.refresh);
    return true;
  }

  /** Into `parent` as the last child. False: readonly, or a drop into itself or its own subtree. */
  moveElement(element: Element, parent: Element): boolean {
    if (this.opts.readonly || element.parent === null) return false;
    for (let c: Element | null = parent; c !== null; c = c.parent) if (c === element) return false;
    this.store.move(element, parent, this.undoManager, this.refresh, parent.elements.length);
    return true;
  }

  undo(): void {
    if (!this.opts.readonly) this.undoManager.undo();
  }

  redo(): void {
    if (!this.opts.readonly) this.undoManager.redo();
  }

  /** Right click: select what is under the cursor unless it is selected already (Swift showPopup). */
  pick(p: Point): void {
    const hit = hitTest(this.scene, p).at(-1);
    if (hit === undefined) {
      this.selection = [];
      this.pivot = p;
    } else if (!this.selection.includes(hit)) this.selection = [hit];
    this.opts.onRedraw?.();
  }

  setDarkMode(dark: boolean): void {
    this.opts.darkMode = dark;
    this.rebuild();
    this.opts.onRedraw?.();
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
      this.pivot = p;
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
    this.pivotRightOf(this.selection[0]!);
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

  /** What the text overlay shows for `item`: scene-space rect, current text and font size (px). Null when readonly. */
  editTarget(item: DiagramItem, mode: EditMode): { rect: Rect; text: string; fontSize: number } | null {
    const d = this.scene.drawables.get(item);
    if (this.opts.readonly || d === undefined) return null;
    const b = d.getSelectorBounds();
    const [width, height] = [Math.max(b.width, 100), Math.max(b.height, 20)];
    // Boxes keep their top-left corner, lines get the box at the middle of their bounds.
    const rect = d instanceof DrawableLine ? { x: b.x + (b.width - width) / 2, y: b.y + (b.height - height) / 2, width, height } : { x: b.x, y: b.y + b.height - height, width, height };
    const fontSize = ((d as { style?: DrawableStyle }).style?.fontSize ?? 18) - (mode === "body" ? 2 : 0);
    return { rect, text: this.editText(item, mode), fontSize };
  }

  private editText(item: DiagramItem, mode: EditMode): string {
    return mode === "name" ? item.name : prepareBodyText(bodyText(item));
  }

  /** Name or body of `item` as one undo step; nothing when the text is unchanged. */
  commitEdit(item: DiagramItem, mode: EditMode, text: string): void {
    if (this.opts.readonly || text === this.editText(item, mode)) return;
    if (mode === "name") return this.store.updateName(item, text, this.undoManager, this.refresh);
    const props = toTennAsProps(item, "BlockExpr");
    const bodyNode = props.getNamedElement("body");
    if (bodyNode === null) {
      props.add(newCommand("body", text.includes("\n") ? newMarkdownNode(text) : newStrNode(text)));
    } else {
      const block = bodyNode.getChild(1);
      // A block body without a "text" entry is left alone, as in Swift setBody.
      const target = block?.kind === "BlockExpr" ? block.getNamedElement("text") : block === null ? null : bodyNode;
      if (target === null) return;
      target.children = null;
      target.add(newIdent(target === bodyNode ? "body" : "text"), newMarkdownNode(text));
    }
    this.store.setItemProperties(this.element, item, props, this.undoManager, this.refresh);
  }

  /** What the properties panel edits: the first selected item, else the element (Swift activeItems.first). */
  propsTarget(): DiagramItem | Element {
    return this.selection[0] ?? this.element;
  }

  propsText(target: DiagramItem | Element): string {
    return toStr(toTennAsProps(target), 0, false);
  }

  /** Port of mergeProperties: one undo step. False: readonly, parse errors or the target is gone. */
  applyProps(target: DiagramItem | Element, text: string): boolean {
    const parser = new TennParser();
    const node = parser.parse(text);
    if (this.opts.readonly || parser.errors.hasErrors()) return false;
    if (target instanceof DiagramItem) {
      if (!this.element.items.includes(target)) return false;
      this.store.setItemProperties(this.element, target, node, this.undoManager, this.refresh);
    } else {
      this.store.setProperties(target, node, this.undoManager, this.refresh);
    }
    return true;
  }

  /** Named styles of the element (Swift StyleManager.update); the default `item` and `line` are not offered. */
  styleNames(): string[] {
    const block = this.element.properties.get("styles")?.getBlock(1) ?? [];
    return block.filter((c) => c.isNamedElement() && c.getChild(1) !== null).map((c) => c.getIdent(0)!).filter((n) => n !== "item" && n !== "line");
  }

  /** `use-style` for every selected item, one undo step (Swift StyleManager.doApply). */
  applyStyle(name: string): void {
    if (this.opts.readonly) return;
    const ops: ElementOperation[] = [];
    for (const item of this.selection) {
      const props = toTennAsProps(item, "BlockExpr");
      const cur = props.getNamedElement("use-style");
      if (cur === null) props.add(newCommand("use-style", newIdent(name)));
      else if (cur.getIdent(1) === name) continue;
      else {
        cur.children = null;
        cur.add(newIdent("use-style"), newIdent(name));
      }
      ops.push(this.store.createProperties(this.element, item, props));
    }
    if (ops.length > 0) this.store.compositeOperation(this.element, this.undoManager, this.refresh, ops);
  }

  /** Adds `new_style_N { color white }` to the element's `styles` (Swift StyleManager.addStyleConfig); returns its name. */
  defineStyle(): string | null {
    if (this.opts.readonly) return null;
    const props = this.element.properties.clone();
    let styles = props.get("styles");
    if (styles === null) {
      styles = newCommand("styles", newBlockExpr());
      props.append(styles);
    }
    const block = styles.getChild(1)!;
    const name = `new_style_${block.count + 1}`;
    block.add(newCommand(name, newBlockExpr(newCommand("color", newStrNode("white")))));
    this.store.setProperties(this.element, props.asNode(), this.undoManager, this.refresh);
    return name;
  }

  /** Sets one property of the single selected item to a quick-style option (Swift changeItemProps). */
  setQuickStyle(prop: string, option: string): void {
    const item = this.selection[0];
    if (this.opts.readonly || item === undefined || this.selection.length !== 1) return;
    const props = toTennAsProps(item, "BlockExpr");
    const cur = props.getNamedElement(prop);
    if (cur === null) props.add(newCommand(prop, ...optionNodes(prop, option)));
    else {
      cur.children = null;
      cur.add(newIdent(prop), ...optionNodes(prop, option));
    }
    this.store.setItemProperties(this.element, item, props, this.undoManager, this.refresh);
  }

  /** `item { shadow -5 -5 5 }` in the element's styles (Swift applyShadow without a selection). */
  enableShadows(): void {
    if (this.opts.readonly) return;
    const props = this.element.properties.clone();
    let styles = props.get("styles");
    if (styles === null) {
      styles = newCommand("styles", newBlockExpr());
      props.append(styles);
    }
    const block = styles.getChild(1)!;
    let item = block.getNamedElement("item");
    if (item === null) {
      item = newCommand("item", newBlockExpr());
      block.add(item);
    }
    const itemBlock = item.getChild(1)!;
    const shadow = newCommand("shadow", newIntNode(-5), newIntNode(-5), newIntNode(5));
    const old = itemBlock.getNamedElement("shadow");
    if (old === null) itemBlock.add(shadow);
    else old.children = shadow.children;
    this.store.setProperties(this.element, props.asNode(), this.undoManager, this.refresh);
  }

  /** Evaluated expressions of `text` by 0-based line (Swift updateAnnotations); empty when it does not parse. */
  propsValues(target: DiagramItem | Element, text: string): Map<number, string> {
    const parser = new TennParser();
    const node = parser.parse(text);
    const values = new Map<number, string>();
    if (parser.errors.hasErrors()) return values;
    const drawable = target instanceof DiagramItem ? (this.scene.drawables.get(target)?.getSelectorBounds() ?? null) : null;
    for (const [tok, v] of this.exec.getEvaluated(target, node, drawable)) values.set(tok.line, String(v));
    return values;
  }

  /** Selection as .tenn text (the Swift clipboard format); null when nothing is selected. */
  copyText(): string | null {
    return this.selection.length === 0 ? null : toStr(storeItems(this.selection), 0, false);
  }

  cut(): string | null {
    const text = this.copyText();
    this.deleteSelection();
    return text;
  }

  deleteSelection(): void {
    if (this.opts.readonly || this.selection.length === 0) return;
    this.store.removeItems(this.element, this.selection, this.undoManager, this.refresh);
    this.opts.onRedraw?.();
  }

  /** Adds the items of a .tenn text as they were written (Swift does not offset them) and selects them. False: not pasteable. */
  paste(text: string): boolean {
    if (this.opts.readonly) return false;
    const parser = new TennParser();
    const items = parseItems(parser.parse(text));
    if (parser.errors.hasErrors() || items.length === 0) return false;
    this.addSelected(items);
    return true;
  }

  /** Port of SceneDrawView.duplicateItem: items shifted right, links that end at them are cloned too. */
  duplicate(): void {
    if (this.opts.readonly) return;
    const copies = new Map<DiagramItem, DiagramItem>();
    const links: LinkItem[] = [];
    const items: DiagramItem[] = [];
    const seen = new Set<DiagramItem>();
    const addLink = (l: DiagramItem): void => {
      if (l.kind !== "Link" || seen.has(l)) return;
      seen.add(l);
      const c = l.clone() as LinkItem;
      links.push(c);
      items.push(c);
    };
    for (const a of this.selection) {
      if (a.kind === "Item") {
        const c = a.clone();
        c.x += 75;
        copies.set(a, c);
        items.push(c);
      }
      addLink(a);
      this.element.getRelatedItems(a, false).forEach(addLink);
    }
    for (const l of links) {
      l.source = (l.source && copies.get(l.source)) ?? l.source;
      l.target = (l.target && copies.get(l.target)) ?? l.target;
    }
    this.addSelected(items);
  }

  private pivotRightOf(item: DiagramItem): void {
    const width = this.scene.drawables.get(item)?.getBounds().width ?? 90;
    this.pivot = { x: item.x + width + 10, y: item.y };
  }

  /** Swift addTopItem (Tab in the outline): a new item at the pivot, selected. */
  addTopItem(): void {
    this.addItem(null);
  }

  /** Swift addNewItem: nothing selected - as addTopItem; an item selected - a new item linked from it; a link - nothing. */
  addNewItem(): void {
    const active = this.selection[0];
    if (active === undefined) this.addItem(null);
    else if (active.kind === "Item") this.addItem(active);
  }

  private addItem(from: DiagramItem | null): void {
    if (this.opts.readonly) return;
    const item = new DiagramItem("Item", `Untitled ${this.createIndex++}`);
    item.x = this.pivot.x;
    item.y = this.pivot.y;
    if (from === null) this.store.addItem(this.element, item, this.undoManager, this.refresh);
    else this.store.addLink(this.element, from, item, this.undoManager, this.refresh);
    this.selection = [item];
    this.pivotRightOf(item);
    this.opts.onRedraw?.();
  }

  private addSelected(items: DiagramItem[]): void {
    if (items.length === 0) return;
    this.store.addItems(this.element, items, this.undoManager, this.refresh);
    this.selection = items;
    this.opts.onRedraw?.();
  }

  /** Scene plus selection, in scene space. */
  draw(ctx: Canvas2D): void {
    this.scene.draw(ctx);
    drawSelection(ctx, this.scene, this.selection, this.band);
  }
}
