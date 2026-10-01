import { test } from "node:test";
import assert from "node:assert/strict";
import { existsSync } from "node:fs";
import { ExecutionContext, readTenn } from "../src/index.ts";
import type { Element, TennNode } from "../src/index.ts";
import { dir, fixtureNames, readGolden, readSource } from "./fixtures.ts";

// Schema: see evalJSON in TenniarbTests/TennGoldenTests.swift.
type Tagged = { t: string; [k: string]: unknown };

function tag(v: unknown, depth: number): Tagged {
  if (v === undefined) return { t: "undefined" };
  if (v === null) return { t: "null" };
  if (typeof v === "boolean") return { t: "boolean", v };
  if (typeof v === "number") return { t: "number", v: Number.isFinite(v) ? v : Number.isNaN(v) ? "NaN" : v > 0 ? "Infinity" : "-Infinity" };
  if (typeof v === "string") return { t: "string", v };
  if (v instanceof Error) return { t: "error", name: v.name };
  if (typeof v === "function") return { t: "function" };
  if (depth > 6) return { t: "truncated" };
  if (Array.isArray(v)) return { t: "array", v: v.map((x) => tag(x, depth + 1)) };
  const obj: Record<string, Tagged> = {};
  for (const k in v as object) {
    obj[k] = tag((v as Record<string, unknown>)[k], depth + 1);
  }
  return { t: "object", v: obj };
}

// JSC error messages are engine specific, so errors keep only the tag and the name.
function sameValue(actual: Tagged, expected: Tagged): void {
  if (expected.t === "error") {
    assert.deepEqual(actual, { t: "error", name: expected["name"] });
  } else {
    assert.deepEqual(actual, expected);
  }
}

interface Entry {
  path: number[];
  kind: string;
  literal: string;
  value: Tagged;
}

function entries(props: TennNode, ev: ReadonlyMap<unknown, unknown>): Entry[] {
  const result: Entry[] = [];
  const walk = (n: TennNode, path: number[]): void => {
    if (n.token !== null && ev.has(n.token)) {
      result.push({ path, kind: n.kind, literal: n.token.literal, value: tag(ev.get(n.token), 0) });
    }
    n.children?.forEach((c, i) => walk(c, [...path, i]));
  };
  props.children?.forEach((p, i) => walk(p, [i]));
  assert.equal(result.length, ev.size, "evaluated tokens not reachable from properties");
  return result;
}

function checkEntries(actual: Entry[], expected: Entry[]): void {
  assert.equal(actual.length, expected.length);
  expected.forEach((e, i) => {
    const a = actual[i]!;
    assert.deepEqual([a.path, a.kind, a.literal], [e.path, e.kind, e.literal]);
    sameValue(a.value, e.value);
  });
}

interface ElementEval {
  path: number[];
  name: string;
  element: Entry[];
  items: { index: number; kind: string; name: string; entries: Entry[] }[];
}

// Same setup as the Swift side; width/height come from a fixed size since drawables are not ported yet.
function evaluateAll(root: Element): ElementEval[] {
  const out: ElementEval[] = [];
  const visit = (e: Element, path: number[]): void => {
    const ctx = new ExecutionContext({ itemSize: () => ({ width: 100, height: 40 }) });
    ctx.setElement(e);
    const own = entries(e.properties.node, ctx.getEvaluated(e));
    const items: ElementEval["items"] = [];
    e.items.forEach((itm, index) => {
      const ens = entries(itm.properties.node, ctx.getEvaluated(itm));
      if (ens.length > 0) {
        items.push({ index, kind: itm.kind === "Link" ? "Link" : "Item", name: itm.name, entries: ens });
      }
    });
    if (own.length > 0 || items.length > 0) {
      out.push({ path, name: e.name, element: own, items });
    }
    e.elements.forEach((c, i) => visit(c, [...path, i]));
  };
  root.elements.forEach((c, i) => visit(c, [i]));
  return out;
}

for (const f of fixtureNames()) {
  test(`eval golden ${f}`, () => {
    const model = readTenn(readSource(f));
    if (model === null) {
      assert.ok(!existsSync(dir + f.replace(/\.tenn$/, ".eval.json")), "golden exists for a document with parse errors");
      return;
    }
    const actual = evaluateAll(model);
    const name = f.replace(/\.tenn$/, ".eval.json");
    if (!existsSync(dir + name)) {
      assert.deepEqual(actual, [], "TS evaluated something Swift did not");
      return;
    }
    const expected = (JSON.parse(readGolden(name)) as { elements: ElementEval[] }).elements;
    assert.equal(actual.length, expected.length);
    expected.forEach((e, i) => {
      const a = actual[i]!;
      assert.deepEqual([a.path, a.name], [e.path, e.name]);
      checkEntries(a.element, e.element);
      assert.equal(a.items.length, e.items.length);
      e.items.forEach((it, j) => {
        const ai = a.items[j]!;
        assert.deepEqual([ai.index, ai.kind, ai.name], [it.index, it.kind, it.name]);
        checkEntries(ai.entries, it.entries);
      });
    });
  });
}
