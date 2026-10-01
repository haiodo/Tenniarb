// Regressions from the tenniarb.tenn Swift-vs-TS comparison (.work/tenniarb-compare).
import assert from "node:assert/strict";
import { test } from "node:test";
import { DrawableScene, cssColor } from "../src/index.ts";
import { ExecutionContext } from "@tenniarb/core";
import { parseDiagram } from "./helpers.ts";

function itemHeight(src: string): number {
  const element = parseDiagram(src);
  const context = new ExecutionContext();
  context.setElement(element);
  const scene = new DrawableScene(element, false, context, {});
  return scene.drawables.get(element.items[0]!)!.getSelectorBounds().height;
}

// SF 10pt: CTLineGetTypographicBounds 9.668 + 2.109 -> floor 11, +1 per line in calculateSize; plus 4+4 padding.
test("line height follows SF metrics, not Inter's", () => {
  assert.equal(itemHeight(`element R { item "a\\nb\\nc\\nd\\ne\\nf" { pos 0 0\n font-size 10 } }`), 6 * 12 + 8);
});

// CoreText counts the "\n" glyph in the line it ends, so a big-font body lifts the title line to its height.
test("calculateSize: the newline after the title takes the body font", () => {
  const src = (bodyFont: number) =>
    `element R { item "T" { pos 0 0\n body { text "B\\nr"\n font-size ${bodyFont} } } }`;
  // SF 37pt: floor(37 * 2412 / 2048) + 1 = 44 per line, three lines (T, B, r), 4+4 padding.
  assert.equal(itemHeight(src(37)), 3 * 44 + 8);
});

// CGColor(red:green:blue:) is Generic RGB; Swift's sRGB bitmap shows red as #FF2600 and 0.2 gray as 66.
test("cssColor converts Generic RGB to sRGB like CGContext", () => {
  assert.equal(cssColor({ r: 1, g: 0, b: 0, a: 1 }), "rgba(255,38,0,1)");
  assert.equal(cssColor({ r: 0.2, g: 0.2, b: 0.2, a: 1 }), "rgba(66,66,66,1)");
  assert.equal(cssColor({ r: 1, g: 1, b: 1, a: 0.5 }), "rgba(255,255,255,0.5)");
});
