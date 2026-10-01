import { test } from "node:test";
import assert from "node:assert/strict";
import {
  AddElement,
  AddItem,
  CompositeOperation,
  DiagramItem,
  Element,
  ElementModel,
  ElementModelStore,
  ExecutionContext,
  LinkItem,
  ModelEvent,
  RemoveElement,
  RemoveItem,
  TennParser,
  UndoManager,
  UpdateElementName,
  UpdateName,
  UpdatePosition,
  ComplexUpdateElement,
  ComplexUpdateItem,
  readTenn,
  toTennAsProps,
  toTennStr,
} from "../src/index.ts";
import type { IElementModelListener, ModelEventOperation } from "../src/index.ts";

// Root model with one diagram element, plus a store over it.
function makeStore(): [ElementModelStore, Element] {
  const model = new ElementModel();
  const diagram = new Element("Diagram");
  model.add(diagram);
  return [new ElementModelStore(model), diagram];
}

function makeItem(name: string, x = 0, y = 0): DiagramItem {
  const item = new DiagramItem("Item", name);
  item.x = x;
  item.y = y;
  return item;
}

const names = (xs: { name: string }[]) => xs.map((x) => x.name);
const noop = () => {};

// 1. AddElement / RemoveElement

test("testAddElementApplyUndoIsSymmetric", () => {
  const [store, diagram] = makeStore();
  const child = new Element("Child");

  const op = new AddElement(store, diagram, child);
  assert.equal(diagram.elements.length, 0);

  op.apply();
  assert.equal(diagram.elements.length, 1);
  assert.ok(diagram.elements.includes(child));
  assert.equal(child.parent, diagram);
  assert.equal(child.model, store.model);

  op.undo();
  assert.equal(diagram.elements.length, 0);
});

test("testAddElementAtIndexRestoresPosition", () => {
  const [store, diagram] = makeStore();
  diagram.add(new Element("A"));
  diagram.add(new Element("B"));

  const op = new AddElement(store, diagram, new Element("Inserted"), 1);
  op.apply();
  assert.deepEqual(names(diagram.elements), ["A", "Inserted", "B"]);

  op.undo();
  assert.deepEqual(names(diagram.elements), ["A", "B"]);
});

test("AddElement with index -1 appends", () => {
  const [store, diagram] = makeStore();
  diagram.add(new Element("A"));
  new AddElement(store, diagram, new Element("Z"), -1).apply();
  assert.deepEqual(names(diagram.elements), ["A", "Z"]);
});

test("testRemoveElementUndoRestoresOriginalIndex", () => {
  const [store, diagram] = makeStore();
  const b = new Element("B");
  diagram.add(new Element("A"));
  diagram.add(b);
  diagram.add(new Element("C"));

  const op = new RemoveElement(store, diagram, b);
  op.apply();
  assert.deepEqual(names(diagram.elements), ["A", "C"]);

  op.undo();
  assert.deepEqual(names(diagram.elements), ["A", "B", "C"]);
});

// 2. AddItem / RemoveItem and link preservation

test("testAddItemApplyUndoIsSymmetric", () => {
  const [store, diagram] = makeStore();
  const item = makeItem("Node");

  const op = new AddItem(store, diagram, item);
  op.apply();
  assert.equal(diagram.items.length, 1);
  assert.equal(item.parent, diagram);

  op.undo();
  assert.equal(diagram.items.length, 0);
});

test("AddItem at index", () => {
  const [store, diagram] = makeStore();
  diagram.add(makeItem("A"));
  diagram.add(makeItem("B"));
  const op = new AddItem(store, diagram, makeItem("X"), 1);
  op.apply();
  assert.deepEqual(names(diagram.items), ["A", "X", "B"]);
  op.undo();
  assert.deepEqual(names(diagram.items), ["A", "B"]);
});

test("testRemoveItemUndoRestoresOriginalIndex", () => {
  const [store, diagram] = makeStore();
  const b = makeItem("B");
  diagram.add(makeItem("A"));
  diagram.add(b);
  diagram.add(makeItem("C"));

  const op = new RemoveItem(store, diagram, b);
  op.apply();
  assert.deepEqual(names(diagram.items), ["A", "C"]);

  op.undo();
  assert.deepEqual(names(diagram.items), ["A", "B", "C"]);
});

test("testRemoveItemAlsoRemovesItsLinks", () => {
  const [store, diagram] = makeStore();
  const source = makeItem("Source");
  const target = makeItem("Target");
  diagram.addSourceTarget(source, target);

  assert.equal(diagram.items.length, 3, "source, target and the link between them");

  const related = diagram.getRelatedItems(source);
  assert.equal(related.length, 2, "The item itself plus the link referencing it");

  const composite = new CompositeOperation(store, diagram, related.map((r) => new RemoveItem(store, diagram, r)));
  composite.apply();
  assert.deepEqual(names(diagram.items), ["Target"]);

  composite.undo();
  assert.equal(diagram.items.length, 3);
  const restoredLink = diagram.items.find((i): i is LinkItem => i instanceof LinkItem);
  assert.ok(restoredLink, "The link must come back with its endpoints intact");
  assert.equal(restoredLink.source, source);
  assert.equal(restoredLink.target, target);
});

// 3. Value updates

