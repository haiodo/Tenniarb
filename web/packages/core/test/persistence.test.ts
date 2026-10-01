import { test } from "node:test";
import assert from "node:assert/strict";
import { existsSync } from "node:fs";
import { TennParser, parseTenn, readTenn, toTennStr } from "../src/index.ts";
import type { DiagramItem, Element, LinkItem, TennNode } from "../src/index.ts";
import { toSwiftToken } from "./swift-units.ts";
import { dir, fixtureNames, readGolden, readSource } from "./fixtures.ts";

// Shape documented above modelJSON in TenniarbTests/TennGoldenTests.swift. tokens: null drops positions (round trip).
function nodeJson(n: TennNode, source: string | null): object {
  return {
    kind: n.kind,
    ...(n.token !== null && { token: source !== null ? toSwiftToken(n.token, source) : { type: n.token.type, literal: n.token.literal } }),
    ...(n.children !== null && { children: n.children.map((c) => nodeJson(c, source)) }),
  };
}

function itemJson(i: DiagramItem, owner: Element, source: string | null): object {
  const r: Record<string, unknown> = {
    kind: i.kind === "Link" ? "Link" : "Item",
    name: i.name,
    x: i.x,
    y: i.y,
    properties: [...i.properties].map((p) => nodeJson(p, source)),
  };
  if (i.description !== null) {
    r.description = i.description;
  }
  if (i.kind === "Link") {
    const l = i as LinkItem;
    const idx = (t: DiagramItem | null) => (t === null ? null : owner.items.indexOf(t) < 0 ? null : owner.items.indexOf(t));
    r.source = idx(l.source);
    r.target = idx(l.target);
  }
  return r;
}

function modelJson(e: Element, source: string | null): object {
  const r: Record<string, unknown> = {
    kind: e.kind === "Root" ? "Root" : "Element",
    name: e.name,
    properties: [...e.properties].map((p) => nodeJson(p, source)),
    items: e.items.map((i) => itemJson(i, e, source)),
    elements: e.elements.map((c) => modelJson(c, source)),
  };
  if (e.description !== null) {
    r.description = e.description;
  }
  return r;
}

function load(source: string) {
  const parser = new TennParser();
  const tree = parser.parse(source);
  return { model: parseTenn(tree), errors: parser.errors.errors.length };
}

for (const f of fixtureNames()) {
  const base = f.replace(/\.tenn$/, "");
  const source = readSource(f);
  const { model, errors } = load(source);

  // Like Document.read: documents with parse errors are not loaded, and have no model/saved goldens.
  if (errors > 0) {
    test(`no persistence goldens for ${f}`, () => {
      assert.ok(!existsSync(`${dir}${base}.model.json`) && !existsSync(`${dir}${base}.saved.tenn`));
    });
    continue;
  }

  test(`model golden ${f}`, () => {
    assert.deepEqual(modelJson(model, source), JSON.parse(readGolden(`${base}.model.json`)));
  });

  test(`saved golden ${f}`, () => {
    assert.equal(toTennStr(model), readGolden(`${base}.saved.tenn`));
  });

  test(`round trip ${f}`, () => {
    const saved = toTennStr(model);
    const again = load(saved);
    assert.equal(again.errors, 0);
    assert.deepEqual(modelJson(again.model, null), modelJson(model, null));
  });
}

test("readTenn strips a leading BOM", () => {
  const src = 'element "A" { item "x" { pos 1 2 } }\n';
  assert.deepEqual(modelJson(readTenn("﻿" + src)!, null), modelJson(readTenn(src)!, null));
  assert.equal(readTenn("﻿" + src)!.elements.length, 1);
});

test("readTenn returns null on parse errors", () => {
  assert.equal(readTenn('element "A" { item "x'), null);
});
