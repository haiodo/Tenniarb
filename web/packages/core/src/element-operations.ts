// Port of ElementOperations.swift. Listeners run synchronously (Swift hops to the main actor).
// Swift overloads are split by name: collectItems/collectElements, addItem/addLink/removeItem/setItemProperties...
import type { TennNode, TennToken } from "./model.ts";
import { swiftDouble } from "./model.ts";
import { DiagramItem, Element, ElementModel, LinkItem } from "./element-model.ts";
import { fromTennProps, toTennAsProps } from "./element-properties.ts";
import type { UndoManager } from "./undo-manager.ts";

export type ModelEventKind = "Structure" | "Layout";

export type ModelEventOperation = "Append" | "Remove" | "Update";

export interface Point {
  x: number;
  y: number;
}

export class ModelEvent {
  readonly kind: ModelEventKind;
  element: Element;
  items = new Map<DiagramItem, ModelEventOperation>();
  elements = new Map<Element, ModelEventOperation>();

  constructor(kind: ModelEventKind, element: Element) {
    this.kind = kind;
    this.element = element;
  }
}

export class ElementOperation {
  store: ElementModelStore;
  isUndoCalled = true;
  get name(): string {
    return "Unnamed";
  }

  constructor(store: ElementModelStore) {
    this.store = store;
  }
  apply(): void {}
  undo(): void {}

  getNotifier(): Element {
    return this.store.model;
  }
  getEventKind(): ModelEventKind {
    return "Structure";
  }
  collectItems(_items: Map<DiagramItem, ModelEventOperation>): void {}
  collectElements(_elements: Map<Element, ModelEventOperation>): void {}
}

/** A composite operation for Element inside changes. */
export class CompositeOperation extends ElementOperation {
  operations: ElementOperation[];
  notifier: Element;

  constructor(store: ElementModelStore, notifier: Element, ...ops: (ElementOperation | ElementOperation[])[]) {
    super(store);
    this.operations = ops.flat();
    this.notifier = notifier;
  }

  add(...ops: ElementOperation[]): void {
    this.operations.push(...ops);
  }

  override getEventKind(): ModelEventKind {
    // Check if all operations are add operations, when it is add.
    if (this.operations.length === 0) {
      return super.getEventKind();
    }
    const kind = this.operations[0]!.getEventKind();
    for (const op of this.operations) {
      if (kind !== op.getEventKind()) {
        // If not same, return Structure.
        return super.getEventKind();
      }
    }
    return kind;
  }

  override get name(): string {
    return this.operations.map((op) => op.name).join(",");
  }

  override apply(): void {
    for (const op of this.operations) {
      op.apply();
      op.isUndoCalled = false;
    }
  }
  override undo(): void {
    for (const op of [...this.operations].reverse()) {
      op.undo();
      op.isUndoCalled = true;
    }
  }
  override getNotifier(): Element {
    return this.notifier;
  }
  override collectItems(items: Map<DiagramItem, ModelEventOperation>): void {
    for (const op of this.operations) {
      op.collectItems(items);
    }
  }
  override collectElements(elements: Map<Element, ModelEventOperation>): void {
    for (const op of this.operations) {
      op.collectElements(elements);
    }
  }
}

export interface IElementModelListener {
  notifyChanges(event: ModelEvent): void;
}

/** The part of Swift ExecutionContext the app calls. */
export interface IExecutionContext {
  setScaleFactor(factor: number): void;
  setElement(element: Element): void;
  updateAll(notifier: () => void): void;
  notifyChanges(event: ModelEvent): void;
  /** With `node`: values of that node evaluated in the item/element scope, `drawable` as in the scene. */
  getEvaluated(
    target: Element | DiagramItem,
    node?: TennNode,
    drawable?: { width: number; height: number } | null,
  ): ReadonlyMap<TennToken, unknown>;
}

export class ElementModelStore {
  readonly model: ElementModel;

  onUpdate: IElementModelListener[] = [];
  modified = false;
  executionContext: IExecutionContext | null = null;
  private pending: ModelEvent[] = [];
  private notifying = false;

  constructor(model: ElementModel) {
    this.model = model;
  }

