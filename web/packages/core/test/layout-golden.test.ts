import { test } from "node:test";
import assert from "node:assert/strict";
import { ElementModelStore, GridLayout, LayoutContext, SpringLayout, UpdatePosition, readTenn } from "../src/index.ts";
import type { DiagramItem, Element, ElementModel, ElementOperation, LayoutScene, Rect } from "../src/index.ts";
import { fixtureNames, readGolden, readSource } from "./fixtures.ts";

// Schema: see TenniarbTests/TennGoldenTests.swift above layoutJSON.
interface Pos {
  item: number;
  x: number;
  y: number;
}
interface LayoutEval {
  path: number[];
  name: string;
  sceneBounds: Rect;
  nodes: { item: number; name: string; x: number; y: number; bounds: Rect }[];
  edges: { item: number; source: number | null; target: number | null }[];
  grid: { viewBounds: Rect; positions: Pos[] };
  spring: { viewBounds: Rect; params: { iterations: number; maxTimeMS: number; random: boolean; move: number; strain: number; length: number; gravitation: number }; positions: Pos[] };
}

// Both layouts matched the Swift output bit-for-bit on all recorded fixtures (Math.log agreed with Darwin log there).

function elementAt(model: ElementModel, path: number[]): Element {
  let e: Element = model;
  for (const i of path) {
    e = e.elements[i]!;
  }
  return e;
}

function positions(ops: ElementOperation[], e: Element): Pos[] {
  return ops.map((op) => {
    const up = op as UpdatePosition;
    return { item: e.items.indexOf(up.item), x: up.newValue.x, y: up.newValue.y };
  });
}

function hasItems(e: Element): boolean {
  return e.items.some((i) => i.kind === "Item") || e.elements.some(hasItems);
}

function checkPositions(actual: Pos[], expected: Pos[], what: string): void {
  assert.equal(actual.length, expected.length, `${what}: position count`);
  actual.forEach((a, i) => {
    assert.deepEqual(a, expected[i], `${what}[${i}]`);
  });
}

for (const f of fixtureNames()) {
  const model = readTenn(readSource(f));
  // Swift records a layout golden for every error-free document that has an Item; readGolden throws when it is missing.
  if (model === null || !hasItems(model)) {
    continue;
  }
  test(`layout golden ${f}`, () => {
    const golden = JSON.parse(readGolden(f.replace(/\.tenn$/, ".layout.json"))) as { elements: LayoutEval[] };
    const store = new ElementModelStore(model);

    for (const ev of golden.elements) {
      const e = elementAt(model, ev.path);
      const where = `${ev.name} ${JSON.stringify(ev.path)}`;
      const sizes = new Map<DiagramItem, Rect>(ev.nodes.map((n) => [e.items[n.item]!, n.bounds]));
      const scene: LayoutScene = { getBounds: () => ev.sceneBounds, getItemBounds: (n) => sizes.get(n) ?? null };

      const lc = new LayoutContext(e, scene, store, ev.grid.viewBounds);
      assert.deepEqual(
        lc.nodes.map((n) => [e.items.indexOf(n), n.name, n.x, n.y]),
        ev.nodes.map((n) => [n.item, n.name, n.x, n.y]),
        `${where}: nodes`,
      );
      assert.deepEqual(
        lc.edges.map((l) => [e.items.indexOf(l), l.source === null ? null : e.items.indexOf(l.source), l.target === null ? null : e.items.indexOf(l.target)]),
        ev.edges.map((l) => [l.item, l.source, l.target]),
        `${where}: edges`,
      );

      checkPositions(positions(new GridLayout().apply(lc, true), e), ev.grid.positions, `${where} grid`);

      const sl = new SpringLayout();
      sl.maxTimeMS = ev.spring.params.maxTimeMS;
      assert.deepEqual(
        [sl.sprIterations, sl.sprRandom, sl.sprMove, sl.sprStrain, sl.sprLength, sl.sprGravitation],
        [ev.spring.params.iterations, ev.spring.params.random, ev.spring.params.move, ev.spring.params.strain, ev.spring.params.length, ev.spring.params.gravitation],
        `${where}: spring params`,
      );
      const slc = new LayoutContext(e, scene, store, ev.spring.viewBounds);
      checkPositions(positions(sl.apply(slc, true), e), ev.spring.positions, `${where} spring`);
    }
  });
}