test("testUpdatePositionApplyUndo", () => {
  const [store, diagram] = makeStore();
  const item = makeItem("Node", 10, 20);
  diagram.add(item);

  const op = new UpdatePosition(store, diagram, item, { x: 10, y: 20 }, { x: 100, y: 200 });
  op.apply();
  assert.equal(item.x, 100);
  assert.equal(item.y, 200);

  op.undo();
  assert.equal(item.x, 10);
  assert.equal(item.y, 20);
});

test("UpdatePosition name matches Swift string", () => {
  const [store, diagram] = makeStore();
  const item = makeItem("N");
  const op = new UpdatePosition(store, diagram, item, { x: 1, y: 2.5 }, { x: 3, y: 4 });
  assert.equal(op.name, "UpdatePosition: N OLD:( 1.0, 2.5NEW:( 3.0, 4.0 ");
});

test("testUpdateNameApplyUndo", () => {
  const [store, diagram] = makeStore();
  const item = makeItem("Old");
  diagram.add(item);

  const op = new UpdateName(store, diagram, item, "Old", "New");
  op.apply();
  assert.equal(item.name, "New");

  op.undo();
  assert.equal(item.name, "Old");
});

test("testUpdateElementNameApplyUndo", () => {
  const [store, diagram] = makeStore();

  const op = new UpdateElementName(store, diagram, "Diagram", "Renamed");
  op.apply();
  assert.equal(diagram.name, "Renamed");

  op.undo();
  assert.equal(diagram.name, "Diagram");
});

test("testComplexUpdateItemRoundTripsProperties", () => {
  const [store, diagram] = makeStore();
  const item = makeItem("Node", 5, 7);
  diagram.add(item);

  const before = toTennAsProps(item);
  const after = new TennParser().parse("name Node\npos 42 43\ncolor red");
  const saved = toTennStr(store.model);

  const op = new ComplexUpdateItem(store, diagram, item, before, after);
  op.apply();
  assert.equal(item.x, 42);
  assert.equal(item.y, 43);
  assert.ok(item.properties.get("color"));

  op.undo();
  assert.equal(item.x, 5, "Undo must restore the property snapshot taken before apply");
  assert.equal(item.y, 7);
  assert.equal(item.properties.get("color"), null);
  assert.equal(toTennStr(store.model), saved, "name, description and properties all restored");
});

test("testComplexUpdateElementRoundTripsProperties", () => {
  const [store, diagram] = makeStore();

  const before = toTennAsProps(diagram);
  const after = new TennParser().parse("name Updated");
  const saved = toTennStr(store.model);

  const op = new ComplexUpdateElement(store, diagram, before, after);
  op.apply();
  assert.equal(diagram.name, "Updated");

  op.undo();
  assert.equal(diagram.name, "Diagram");
  assert.equal(toTennStr(store.model), saved);
});

test("ComplexUpdateItem on a link restores label and drops pos", () => {
  const [store, diagram] = makeStore();
  const a = makeItem("A");
  const b = makeItem("B");
  diagram.addSourceTarget(a, b);
  const link = diagram.items[0] as LinkItem;
  link.name = "old";

  const before = toTennAsProps(link);
  const op = new ComplexUpdateItem(store, diagram, link, before, new TennParser().parse("label new"));
  op.apply();
  assert.equal(link.name, "new");
  op.undo();
  assert.equal(link.name, "old");
});

// 4. CompositeOperation

test("testCompositeAppliesInOrderAndUndoesInReverse", () => {
  const [store, diagram] = makeStore();
  const item = makeItem("Node", 0, 0);
  diagram.add(item);

  const first = new UpdatePosition(store, diagram, item, { x: 0, y: 0 }, { x: 1, y: 1 });
  const second = new UpdatePosition(store, diagram, item, { x: 1, y: 1 }, { x: 2, y: 2 });
  const composite = new CompositeOperation(store, diagram, first, second);

  composite.apply();
  assert.equal(item.x, 2, "The last operation in the list wins");

  composite.undo();
  assert.equal(item.x, 0, "Reverse order undo must walk back through both positions");
});

test("testCompositeTracksIsUndoCalledOnChildren", () => {
  const [store, diagram] = makeStore();
  const composite = new CompositeOperation(
    store,
    diagram,
    new AddItem(store, diagram, makeItem("A")),
    new AddItem(store, diagram, makeItem("B")),
  );

  composite.apply();
  assert.ok(composite.operations.every((o) => !o.isUndoCalled));

  composite.undo();
  assert.ok(composite.operations.every((o) => o.isUndoCalled));
});

test("testCompositeEventKindIsStructureWhenChildrenDisagree", () => {
  const [store, diagram] = makeStore();
  const empty = new CompositeOperation(store, diagram, []);
  assert.equal(empty.getEventKind(), "Structure", "An empty composite falls back to the base kind");

  const item = makeItem("Node");
  const uniform = new CompositeOperation(store, diagram, new AddItem(store, diagram, item), new RemoveItem(store, diagram, item));
  assert.equal(uniform.getEventKind(), "Structure");
});

test("testCompositeNameJoinsChildNames", () => {
  const [store, diagram] = makeStore();
  const item = makeItem("Node");
  const composite = new CompositeOperation(store, diagram, new AddItem(store, diagram, item), new RemoveItem(store, diagram, item));
  assert.equal(composite.name, "AddItem,RemoveItem");
});

