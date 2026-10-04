import assert from "node:assert/strict";
import { test } from "node:test";
import { readTenn } from "@tenniarb/core";
import { rowText, searchItems } from "../src/search.ts";

const el = readTenn(`element "D" {
  item "Zed" { body "first\\nsecond" }
  item "alpha" { body { text "Hello World" } }
  item "Mid"
}
element "Other" { item "Hello" }`)!.elements[0]!;

test("matches name or body, case-insensitive, sorted by name, current element only", () => {
  assert.deepEqual(searchItems(el, "hello").map((i) => i.name), ["alpha"]);
  assert.deepEqual(searchItems(el, "ZE").map((i) => i.name), ["Zed"]);
  assert.deepEqual(searchItems(el, "second").map((i) => i.name), ["Zed"]);
  assert.deepEqual(searchItems(el, "").map((i) => i.name), ["Mid", "Zed", "alpha"]);
  assert.deepEqual(searchItems(el, "nothing"), []);
});

test("row text: name, body, newlines escaped", () => {
  const [mid, zed] = searchItems(el, "");
  assert.equal(rowText(mid!), "Mid");
  assert.equal(rowText(zed!), "Zed - first\\nsecond");
});