  execute(action: ElementOperation, undoManager: UndoManager | null, refresh: () => void): void {
    if (!action.isUndoCalled) {
      action.undo();
      action.isUndoCalled = true;
    } else {
      action.apply();
      action.isUndoCalled = false;
    }
    // Registered after apply so a throwing op leaves no undo entry. The same action is re-registered, so undo/redo toggle.
    undoManager?.registerUndo(() => this.execute(action, undoManager, refresh));

    const evt = new ModelEvent(action.getEventKind(), action.getNotifier());
    action.collectItems(evt.items);
    action.collectElements(evt.elements);

    this.executionContext?.notifyChanges(evt);
    this.modifiedEvent(evt);
    refresh();
  }

  /** Swift: updateName(item:). */
  updateName(item: DiagramItem, newName: string, undoManager: UndoManager | null, refresh: () => void): void {
    this.execute(new UpdateName(this, item.parent!, item, item.name, newName), undoManager, refresh);
  }

  /** Swift: updateName(element:). */
  updateElementName(element: Element, newName: string, undoManager: UndoManager | null, refresh: () => void): void {
    this.execute(new UpdateElementName(this, element, element.name, newName), undoManager, refresh);
  }

  updatePosition(item: DiagramItem, newPos: Point, undoManager: UndoManager | null, refresh: () => void): void {
    this.execute(this.createUpdatePosition(item, newPos), undoManager, refresh);
  }

  createUpdatePosition(item: DiagramItem, newPos: Point): ElementOperation {
    return new UpdatePosition(this, item.parent!, item, { x: item.x, y: item.y }, newPos);
  }

  createUpdateOrder(item: DiagramItem, newPos: number | null): ElementOperation[] {
    return [new RemoveItem(this, item.parent!, item), new AddItem(this, item.parent!, item, newPos)];
  }

  compositeOperation(notifier: Element, undoManager: UndoManager | null, refresh: () => void, operations: ElementOperation[]): void {
    this.execute(new CompositeOperation(this, notifier, operations), undoManager, refresh);
  }

  /** Swift: add(_ parent, _ child: Element). */
  add(parent: Element, child: Element, undoManager: UndoManager | null, refresh: () => void, index: number | null = null): void {
    this.execute(new AddElement(this, parent, child, index), undoManager, refresh);
  }

  // index is accepted but unused, as in Swift.
  addElements(parent: Element, childs: Element[], undoManager: UndoManager | null, refresh: () => void, _index: number | null = null): void {
    const ops = childs.map((el) => new AddElement(this, parent, el, null));
    this.compositeOperation(parent, undoManager, refresh, ops);
  }

  move(element: Element, newParent: Element, undoManager: UndoManager | null, refresh: () => void, index: number): void {
    const op = new CompositeOperation(this, element);

    op.add(new RemoveElement(this, element.parent!, element));
    op.add(new AddElement(this, newParent, element, index));

    this.execute(op, undoManager, refresh);
  }

  remove(parent: Element, child: Element, undoManager: UndoManager | null, refresh: () => void): void {
    this.execute(new RemoveElement(this, parent, child), undoManager, refresh);
  }

  /** Swift: add(_ element, _ item: DiagramItem). */
  addItem(element: Element, item: DiagramItem, undoManager: UndoManager | null, refresh: () => void): void {
    this.execute(new AddItem(this, element, item), undoManager, refresh);
  }

  /** Swift: add(_ element, _ items: [DiagramItem]). */
  addItems(element: Element, items: DiagramItem[], undoManager: UndoManager | null, refresh: () => void): void {
    const ops = items.map((itm) => new AddItem(this, element, itm));
    this.execute(new CompositeOperation(this, element, ops), undoManager, refresh);
  }

  /** Swift: add(_ element, source:, target:). */
  addLink(
    element: Element,
    source: DiagramItem,
    target: DiagramItem,
    undoManager: UndoManager | null,
    refresh: () => void,
    props: TennNode[] = [],
  ): void {
    const op = new CompositeOperation(this, element);

    const link = new LinkItem("Link", "", source, target);
    link.properties.appendContentsOf(props);

    if (!element.items.includes(source)) {
      op.add(new AddItem(this, element, source));
    }

    if (!element.items.includes(target)) {
      op.add(new AddItem(this, element, target));
    }

    op.add(new AddItem(this, element, link));

    this.execute(op, undoManager, refresh);
  }