test("testCompositeNotifierIsTheGivenElement", () => {
  const [store] = makeStore();
  const other = new Element("Other");
  assert.equal(new CompositeOperation(store, other, []).getNotifier(), other);
});

// 5. ModelEvent collection

test("testAddItemCollectsAppendThenRemove", () => {
  const [store, diagram] = makeStore();
  const item = makeItem("Node");
  const op = new AddItem(store, diagram, item);

  op.apply();
  op.isUndoCalled = false;
  const applied = new Map<DiagramItem, ModelEventOperation>();
  op.collectItems(applied);
  assert.equal(applied.get(item), "Append");

  op.undo();
  op.isUndoCalled = true;
  const undone = new Map<DiagramItem, ModelEventOperation>();
  op.collectItems(undone);
  assert.equal(undone.get(item), "Remove");
});

test("testRemoveElementCollectsRemoveThenAppend", () => {
  const [store, diagram] = makeStore();
  const child = new Element("Child");
  diagram.add(child);
  const op = new RemoveElement(store, diagram, child);

  op.apply();
  op.isUndoCalled = false;
  const applied = new Map<Element, ModelEventOperation>();
  op.collectElements(applied);
  assert.equal(applied.get(child), "Remove");

  op.undo();
  op.isUndoCalled = true;
  const undone = new Map<Element, ModelEventOperation>();
  op.collectElements(undone);
  assert.equal(undone.get(child), "Append");
});

test("testCompositeCollectsFromEveryChild", () => {
  const [store, diagram] = makeStore();
  const a = makeItem("A");
  const b = makeItem("B");
  const composite = new CompositeOperation(store, diagram, new AddItem(store, diagram, a), new AddItem(store, diagram, b));
  composite.apply();

  const items = new Map<DiagramItem, ModelEventOperation>();
  composite.collectItems(items);
  assert.equal(items.size, 2);
  assert.equal(items.get(a), "Append");
  assert.equal(items.get(b), "Append");
});

test("testUpdateOperationCollectsUpdate", () => {
  const [store, diagram] = makeStore();
  const item = makeItem("Node");
  diagram.add(item);

  const op = new UpdateName(store, diagram, item, "Node", "Renamed");
  const items = new Map<DiagramItem, ModelEventOperation>();
  op.collectItems(items);
  assert.equal(items.get(item), "Update", "Value updates report Update regardless of direction");
});

test("AddElement and RemoveItem collect into the other map only for their kind", () => {
  const [store, diagram] = makeStore();
  const child = new Element("C");
  const add = new AddElement(store, diagram, child);
  const els = new Map<Element, ModelEventOperation>();
  const its = new Map<DiagramItem, ModelEventOperation>();
  add.collectElements(els);
  add.collectItems(its);
  assert.equal(els.get(child), "Remove", "isUndoCalled starts true, so a fresh op reports the undo direction");
  assert.equal(its.size, 0);
  add.apply();
  add.isUndoCalled = false;
  add.collectElements(els);
  assert.equal(els.get(child), "Append");

  const item = makeItem("I");
  const rm = new RemoveItem(store, diagram, item);
  const els2 = new Map<Element, ModelEventOperation>();
  rm.collectElements(els2);
  assert.equal(els2.size, 0);
  rm.isUndoCalled = false;
  rm.collectItems(its);
  assert.equal(its.get(item), "Remove");
  rm.isUndoCalled = true;
  rm.collectItems(its);
  assert.equal(its.get(item), "Append");
});

// 6. Store execute, undo manager and listeners

class RecordingListener implements IElementModelListener {
  events: ModelEvent[] = [];
  notifyChanges(event: ModelEvent): void {
    this.events.push(event);
  }
}

test("testExecuteAppliesAndNotifiesListener", () => {
  const [store, diagram] = makeStore();
  const listener = new RecordingListener();
  store.onUpdate = [listener];

  const item = makeItem("Node");
  store.addItem(diagram, item, null, noop);

  assert.equal(diagram.items.length, 1);
  assert.equal(listener.events.length, 1);
  const event = listener.events[0]!;
  assert.equal(event.element, diagram);
  assert.equal(event.kind, "Structure");
  assert.equal(event.items.get(item), "Append");
  assert.ok(store.modified);
});

test("testExecuteCallsRefreshAfterApply", () => {
  const [store, diagram] = makeStore();
  let refreshed = 0;
  store.addItem(diagram, makeItem("Node"), null, () => {
    refreshed++;
    assert.equal(diagram.items.length, 1);
  });
  assert.equal(refreshed, 1);
});

test("execute calls the executionContext hook before listeners", () => {
  const [store, diagram] = makeStore();
  const order: string[] = [];
  store.executionContext = Object.assign(new ExecutionContext({ evaluate: false }), { notifyChanges: () => order.push("ctx") });
  store.onUpdate = [{ notifyChanges: () => order.push("listener") }];
  store.addItem(diagram, makeItem("Node"), null, () => order.push("refresh"));
  assert.deepEqual(order, ["ctx", "listener", "refresh"]);
});

