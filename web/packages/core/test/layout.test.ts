import { test } from "node:test";
import assert from "node:assert/strict";
import {
  DiagramItem,
  Element,
  ElementModel,
  ElementModelStore,
  GridLayout,
  LayoutContext,
  SpringLayout,
  TreeLayout,
  UndoManager,
  makeDrand48,
} from "../src/index.ts";
import type { LayoutScene, Rect } from "../src/index.ts";

// The Swift tests measured real drawables; here every item is a fixed 60x30 box at its position.
const ITEM_W = 60;
const ITEM_H = 30;

class FixedScene implements LayoutScene {
  items: DiagramItem[];
  constructor(items: DiagramItem[]) {
    this.items = items;
  }
  getItemBounds(node: DiagramItem): Rect | null {
    return this.items.includes(node) ? { x: node.x, y: node.y, width: ITEM_W, height: ITEM_H } : null;
  }
  getBounds(): Rect {
    if (this.items.length === 0) {
      return { x: 0, y: 0, width: 0, height: 0 };
    }
    const minX = Math.min(...this.items.map((i) => i.x));
    const minY = Math.min(...this.items.map((i) => i.y));
    const maxX = Math.max(...this.items.map((i) => i.x + ITEM_W));
    const maxY = Math.max(...this.items.map((i) => i.y + ITEM_H));
    return { x: minX, y: minY, width: maxX - minX, height: maxY - minY };
  }
}

// A diagram with `count` items laid out in a column, plus links chaining them.
function makeDiagram(count: number, linked = false): [ElementModelStore, Element, DiagramItem[]] {
  const model = new ElementModel();
  const diagram = new Element("Diagram");
  model.add(diagram);

  const items: DiagramItem[] = [];
  for (let i = 0; i < count; i++) {
    const item = new DiagramItem("Item", `Item${i}`);
    item.x = i * 10;
    item.y = i * 10;
    diagram.add(item);
    items.push(item);
  }
  if (linked) {
    for (let i = 1; i < Math.max(count, 1); i++) {
      diagram.addSourceTarget(items[i - 1]!, items[i]!);
    }
  }
  return [new ElementModelStore(model), diagram, items];
}

function makeContext(store: ElementModelStore, element: Element, items: DiagramItem[], bounds: Rect = { x: 0, y: 0, width: 800, height: 600 }) {
  return new LayoutContext(element, new FixedScene(items), store, bounds);
}

const noop = () => {};

// LayoutContext

test("testFindNodesSkipsLinks", () => {
  const [store, diagram, items] = makeDiagram(3, true);
  const context = makeContext(store, diagram, items);
  assert.equal(context.nodes.length, items.length);
  assert.ok(context.nodes.every((n) => n.kind === "Item"));
});

test("testFindEdgesReturnsOnlyLinks", () => {
  const [store, diagram, items] = makeDiagram(3, true);
  const context = makeContext(store, diagram, items);
  assert.equal(context.edges.length, 2);
  assert.ok(context.edges.every((e) => e.kind === "Link"));
});

test("testViewBoundsAreTheOnesPassedIn", () => {
  const [store, diagram, items] = makeDiagram(1);
  const bounds = { x: 5, y: 6, width: 100, height: 200 };
  assert.deepEqual(makeContext(store, diagram, items, bounds).getViewBounds(), bounds);
});

test("testBoundsOfKnownNodeIsNonEmpty", () => {
  const [store, diagram, items] = makeDiagram(1);
  const bounds = makeContext(store, diagram, items).getBounds(items[0]!);
  assert.ok(bounds.width > 0);
  assert.ok(bounds.height > 0);
});

test("testBoundsOfUnknownNodeIsZero", () => {
  const [store, diagram, items] = makeDiagram(1);
  const stranger = new DiagramItem("Item", "NotInScene");
  assert.deepEqual(makeContext(store, diagram, items).getBounds(stranger), { x: 0, y: 0, width: 0, height: 0 });
});

test("testEveryNodeIsMovable", () => {
  const [store, diagram, items] = makeDiagram(2);
  const context = makeContext(store, diagram, items);
  assert.ok(items.every((i) => context.isMovable(i)));
});

test("testPrePostLayoutPassesRunInOrder", () => {
  const [store, diagram, items] = makeDiagram(1);
  const context = makeContext(store, diagram, items);
  const order: string[] = [];
  context.preLayoutPass = [() => order.push("pre1"), () => order.push("pre2")];
  context.postLayoutPass = [() => order.push("post")];
  context.preLayout();
  context.postLayout();
  assert.deepEqual(order, ["pre1", "pre2", "post"]);
});

test("testApplyWithoutAlgorithmReturnsNoOperations", () => {
  const [store, diagram, items] = makeDiagram(2);
  assert.equal(makeContext(store, diagram, items).apply(true).length, 0);
});

test("testApplyRunsPassesAroundTheAlgorithm", () => {
  const [store, diagram, items] = makeDiagram(2);
  const context = makeContext(store, diagram, items);
  const order: string[] = [];
  context.preLayoutPass = [() => order.push("pre")];
  context.postLayoutPass = [() => order.push("post")];
  context.layout = new GridLayout();
  context.apply(true);
  assert.deepEqual(order, ["pre", "post"]);
});

// Layout algorithms

test("testGridLayoutProducesOnePositionOperationPerNode", () => {
  const [store, diagram, items] = makeDiagram(4);
  assert.equal(new GridLayout().apply(makeContext(store, diagram, items), true).length, items.length);
});

test("testGridLayoutIsNoOpWhenNotClean", () => {
  const [store, diagram, items] = makeDiagram(4);
  assert.equal(new GridLayout().apply(makeContext(store, diagram, items), false).length, 0);
});