  /** Swift: remove(_ element, item:). */
  removeItem(element: Element, item: DiagramItem, undoManager: UndoManager | null, refresh: () => void): void {
    const op = new CompositeOperation(this, element);
    for (const it of element.getRelatedItems(item)) {
      op.add(new RemoveItem(this, element, it));
    }
    this.execute(op, undoManager, refresh);
  }

  /** Swift: remove(_ element, items:). */
  removeItems(element: Element, deleteItems: DiagramItem[], undoManager: UndoManager | null, refresh: () => void): void {
    const op = new CompositeOperation(this, element);

    const items: DiagramItem[] = [];

    for (const itm of deleteItems) {
      for (const rel of element.getRelatedItems(itm)) {
        if (!items.includes(rel)) {
          items.push(rel);
        }
      }
    }

    for (const item of items) {
      op.add(new RemoveItem(this, element, item));
    }
    this.execute(op, undoManager, refresh);
  }

  makeNonModified(): void {
    this.modified = false;
  }

  /** Swift: modified(_ event); renamed because the dirty flag is called modified too. */
  modifiedEvent(event: ModelEvent): void {
    this.modified = true;
    // A listener may execute again: queue the inner event so every listener sees events in production order.
    this.pending.push(event);
    if (this.notifying) {
      return;
    }
    this.notifying = true;
    try {
      for (let e = this.pending.shift(); e !== undefined; e = this.pending.shift()) {
        for (const op of [...this.onUpdate]) {
          op.notifyChanges(e);
        }
      }
    } finally {
      this.notifying = false;
      this.pending.length = 0;
    }
  }

  /** Swift: setProperties(_ element, _ node). */
  setProperties(element: Element, node: TennNode, undoManager: UndoManager | null, refresh: () => void): void {
    this.execute(new ComplexUpdateElement(this, element, toTennAsProps(element), node), undoManager, refresh);
  }

  /** Swift: setProperties(_ element, _ item, _ node). */
  setItemProperties(element: Element, item: DiagramItem, node: TennNode, undoManager: UndoManager | null, refresh: () => void): void {
    this.execute(new ComplexUpdateItem(this, element, item, toTennAsProps(item), node), undoManager, refresh);
  }

  createProperties(element: Element, item: DiagramItem, node: TennNode): ElementOperation {
    return new ComplexUpdateItem(this, element, item, toTennAsProps(item), node);
  }
}

abstract class AbstractUpdateValue<ValueType> extends ElementOperation {
  readonly element: Element;
  readonly item: DiagramItem;
  readonly oldValue: ValueType;
  readonly newValue: ValueType;

  constructor(store: ElementModelStore, element: Element, item: DiagramItem, old: ValueType, newValue: ValueType) {
    super(store);
    this.element = element;
    this.item = item;
    this.oldValue = old;
    this.newValue = newValue;
  }

  abstract applyValue(value: ValueType): void;

  override apply(): void {
    this.applyValue(this.newValue);
    super.apply();
  }
  override undo(): void {
    this.applyValue(this.oldValue);
    super.undo();
  }
  override collectItems(items: Map<DiagramItem, ModelEventOperation>): void {
    items.set(this.item, "Update");
  }
  override getNotifier(): Element {
    return this.element;
  }
}

abstract class AbstractUpdateElementValue<ValueType> extends ElementOperation {
  readonly element: Element;
  readonly oldValue: ValueType;
  readonly newValue: ValueType;

  constructor(store: ElementModelStore, element: Element, old: ValueType, newValue: ValueType) {
    super(store);
    this.element = element;
    this.oldValue = old;
    this.newValue = newValue;
  }

  abstract applyValue(value: ValueType): void;

  override apply(): void {
    this.applyValue(this.newValue);
    super.apply();
  }
  override undo(): void {
    this.applyValue(this.oldValue);
    super.undo();
  }
  override getNotifier(): Element {
    return this.element;
  }
}

export class UpdatePosition extends AbstractUpdateValue<Point> {
  override get name(): string {
    return (
      `UpdatePosition: ${this.item.name} OLD:( ${swiftDouble(this.oldValue.x)}, ${swiftDouble(this.oldValue.y)}` +
      `NEW:( ${swiftDouble(this.newValue.x)}, ${swiftDouble(this.newValue.y)} `
    );
  }