test("testUndoManagerRoundTripRestoresModel", () => {
  const [store, diagram] = makeStore();
  const undoManager = new UndoManager();

  undoManager.beginUndoGrouping();
  store.addItem(diagram, makeItem("Node"), undoManager, noop);
  undoManager.endUndoGrouping();
  assert.equal(diagram.items.length, 1);
  assert.ok(undoManager.canUndo);

  undoManager.undo();
  assert.equal(diagram.items.length, 0, "Undo must take the item back out");
  assert.ok(undoManager.canRedo);
  assert.ok(!undoManager.canUndo);

  undoManager.redo();
  assert.equal(diagram.items.length, 1, "Redo must put it back");
  assert.ok(undoManager.canUndo);
  assert.ok(!undoManager.canRedo);
});

test("testExecuteTogglesBetweenApplyAndUndo", () => {
  const [store, diagram] = makeStore();
  const op = new AddItem(store, diagram, makeItem("Node"));

  store.execute(op, null, noop);
  assert.equal(diagram.items.length, 1);
  assert.equal(op.isUndoCalled, false);

  store.execute(op, null, noop);
  assert.equal(diagram.items.length, 0, "Re-executing the same operation undoes it");
  assert.equal(op.isUndoCalled, true);
});

test("undo manager: ungrouped registrations are separate steps, new edit clears redo", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  store.addItem(diagram, makeItem("A"), um, noop);
  store.addItem(diagram, makeItem("B"), um, noop);

  um.undo();
  assert.deepEqual(names(diagram.items), ["A"]);
  um.undo();
  assert.deepEqual(names(diagram.items), []);
  um.redo();
  assert.deepEqual(names(diagram.items), ["A"]);

  store.addItem(diagram, makeItem("C"), um, noop);
  assert.ok(!um.canRedo, "A fresh registration drops the redo stack");
  um.undo();
  um.undo();
  assert.deepEqual(names(diagram.items), []);
  assert.ok(!um.canUndo);
});

test("undo manager: a group undoes as one step, in reverse order", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  um.beginUndoGrouping();
  store.addItem(diagram, makeItem("A"), um, noop);
  store.addItem(diagram, makeItem("B"), um, noop);
  um.endUndoGrouping();

  um.undo();
  assert.equal(diagram.items.length, 0);
  um.redo();
  assert.deepEqual(names(diagram.items), ["A", "B"]);
  um.undo();
  assert.equal(diagram.items.length, 0);
});

test("undo manager: removeAllActions and undo/redo on empty stacks", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  um.undo();
  um.redo();
  store.addItem(diagram, makeItem("A"), um, noop);
  um.removeAllActions();
  assert.ok(!um.canUndo && !um.canRedo);
  um.undo();
  assert.equal(diagram.items.length, 1);
});

test("undo manager: listeners see every step and modified is set", () => {
  const [store, diagram] = makeStore();
  const listener = new RecordingListener();
  store.onUpdate = [listener];
  const um = new UndoManager();
  const item = makeItem("A");
  assert.ok(!store.modified);
  store.addItem(diagram, item, um, noop);
  assert.ok(store.modified);
  um.undo();
  assert.ok(store.modified, "modified stays true after undoing back to the start");
  um.redo();
  assert.deepEqual(
    listener.events.map((e) => e.items.get(item)),
    ["Append", "Remove", "Append"],
  );
});

// Store entry points: every operation through the store, with undo and redo.

function undoRedo(um: UndoManager, check: { applied: () => void; undone: () => void }): void {
  check.applied();
  um.undo();
  check.undone();
  um.redo();
  check.applied();
}

test("testStoreAddElement undo redo", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  const child = new Element("Child");
  store.add(diagram, child, um, noop);
  undoRedo(um, {
    applied: () => assert.deepEqual(diagram.elements, [child]),
    undone: () => assert.deepEqual(diagram.elements, []),
  });
});

test("testStoreAddAndRemoveElements", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  const a = new Element("A");
  const b = new Element("B");

  store.addElements(diagram, [a, b], um, noop);
  assert.deepEqual(names(diagram.elements), ["A", "B"]);
  um.undo();
  assert.deepEqual(names(diagram.elements), []);
  um.redo();

  store.remove(diagram, a, um, noop);
  assert.deepEqual(names(diagram.elements), ["B"]);
  um.undo();
  assert.deepEqual(names(diagram.elements), ["A", "B"]);
  um.redo();
  assert.deepEqual(names(diagram.elements), ["B"]);
});

test("testStoreMoveElement", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  const other = new Element("Other");
  store.model.add(other);
  const keep = new Element("Keep");
  const child = new Element("Child");
  other.add(keep);
  diagram.add(child);

  store.move(child, other, um, noop, 0);
  assert.ok(!diagram.elements.includes(child));
  assert.deepEqual(names(other.elements), ["Child", "Keep"]);
  assert.equal(child.parent, other);

  um.undo();
  assert.deepEqual(names(diagram.elements), ["Child"]);
  assert.deepEqual(names(other.elements), ["Keep"]);
  assert.equal(child.parent, diagram);

  um.redo();
  assert.deepEqual(names(other.elements), ["Child", "Keep"]);
});

test("testStoreAddItem and addItems undo redo", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  store.addItem(diagram, makeItem("A"), um, noop);
  store.addItems(diagram, [makeItem("B"), makeItem("C")], um, noop);
  assert.deepEqual(names(diagram.items), ["A", "B", "C"]);
  um.undo();
  assert.deepEqual(names(diagram.items), ["A"]);
  um.redo();
  assert.deepEqual(names(diagram.items), ["A", "B", "C"]);
});

