import { test } from "node:test";
import assert from "node:assert/strict";
import { TennParser, parseTenn } from "@tenniarb/core";
import { decodeSource, optionsFromAttrs } from "../src/run.ts";
import { drawable, elementTree, parseBool, pathOf, pickElement } from "../src/util.ts";

const parse = (text: string) => parseTenn(new TennParser().parse(text));
const root = parse(`
element "Empty" { element "Sub" { item "a" { pos 0 0 } } }
element "B" { item "b" { pos 0 0 } }
element "Sub" { item "c" { pos 0 0 } }
`);

test("pickElement: default is the first element with items", () => {
  assert.equal(pathOf(pickElement(root)!), "Empty/Sub");
});

test("pickElement: path, then name anywhere, else null", () => {
  assert.equal(pathOf(pickElement(root, "Empty/Sub")!), "Empty/Sub");
  assert.equal(pathOf(pickElement(root, "Sub")!), "Sub"); // top-level path wins
  assert.equal(pathOf(pickElement(parse(`element "A" { element "Deep" { item "x" { pos 0 0 } } }`), "Deep")!), "A/Deep");
  assert.equal(pickElement(root, "Sub/Empty"), null);
  assert.equal(pickElement(root, "nope"), null);
});

test("pickElement: no items anywhere falls back to the first element", () => {
  assert.equal(pathOf(pickElement(parse(`element "X" {}`))!), "X");
  assert.equal(pickElement(parse("")), null);
});

test("drawable: nested elements with items, depth-first", () => {
  assert.deepEqual(drawable(root).map(pathOf), ["Empty/Sub", "B", "Sub"]);
  assert.deepEqual(drawable(root.elements[0]!).map(pathOf), ["Empty/Sub"]);
  assert.deepEqual(drawable(root.elements[1]!), []);
});

test("elementTree", () => {
  const empty = root.elements[0]!;
  assert.deepEqual(elementTree(root)[0], {
    id: empty.id, name: "Empty", path: "Empty", hasItems: false,
    children: [{ id: empty.elements[0]!.id, name: "Sub", path: "Empty/Sub", hasItems: true, children: [] }],
  });
});

test("parseBool", () => {
  assert.equal(parseBool(null, true), true);
  assert.equal(parseBool(null, false), false);
  for (const v of ["false", "0", "No", " OFF "]) assert.equal(parseBool(v, true), false);
  for (const v of ["", "true", "1", "yes"]) assert.equal(parseBool(v, false), true);
});

test("optionsFromAttrs: attributes override run() defaults", () => {
  const attrs: Record<string, string> = { "data-element": "A/B", "data-evaluate": "false", "data-interactive": "" };
  const o = optionsFromAttrs((n) => attrs[n] ?? null, { interactive: false, fontBaseUrl: "/f/" });
  assert.deepEqual(o, { interactive: true, evaluate: false, element: "A/B", fontBaseUrl: "/f/" });
  assert.deepEqual(optionsFromAttrs(() => null, { evaluate: false }), { evaluate: false, interactive: true, element: undefined });
});

test("decodeSource: base64 is UTF-8, otherwise untouched", () => {
  const text = 'element "Привет" {}';
  assert.equal(decodeSource(Buffer.from(text).toString("base64"), "base64"), text);
  assert.equal(decodeSource(Buffer.from(text).toString("base64") + "\n", "BASE64"), text);
  assert.equal(decodeSource(text, null), text);
});

test("pickElement: element names may contain a slash", () => {
  const r = parse(`element "Styling" { element "Width/Height" { item "x" { pos 0 0 } } }`);
  const e = pickElement(r, "Styling/Width/Height")!;
  assert.equal(e.name, "Width/Height");
  assert.equal(pathOf(e), "Styling/Width/Height");
  assert.equal(pickElement(r, "Width/Height")!, e);
});