test("testGridLayoutMovesItemsApart", () => {
  const [store, diagram, items] = makeDiagram(4);
  for (const op of new GridLayout().apply(makeContext(store, diagram, items), true)) {
    op.apply();
  }
  assert.equal(new Set(items.map((i) => `${i.x},${i.y}`)).size, items.length);
});

test("testGridLayoutOnEmptyDiagramProducesNothing", () => {
  const [store, diagram, items] = makeDiagram(0);
  assert.equal(new GridLayout().apply(makeContext(store, diagram, items), true).length, 0);
});

// Scene is 90x60 at the origin: 3 columns x 2 rows of 30x30 cells, 28.5 box, 0.75 offset (hand-computed from the Swift formulas).
test("grid layout cell positions", () => {
  const [store, diagram, items] = makeDiagram(4);
  const layout = new GridLayout();
  const ops = layout.apply(makeContext(store, diagram, items), true);
  assert.equal(layout.cols, 3);
  assert.equal(layout.rows, 2);
  for (const op of ops) {
    op.apply();
  }
  assert.deepEqual(
    items.map((i) => [i.x, i.y]),
    [
      [0.75, 30.75],
      [30.75, 30.75],
      [60.75, 30.75],
      [0.75, 0.75],
    ],
  );
});

test("grid layout with non-1 aspect ratio uses a ceil(sqrt) square", () => {
  const layout = new GridLayout();
  layout.aspectRatio = 2;
  assert.deepEqual(layout.calculateNumberOfRowsAndCols(5, 0, 0, 100, 100), [3, 3]);
});

test("grid layout on zero-width bounds throws like the Swift Int() trap instead of looping", () => {
  assert.throws(() => new GridLayout().calculateNumberOfRowsAndCols(3, 0, 0, 0, 100), RangeError);
});

test("testSpringLayoutProducesPositionOperations", () => {
  const [store, diagram, items] = makeDiagram(5, true);
  const ops = new SpringLayout(() => 0).apply(makeContext(store, diagram, items), true);
  assert.equal(ops.length, items.length);
});

test("testSpringLayoutIsNoOpWhenNotClean", () => {
  const [store, diagram, items] = makeDiagram(5, true);
  assert.equal(new SpringLayout(() => 0).apply(makeContext(store, diagram, items), false).length, 0);
});

test("testSpringLayoutKeepsItemsWithinFiniteCoordinates", () => {
  const [store, diagram, items] = makeDiagram(6, true);
  for (const op of new SpringLayout(() => 0).apply(makeContext(store, diagram, items), true)) {
    op.apply();
  }
  for (const item of items) {
    assert.ok(Number.isFinite(item.x));
    assert.ok(Number.isFinite(item.y));
  }
});

test("testSpringLayoutOnSingleNode", () => {
  const [store, diagram, items] = makeDiagram(1);
  assert.equal(new SpringLayout(() => 0).apply(makeContext(store, diagram, items), true).length, 1);
});

test("testTreeLayoutIsStillAStub", () => {
  const [store, diagram, items] = makeDiagram(3, true);
  assert.equal(new TreeLayout().apply(makeContext(store, diagram, items), true).length, 0);
});

// Beyond the Swift tests

test("spring layout is deterministic with a fixed clock", () => {
  const run = () => {
    const [store, diagram, items] = makeDiagram(5, true);
    for (const op of new SpringLayout(() => 0).apply(makeContext(store, diagram, items), true)) {
      op.apply();
    }
    return items.map((i) => [i.x, i.y]);
  };
  assert.deepEqual(run(), run());
});

test("spring layout stops iterating once the clock passes maxTimeMS", () => {
  const [store, diagram, items] = makeDiagram(3, true);
  let reads = 0;
  const layout = new SpringLayout(() => (reads++ === 0 ? 0 : 1)); // the first read is the start time, then a full budget has passed
  layout.apply(makeContext(store, diagram, items), true);
  assert.equal(layout.iteration, 1001);
});

test("spring layout random placement uses the injected source", () => {
  const [store, diagram, items] = makeDiagram(4);
  const draws = [0.1, 0.2, 0.3, 0.4];
  let n = 0;
  const layout = new SpringLayout(() => 0, () => draws[n++]!);
  layout.sprRandom = true;
  layout.sprIterations = 0;
  layout.fitWithinBoundsEnabled = false;
  const ops = layout.apply(makeContext(store, diagram, items, { x: 0, y: 0, width: 100, height: 200 }), true);
  for (const op of ops) {
    op.apply();
  }
  assert.deepEqual(
    items.map((i) => [i.x, i.y]),
    [
      [0, 0],
      [100, 200],
      [10, 40],
      [30, 80],
    ],
  );
});

test("drand48 matches the POSIX default-seed sequence", () => {
  const r = makeDrand48();
  // Values printed by the C library's unseeded drand48() on macOS.
  assert.deepEqual([r(), r(), r()], [0.39646477376027534, 0.84048536941142515, 0.35333609724524351]);
});

test("layout operations apply through the store and undo restores positions", () => {
  const [store, diagram, items] = makeDiagram(4);
  const before = items.map((i) => [i.x, i.y]);
  const undo = new UndoManager();
  const ops = new GridLayout().apply(makeContext(store, diagram, items), true);
  store.compositeOperation(diagram, undo, noop, ops);
  assert.notDeepEqual(items.map((i) => [i.x, i.y]), before);
  undo.undo();
  assert.deepEqual(items.map((i) => [i.x, i.y]), before);
  undo.redo();
  assert.equal(items[0]!.x, 0.75);
});