test("testStoreAddLink adds missing endpoints and the link", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  const a = makeItem("A");
  const b = makeItem("B");
  diagram.add(a);
  const props = new TennParser().parse("color red").children!;

  store.addLink(diagram, a, b, um, noop, props);
  assert.equal(diagram.items.length, 3);
  assert.deepEqual(diagram.items.slice(0, 2), [a, b], "only the missing endpoint is added, before the link");
  const link = diagram.items[2] as LinkItem;
  assert.ok(link instanceof LinkItem && link.source === a && link.target === b);
  assert.ok(link.properties.get("color"));

  um.undo();
  assert.deepEqual(diagram.items, [a]);
  um.redo();
  assert.equal(diagram.items.length, 3);
});

test("testRemoveItemThroughStoreDropsRelatedLinks", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  const source = makeItem("Source");
  const target = makeItem("Target");
  diagram.addSourceTarget(source, target);
  const original = [...diagram.items];

  store.removeItem(diagram, source, um, noop);
  assert.deepEqual(names(diagram.items), ["Target"], "Removing an endpoint drops the link too");

  um.undo();
  assert.deepEqual(diagram.items, original, "same items in the same order");
  um.redo();
  assert.deepEqual(names(diagram.items), ["Target"]);
});

test("testStoreAddAndRemoveMultipleItems", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  const a = makeItem("A");
  const b = makeItem("B");
  const c = makeItem("C");

  store.addItems(diagram, [a, b], um, noop);
  assert.equal(diagram.items.length, 2);

  diagram.addSourceTarget(a, c);
  const original = [...diagram.items];
  store.removeItems(diagram, [a, b], um, noop);
  assert.deepEqual(names(diagram.items), ["C"], "shared related items are removed once");
  um.undo();
  assert.deepEqual(diagram.items, original);
  um.redo();
  assert.deepEqual(names(diagram.items), ["C"]);
});

test("testStoreUpdateNameForItemAndElement", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  const item = makeItem("Old");
  diagram.add(item);

  store.updateName(item, "New", um, noop);
  assert.equal(item.name, "New");
  um.undo();
  assert.equal(item.name, "Old");
  um.redo();
  assert.equal(item.name, "New");

  store.updateElementName(diagram, "Renamed", um, noop);
  assert.equal(diagram.name, "Renamed");
  um.undo();
  assert.equal(diagram.name, "Diagram");
  um.redo();
  assert.equal(diagram.name, "Renamed");
});

test("testStoreUpdatePosition", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  const item = makeItem("Node", 1, 2);
  diagram.add(item);

  store.updatePosition(item, { x: 30, y: 40 }, um, noop);
  assert.deepEqual([item.x, item.y], [30, 40]);
  um.undo();
  assert.deepEqual([item.x, item.y], [1, 2]);
  um.redo();
  assert.deepEqual([item.x, item.y], [30, 40]);
});

test("testStoreSetPropertiesOnItemAndElement", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  const item = makeItem("Node", 1, 2);
  diagram.add(item);

  const saved = toTennStr(store.model);
  store.setItemProperties(diagram, item, new TennParser().parse("name Node\npos 11 12\ncolor red"), um, noop);
  assert.deepEqual([item.x, item.y], [11, 12]);
  um.undo();
  assert.deepEqual([item.x, item.y], [1, 2]);
  assert.equal(toTennStr(store.model), saved);
  um.redo();
  assert.deepEqual([item.x, item.y], [11, 12]);

  store.setProperties(diagram, new TennParser().parse("name Renamed"), um, noop);
  assert.equal(diagram.name, "Renamed");
  um.undo();
  assert.equal(diagram.name, "Diagram");
  um.redo();
  assert.equal(diagram.name, "Renamed");
});

test("testCreatePropertiesBuildsUndoableOperation", () => {
  const [store, diagram] = makeStore();
  const item = makeItem("Node", 3, 4);
  diagram.add(item);

  const op = store.createProperties(diagram, item, new TennParser().parse("name Node\npos 55 66"));
  op.apply();
  assert.equal(item.x, 55);

  op.undo();
  assert.equal(item.x, 3);
});

test("testMakeNonModifiedClearsDirtyFlag", () => {
  const [store, diagram] = makeStore();
  store.addItem(diagram, makeItem("Node"), null, noop);
  assert.ok(store.modified);

  store.makeNonModified();
  assert.ok(!store.modified);
});

test("testCompositeAddAppendsOperations", () => {
  const [store, diagram] = makeStore();
  const composite = new CompositeOperation(store, diagram, []);
  assert.equal(composite.operations.length, 0);

  composite.add(new AddItem(store, diagram, makeItem("A")), new AddItem(store, diagram, makeItem("B")));
  assert.equal(composite.operations.length, 2);
});

test("testCompositeOperation through store", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  const item = makeItem("Node", 1, 1);
  diagram.add(item);
  const ops = [store.createUpdatePosition(item, { x: 5, y: 5 }), new UpdateName(store, diagram, item, "Node", "N2")];
  store.compositeOperation(diagram, um, noop, ops);
  assert.deepEqual([item.x, item.name], [5, "N2"]);
  um.undo();
  assert.deepEqual([item.x, item.name], [1, "Node"]);
  um.redo();
  assert.deepEqual([item.x, item.name], [5, "N2"]);
});