  override applyValue(value: Point): void {
    this.item.x = value.x;
    this.item.y = value.y;
  }
}

export class UpdateName extends AbstractUpdateValue<string> {
  override get name(): string {
    return "UpdateName";
  }

  override applyValue(value: string): void {
    this.item.name = value;
  }
}

export class ComplexUpdateItem extends AbstractUpdateValue<TennNode> {
  override get name(): string {
    return "UpdateItem";
  }

  override applyValue(value: TennNode): void {
    fromTennProps(this.item, value);
  }
}

export class ComplexUpdateElement extends AbstractUpdateElementValue<TennNode> {
  override get name(): string {
    return "UpdateElement";
  }

  override applyValue(value: TennNode): void {
    fromTennProps(this.element, value);
  }
}

export class UpdateElementName extends AbstractUpdateElementValue<string> {
  override get name(): string {
    return "UpdateElementName";
  }

  override applyValue(value: string): void {
    this.element.name = value;
  }
}

export class AddElement extends ElementOperation {
  readonly parent: Element;
  readonly child: Element;
  readonly index: number | null;
  constructor(store: ElementModelStore, element: Element, child: Element, index: number | null = null) {
    super(store);
    this.parent = element;
    this.child = child;
    this.index = index;
  }

  override get name(): string {
    return "AddElement";
  }
  override apply(): void {
    if (this.index === -1) {
      this.parent.add(this.child);
    } else {
      this.parent.add(this.child, this.index);
    }
  }
  override undo(): void {
    this.parent.remove(this.child);
  }
  override getNotifier(): Element {
    return this.parent;
  }
  override collectElements(elements: Map<Element, ModelEventOperation>): void {
    elements.set(this.child, !this.isUndoCalled ? "Append" : "Remove");
  }
}

export class AddItem extends ElementOperation {
  readonly parent: Element;
  readonly item: DiagramItem;
  readonly at: number | null;
  constructor(store: ElementModelStore, element: Element, item: DiagramItem, at: number | null = null) {
    super(store);
    this.parent = element;
    this.item = item;
    this.at = at;
  }
  override get name(): string {
    return "AddItem";
  }
  override apply(): void {
    this.parent.add(this.item, this.at);
  }
  override undo(): void {
    this.parent.remove(this.item);
  }
  override getNotifier(): Element {
    return this.parent;
  }
  override collectItems(items: Map<DiagramItem, ModelEventOperation>): void {
    items.set(this.item, !this.isUndoCalled ? "Append" : "Remove");
  }
}

export class RemoveElement extends ElementOperation {
  readonly parent: Element;
  readonly child: Element;
  removeIndex = -1;
  constructor(store: ElementModelStore, element: Element, child: Element) {
    super(store);
    this.parent = element;
    this.child = child;
  }
  override get name(): string {
    return "RemoveElement";
  }
  override apply(): void {
    this.removeIndex = this.parent.remove(this.child);
  }
  override undo(): void {
    if (this.removeIndex < 0) {
      return;
    }
    this.parent.add(this.child, this.removeIndex);
  }
  override getNotifier(): Element {
    return this.parent;
  }
  override collectElements(elements: Map<Element, ModelEventOperation>): void {
    elements.set(this.child, !this.isUndoCalled ? "Remove" : "Append");
  }
}

export class RemoveItem extends ElementOperation {
  readonly parent: Element;
  readonly child: DiagramItem;
  removeIndex = -1;
  constructor(store: ElementModelStore, element: Element, child: DiagramItem) {
    super(store);
    this.parent = element;
    this.child = child;
  }
  override get name(): string {
    return "RemoveItem";
  }
  override apply(): void {
    this.removeIndex = this.parent.remove(this.child);
  }
  override undo(): void {
    if (this.removeIndex < 0) {
      return;
    }
    this.parent.add(this.child, this.removeIndex);
  }
  override getNotifier(): Element {
    return this.parent;
  }
  override collectItems(items: Map<DiagramItem, ModelEventOperation>): void {
    items.set(this.child, !this.isUndoCalled ? "Remove" : "Append");
  }
}
