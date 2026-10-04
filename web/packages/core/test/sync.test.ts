import { test } from "node:test";
import assert from "node:assert/strict";
import { LinkItem, readTenn, toSyncJson } from "../src/index.ts";

const model = readTenn('element "A/B" {\n item "x\\ny" {\n pos 10 20.5\n description "d"\n fill red\n }\n item "z"\n}');
test("toSyncJson: items, edges, Swift JSONEncoder layout", () => {
  assert.ok(model);
  const el = model.elements[0]!;
  const a = el.items[0]!;
  const b = el.items[1]!;
  el.add(new LinkItem("Link", "", a, b));
  const json = toSyncJson(el);
  const o = JSON.parse(json.replaceAll("\\/", "/"));
  assert.equal(o.name, "A/B");
  assert.equal(o.items.length, 2);
  assert.equal(o.items[0].name, "x\\ny");
  assert.deepEqual(o.items[0].pos, { x: 10, y: 20.5 });
  assert.equal(o.items[0].description, "d");
  assert.deepEqual(o.items[0].properties, [["fill", "red"]]);
  assert.equal(o.items[1].properties, undefined);
  assert.equal(o.description, undefined);
  assert.match(json, /^ {2}"name" : "A\\\/B",/m);
});