test("testCreateUpdatePositionCapturesCurrentPositionAsOldValue", () => {
  const [store, diagram] = makeStore();
  const item = makeItem("Node", 7, 8);
  diagram.add(item);

  const op = store.createUpdatePosition(item, { x: 70, y: 80 });
  op.apply();
  assert.equal(item.x, 70);

  op.undo();
  assert.equal(item.x, 7, "Old value is the position captured at creation time");
  assert.equal(item.y, 8);
});

test("testCreateUpdateOrderMovesItemToNewIndex", () => {
  const [store, diagram] = makeStore();
  const c = makeItem("C");
  diagram.add(makeItem("A"));
  diagram.add(makeItem("B"));
  diagram.add(c);

  const composite = new CompositeOperation(store, diagram, store.createUpdateOrder(c, 0));
  composite.apply();
  assert.deepEqual(names(diagram.items), ["C", "A", "B"]);

  composite.undo();
  assert.deepEqual(names(diagram.items), ["A", "B", "C"]);
});

test("createUpdateOrder through the store: move to index, move to end (null), event", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  const listener = new RecordingListener();
  store.onUpdate = [listener];
  const [a, b, c] = [makeItem("A"), makeItem("B"), makeItem("C")];
  diagram.add(a);
  diagram.add(b);
  diagram.add(c);
  const saved = toTennStr(store.model);

  store.compositeOperation(diagram, um, noop, store.createUpdateOrder(c, 0));
  assert.deepEqual(diagram.items, [c, a, b]);
  um.undo();
  assert.deepEqual(diagram.items, [a, b, c]);
  um.redo();
  assert.deepEqual(diagram.items, [c, a, b]);
  um.undo();

  store.compositeOperation(diagram, um, noop, store.createUpdateOrder(a, null));
  assert.deepEqual(diagram.items, [b, c, a]);
  um.undo();
  assert.deepEqual(diagram.items, [a, b, c]);
  assert.equal(toTennStr(store.model), saved);

  const evt = listener.events[0]!;
  assert.equal(evt.element, diagram);
  assert.equal(evt.items.get(c), "Append", "Remove then Append of the same item: last wins");
});

test("RemoveItem/RemoveElement of a missing target: undo is a no-op", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  diagram.add(makeItem("a"));
  diagram.add(makeItem("b"));
  diagram.add(new Element("e"));
  const saved = toTennStr(store.model);

  store.execute(new RemoveItem(store, diagram, makeItem("ghost")), um, noop);
  store.execute(new RemoveElement(store, diagram, new Element("ghost")), um, noop);
  assert.equal(toTennStr(store.model), saved);
  um.undo();
  um.undo();
  assert.equal(toTennStr(store.model), saved, "no phantom insert");
});

test("Element.add throws RangeError on a bad index and leaves the model untouched", () => {
  const [, diagram] = makeStore();
  diagram.add(makeItem("a"));
  diagram.add(new Element("e"));
  assert.throws(() => diagram.add(makeItem("z"), 2), RangeError);
  assert.throws(() => diagram.add(makeItem("z"), -1), RangeError);
  assert.throws(() => diagram.add(new Element("z"), -1), RangeError);
  assert.equal(diagram.items.length, 1);
  assert.equal(diagram.elements.length, 1);
  diagram.add(makeItem("end"), 1);
  assert.deepEqual(names(diagram.items), ["a", "end"]);
  diagram.add(new Element("tail"), 99);
  assert.equal(diagram.elements[1]!.name, "tail", "elements beyond count append, as in Swift");
});

test("a throwing execute leaves no undo entry and no state change", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  const item = makeItem("a");
  const op = new AddItem(store, diagram, item, 5);
  assert.throws(() => store.execute(op, um, noop), RangeError);
  assert.ok(!um.canUndo);
  assert.ok(op.isUndoCalled, "flag not flipped");
  assert.ok(!store.modified);
  assert.equal(diagram.items.length, 0);
});

test("a listener removing itself during notify does not skip the next listener", () => {
  const [store, diagram] = makeStore();
  const calls: string[] = [];
  const l1 = {
    notifyChanges() {
      calls.push("l1");
      store.onUpdate = store.onUpdate.filter((l) => l !== l1);
    },
  };
  const l2 = { notifyChanges: () => void calls.push("l2") };
  store.onUpdate = [l1, l2];
  store.addItem(diagram, makeItem("a"), null, noop);
  assert.deepEqual(calls, ["l1", "l2"]);
});

test("re-entrant execute from a listener: every listener sees events in production order", () => {
  const [store, diagram] = makeStore();
  const log: string[] = [];
  let fired = false;
  const tag = (e: ModelEvent) => [...e.items.keys()].map((i) => i.name).join();
  store.onUpdate = [
    {
      notifyChanges(e) {
        log.push("A:" + tag(e));
        if (!fired) {
          fired = true;
          store.addItem(diagram, makeItem("inner"), null, noop);
        }
      },
    },
    { notifyChanges: (e) => void log.push("B:" + tag(e)) },
  ];
  store.addItem(diagram, makeItem("outer"), null, noop);
  assert.deepEqual(log, ["A:outer", "B:outer", "A:inner", "B:inner"]);
  assert.equal(diagram.items.length, 2);
});

