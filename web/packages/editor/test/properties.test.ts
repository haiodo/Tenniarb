import assert from "node:assert/strict";
import { test } from "node:test";
import { COLORS, highlightRanges, parseDiagnostics } from "../src/properties.ts";

test("highlightRanges: delimiters are part of strings and expressions", () => {
  const text = 'name "A b"\nfontSize $(1 + 2)\npos 10 2.5\ntext ${w}';
  const parts = highlightRanges(text).map((r) => [r.cls, text.slice(r.from, r.to)]);
  assert.deepEqual(parts, [
    ["symbol", "name"],
    ["string", '"A b"'],
    ["symbol", "fontSize"],
    ["expression", "$(1 + 2)"],
    ["symbol", "pos"],
    ["number", "10"],
    ["number", "2.5"],
    ["symbol", "text"],
    ["expression", "${w}"],
  ]);
});

test("highlightRanges: multi-line expression block stays one range", () => {
  const text = "a ${\n  1 +\n  2\n}\nb";
  const r = highlightRanges(text).find((x) => x.cls === "expression")!;
  assert.equal(text.slice(r.from, r.to), "${\n  1 +\n  2\n}");
});

test("COLORS are the Swift TennColors", () => {
  assert.equal(COLORS.light.symbol, "#815f03");
  assert.equal(COLORS.dark.expression, "#c67c48");
});

test("parseDiagnostics: error is placed on its line, valid text has none", () => {
  assert.deepEqual(parseDiagnostics("a 1\nb 2"), []);
  const text = "a 1\nb {\n  c 2";
  const d = parseDiagnostics(text);
  assert.ok(d.length > 0);
  for (const x of d) assert.ok(x.from >= 0 && x.from <= x.to && x.to <= text.length);
  assert.ok(d[0]!.from >= text.indexOf("b {"));
});
