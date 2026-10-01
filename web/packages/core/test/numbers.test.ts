import { test } from "node:test";
import assert from "node:assert/strict";
import { swiftDouble, newFloatNode, newBlockExpr, newCommand, newStrNode } from "../src/index.ts";

// Expected strings are real Swift `String(Double)` output.
const table: [number, string][] = [
  [0, "0.0"], [-0, "-0.0"], [1, "1.0"], [-3, "-3.0"], [100, "100.0"], [0.5, "0.5"],
  [0.1 + 0.2, "0.30000000000000004"], [123456789.123, "123456789.123"],
  [1e15, "1000000000000000.0"], [8e15, "8000000000000000.0"],
  [9007199254740991, "9007199254740991.0"], [9007199254740992, "9007199254740992.0"],
  [9007199254740994, "9.007199254740994e+15"], [9.5e15, "9.5e+15"], [1e16, "1e+16"], [1.5e16, "1.5e+16"],
  [1.2345678901234568e17, "1.2345678901234568e+17"], [1e21, "1e+21"], [1e100, "1e+100"],
  [1.7976931348623157e308, "1.7976931348623157e+308"],
  [1e-4, "0.0001"], [0.00099, "0.00099"], [0.00010000000000000002, "0.00010000000000000002"],
  [9.999999999999999e-5, "9.999999999999999e-05"], [1e-5, "1e-05"], [3e-5, "3e-05"], [1.5e-7, "1.5e-07"],
  [-1.5e-7, "-1.5e-07"], [5e-324, "5e-324"], [-2.5e-10, "-2.5e-10"],
  [NaN, "nan"], [Infinity, "inf"], [-Infinity, "-inf"],
];

test("swiftDouble matches Swift String(Double)", () => {
  for (const [v, s] of table) {
    assert.equal(swiftDouble(v), s, String(v));
  }
});

test("newFloatNode uses it", () => {
  assert.equal(newFloatNode(1e-5).token!.literal, "1e-05");
});

test("getValue trims Swift whitespacesAndNewlines", () => {
  const b = (v: string) => newBlockExpr(newCommand("k", newStrNode(v)));
  assert.equal(b("\u00855").getValue("k", 0), 5);
  assert.equal(b("​5​").getValue("k", 0), 5);
  assert.equal(b("﻿5").getValue("k", 0), 0);
  assert.equal(b("​ x 　").getValue("k", ""), "x");
  assert.equal(b("﻿x").getValue("k", ""), "﻿x");
});