test("undone ops report the opposite event kind", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  const listener = new RecordingListener();
  store.onUpdate = [listener];
  const a = makeItem("A");
  const b = makeItem("B");
  diagram.add(a);
  diagram.add(b);
  const child = new Element("Child");
  diagram.add(child);

  store.removeItem(diagram, a, um, noop);
  um.undo();
  assert.deepEqual(
    listener.events.map((e) => e.items.get(a)),
    ["Remove", "Append"],
  );
  store.remove(diagram, child, um, noop);
  um.undo();
  assert.deepEqual(
    listener.events.slice(2).map((e) => e.elements.get(child)),
    ["Remove", "Append"],
  );

  listener.events = [];
  store.updatePosition(b, { x: 5, y: 6 }, um, noop);
  um.undo();
  assert.deepEqual(
    listener.events.map((e) => [e.kind, e.items.get(b)]),
    [
      ["Structure", "Update"],
      ["Structure", "Update"],
    ],
  );
  store.setItemProperties(diagram, b, new TennParser().parse("name B\npos 1 1"), um, noop);
  um.undo();
  assert.deepEqual(
    listener.events.slice(2).map((e) => e.items.get(b)),
    ["Update", "Update"],
  );
});

test("move notifies the moved element; addElements ignores the index", () => {
  const [store, diagram] = makeStore();
  const listener = new RecordingListener();
  store.onUpdate = [listener];
  const x = new Element("X");
  const y = new Element("Y");
  diagram.add(x);
  diagram.add(y);
  const target = new Element("T");
  diagram.add(target);

  store.move(x, target, null, noop, 0);
  assert.equal(listener.events[0]!.element, x);
  assert.equal(x.parent, target);

  const p = new Element("P");
  const q = new Element("Q");
  store.addElements(diagram, [p, q], null, noop, 0);
  assert.deepEqual(names(diagram.elements.slice(-2)), ["P", "Q"]);
});

test("undo manager: nested groups close only at the outermost end", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  um.beginUndoGrouping();
  um.beginUndoGrouping();
  assert.equal(um.groupingLevel, 2);
  store.addItem(diagram, makeItem("A"), um, noop);
  um.endUndoGrouping();
  assert.ok(!um.canUndo);
  store.addItem(diagram, makeItem("B"), um, noop);
  um.endUndoGrouping();
  assert.equal(um.groupingLevel, 0);
  um.undo();
  assert.equal(diagram.items.length, 0);
  assert.throws(() => um.endUndoGrouping(), Error);
  assert.equal(um.groupingLevel, 0);
});

test("undo manager: undo/redo inside an open group and removeAllActions during undo throw", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  store.addItem(diagram, makeItem("A"), um, noop);
  um.beginUndoGrouping();
  assert.throws(() => um.undo(), Error);
  assert.throws(() => um.redo(), Error);
  um.endUndoGrouping();

  const um2 = new UndoManager();
  store.addItem(diagram, makeItem("B"), um2, () => {
    if (um2.isUndoing) {
      um2.removeAllActions();
    }
  });
  assert.throws(() => um2.undo(), Error);
  assert.equal(um2.groupingLevel, 0, "level never goes negative");
  store.addItem(diagram, makeItem("D"), um2, noop);
  assert.ok(um2.canUndo);
});

test("undo manager: registration during undo of a multi-step group, then redo of removes", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  const [a, b] = [makeItem("A"), makeItem("B")];
  diagram.add(a);
  diagram.add(b);
  const saved = toTennStr(store.model);

  um.beginUndoGrouping();
  store.removeItem(diagram, a, um, noop);
  store.removeItem(diagram, b, um, noop);
  um.endUndoGrouping();
  assert.equal(diagram.items.length, 0);

  um.undo();
  assert.equal(toTennStr(store.model), saved);
  assert.ok(um.canRedo && !um.canUndo);
  um.redo();
  assert.equal(diagram.items.length, 0);
  um.undo();
  assert.deepEqual(diagram.items, [a, b]);
});

test("undo manager: groupsByEvent collects a task's registrations into one step", async () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  um.groupsByEvent = true;
  store.addItem(diagram, makeItem("A"), um, noop);
  store.addItem(diagram, makeItem("B"), um, noop);
  await Promise.resolve();
  store.addItem(diagram, makeItem("C"), um, noop);
  await Promise.resolve();
  assert.equal(um.groupingLevel, 0);

  um.undo();
  assert.deepEqual(names(diagram.items), ["A", "B"]);
  um.undo();
  assert.equal(diagram.items.length, 0);
  um.redo();
  assert.deepEqual(names(diagram.items), ["A", "B"]);

  // undo in the same task closes the open automatic group first
  store.addItem(diagram, makeItem("D"), um, noop);
  um.undo();
  assert.deepEqual(names(diagram.items), ["A", "B"]);
  await Promise.resolve();
  assert.equal(um.groupingLevel, 0);
});

test("generated ids are RFC 4122 v4 and unique", () => {
  const ids = new Set<string>();
  for (let i = 0; i < 50; i++) {
    const id = new Element("E").id;
    assert.match(id, /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
    ids.add(id);
  }
  assert.equal(ids.size, 50);
});

// Model helpers (ElementModel.swift parts used by the operations)

test("Element.clone is deep and relinks links to cloned items", () => {
  const [, diagram] = makeStore();
  const a = makeItem("A", 1, 2);
  const b = makeItem("B");
  diagram.addSourceTarget(a, b);
  diagram.add(new Element("Sub"));
  diagram.description = "d";

  diagram.elements[0]!.add(makeItem("Deep"));
  const link0 = diagram.items.find((i): i is LinkItem => i instanceof LinkItem)!;
  link0.properties.appendContentsOf(new TennParser().parse("color red").children!);

  const copy = diagram.clone();
  assert.deepEqual(
    copy.items.map((i) => i.id).filter((id) => diagram.items.some((o) => o.id === id)),
    [],
    "item ids differ",
  );
  assert.ok(copy.items.every((i) => i.parent === copy));
  assert.ok(copy.items.find((i): i is LinkItem => i instanceof LinkItem)!.properties.get("color"), "link properties copied");
  assert.notEqual(copy.elements[0]!.items[0], diagram.elements[0]!.items[0]);
  assert.equal(copy.elements[0]!.items[0]!.name, "Deep");
  assert.equal(copy.elements[0]!.items[0]!.parent, copy.elements[0]);
  assert.equal(copy.name, "Diagram");
  assert.equal(copy.description, "d");
  assert.notEqual(copy.id, diagram.id);
  assert.equal(copy.items.length, 3);
  assert.equal(copy.elements.length, 1);
  assert.equal(copy.elements[0]!.parent, copy);
  const link = copy.items.find((i): i is LinkItem => i instanceof LinkItem)!;
  assert.ok(copy.items.includes(link.source!) && link.source !== a);
  assert.ok(copy.items.includes(link.target!) && link.target !== b);
  assert.equal(link.parent, copy);

  copy.items[0]!.name = "changed";
  assert.equal(a.name, "A", "Mutating the clone must not touch the original");

  assert.equal(diagram.clone(false, false).items.length, 0);
  assert.equal(diagram.clone(true, false).elements.length, 0);
});

test("clone: link endpoint outside the cloned element becomes null", () => {
  const [, diagram] = makeStore();
  const outside = makeItem("Out");
  const inside = makeItem("In");
  diagram.add(inside);
  diagram.add(new LinkItem("Link", "", inside, outside));
  const link = diagram.clone().items.find((i): i is LinkItem => i instanceof LinkItem)!;
  assert.equal(link.target, null);
  assert.notEqual(link.source, inside);
  assert.equal(link.source!.name, "In");
});

test("Element.findLinks / getRelatedItems / remove", () => {
  const [, diagram] = makeStore();
  const a = makeItem("A");
  const b = makeItem("B");
  const c = makeItem("C");
  diagram.add(c);
  diagram.addSourceTarget(a, b);
  const link = diagram.items[1]!;

  assert.deepEqual(diagram.findLinks(a), [link]);
  assert.deepEqual(diagram.findLinks(c), []);
  assert.deepEqual(diagram.getRelatedItems(a, true, false), [link, a]);
  assert.deepEqual(diagram.getRelatedItems(a, false, true), [a], "a is not the target of its link");
  assert.deepEqual(diagram.getRelatedItems(b, false, true), [link, b]);

  assert.equal(diagram.remove(b), 3);
  assert.equal(diagram.remove(b), -1);
  assert.equal(diagram.remove(new Element("none")), -1);
});

test("Element.addGet / addMakeItem / asElements / counts", () => {
  const [, diagram] = makeStore();
  assert.equal(diagram.asElements, null);
  const item = makeItem("X");
  assert.equal(diagram.addGet(item), item);
  const sub = new Element("Sub");
  const made = diagram.addMakeItem(sub);
  assert.equal(made.name, "Sub");
  assert.equal(made.parent, diagram);
  assert.equal(diagram.itemCount, 2);
  assert.equal(diagram.count, 1);
  assert.deepEqual(diagram.asElements, [sub]);
});

// Store operations followed by save

test("store operations then toTennStr produce the expected .tenn", () => {
  const [store, diagram] = makeStore();
  const um = new UndoManager();
  const a = makeItem("A", 10, 20);
  const b = makeItem("B", 30, 40);
  store.addItems(diagram, [a, b], um, noop);
  store.addLink(diagram, a, b, um, noop);
  store.updateName(a, "Alpha", um, noop);
  store.updatePosition(b, { x: 50, y: 60 }, um, noop);
  store.updateElementName(diagram, "Main", um, noop);

  const expected = [
    'element "Main" {',
    '    item "Alpha" {',
    "        pos 10.0 20.0",
    "    }",
    '    item "B" {',
    "        pos 50.0 60.0",
    "    }",
    '    link "Alpha" "B"',
    "}",
  ].join("\n");
  const saved = toTennStr(store.model);
  assert.equal(saved, expected);

  // Every step undone gives the saved text of the empty diagram; redo gives the same text again.
  while (um.canUndo) {
    um.undo();
  }
  assert.equal(toTennStr(store.model), 'element "Diagram" {\n}');
  while (um.canRedo) {
    um.redo();
  }
  assert.equal(toTennStr(store.model), expected);

  // And the saved text loads back into an equivalent model.
  const loaded = readTenn(saved)!;
  assert.equal(toTennStr(loaded), saved);
});
