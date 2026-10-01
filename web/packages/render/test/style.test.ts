// Port of DrawableStyleTests.swift (non-AppKit parts).
import assert from "node:assert/strict";
import { test } from "node:test";
import { TennNode, newBlockExpr, newCommand, newToken, parse } from "@tenniarb/core";
import { DrawableItemStyle, DrawableLineStyle, DrawableStyle, SceneStyle, prepareBodyText } from "../src/index.ts";
import type { Color } from "../src/index.ts";

const apply = (source: string, style: DrawableStyle): void => style.parseStyle(parse(source).tree, new Map());

test("defaults differ between light and dark mode", () => {
  const light = new DrawableStyle(false);
  const dark = new DrawableStyle(true);
  assert.notDeepEqual(light.color, dark.color);
  assert.notDeepEqual(light.borderColor, dark.borderColor);
  assert.equal(light.fontSize, 18);
  assert.equal(light.width, null);
  assert.equal(light.height, null);
});

test("color is parsed by name, unknown name keeps the default", () => {
  const style = new DrawableStyle(false);
  apply("color red", style);
  assert.deepEqual(style.color, { r: 1, g: 0, b: 0, a: 1 });

  const unknown = new DrawableStyle(false);
  const before = unknown.color;
  apply("color notacoloratall", unknown);
  assert.deepEqual(unknown.color, before);
});

test("hex colors: 6 digits, 8 digits carry alpha, other lengths are rejected", () => {
  const style = new DrawableStyle(false);
  apply("color #336699", style);
  assert.deepEqual(style.color, { r: 0.2, g: 0.4, b: 0.6, a: 1 });
  apply("color #33669980", style);
  assert.equal(style.color.a, 128 / 255);
  const before: Color = style.color;
  apply("color #123", style);
  assert.deepEqual(style.color, before);
});

test("text color falls back to contrast with the background", () => {
  const style = new DrawableStyle(false);
  assert.equal(style.textColorValue, null);
  assert.deepEqual(style.textColor, { r: 0, g: 0, b: 0, a: 1 }, "white box, black text");
  apply("color black", style);
  assert.deepEqual(style.textColor, { r: 1, g: 1, b: 1, a: 1 }, "dark box, white text");
  apply("text-color red", style);
  assert.deepEqual(style.textColor, { r: 1, g: 0, b: 0, a: 1 }, "explicit value wins");
});

test("font-size is clamped to the allowed range", () => {
  for (const [src, expected] of [["font-size 99", 36], ["font-size 1", 4], ["font-size 20", 20]] as const) {
    const style = new DrawableStyle(false);
    apply(src, style);
    assert.equal(style.fontSize, expected);
  }
});

test("width and height are capped at ten thousand", () => {
  const style = new DrawableStyle(false);
  apply("width 99999\nheight 99999", style);
  assert.equal(style.width, 10000);
  assert.equal(style.height, 10000);
  apply("width 120\nheight 40", style);
  assert.equal(style.width, 120);
  assert.equal(style.height, 40);
});

test("display, layer and line-style are stored verbatim", () => {
  const style = new DrawableStyle(false);
  apply("display text\nlayer background\nline-style dashed", style);
  assert.equal(style.display, "text");
  assert.equal(style.layer, "background");
  assert.equal(style.lineStyle, "dashed");
});

test("layout joins all its arguments, none gives an empty string", () => {
  const style = new DrawableStyle(false);
  apply("layout auto center", style);
  assert.equal(style.layout, "auto, center");
  apply("layout", style);
  assert.equal(style.layout, "");
});

test("line-width is parsed", () => {
  const style = new DrawableStyle(false);
  apply("line-width 2.5", style);
  assert.equal(style.lineWidth, 2.5);
});

test("shadow reads offset, blur and color; offset alone keeps default blur", () => {
  const style = new DrawableStyle(false);
  apply("shadow 2 3 7 red", style);
  assert.deepEqual(style.shadow, { width: 2, height: 3 });
  assert.equal(style.shadowBlur, 7);
  assert.notEqual(style.shadowColor, null);

  const plain = new DrawableStyle(false);
  const defaultBlur = plain.shadowBlur;
  apply("shadow 1 1", plain);
  assert.deepEqual(plain.shadow, { width: 1, height: 1 });
  assert.equal(plain.shadowBlur, defaultBlur);
  assert.equal(plain.shadowColor, null);
});

test("inherit reads name and index", () => {
  const style = new DrawableStyle(false);
  apply("inherit 3", style);
  assert.equal(style.inherit, "inherit");
  assert.equal(style.inheritIndex, 3);
});

test("unknown commands are ignored", () => {
  const style = new DrawableStyle(false);
  apply("totally-unknown 1 2 3", style);
  assert.equal(style.fontSize, 18);
});

test("reset restores defaults", () => {
  const style = new DrawableStyle(false);
  apply("font-size 30\nwidth 100\ndisplay text\nline-style dashed", style);
  style.reset();
  assert.equal(style.fontSize, 18);
  assert.equal(style.width, null);
  assert.equal(style.display, null);
  assert.equal(style.lineStyle, null);
});

test("copy carries every field and is independent of the original", () => {
  const style = new DrawableStyle(false);
  apply("color red\nfont-size 22\nwidth 120\nheight 40\ndisplay text\nlayer hover\nline-style dashed\nline-width 3", style);
  apply("shadow 1 2 3 blue", style);

  const copy = style.copy();
  for (const key of ["fontSize", "width", "height", "display", "layer", "lineStyle", "lineWidth", "shadow", "shadowBlur", "color"] as const) {
    assert.deepEqual(copy[key], style[key], key);
  }
  apply("font-size 30", style);
  assert.equal(copy.fontSize, 22);
});

test("getComponentValue accepts numbers, anything else reads as full intensity", () => {
  const style = new DrawableStyle(false);
  assert.equal(style.getComponentValue(10), 10);
  assert.equal(style.getComponentValue(20.5), 20.5);
  assert.equal(style.getComponentValue("not a number"), 255);
});

test("line style defaults: darker colour, thicker stroke, copy keeps the class", () => {
  const line = new DrawableLineStyle(false);
  assert.deepEqual(line.color, { r: 0.2, g: 0.2, b: 0.2, a: 1 });
  assert.equal(line.lineWidth, 1);
  assert.ok(line.copy() instanceof DrawableLineStyle);
  assert.deepEqual(new DrawableLineStyle(true).color, { r: 1, g: 1, b: 1, a: 1 });
});

// Evaluated values replace the written token (JSValue in Swift).
function commandWithToken(name: string, literal: string) {
  const token = newToken("expression", literal);
  return { node: newCommand(name, new TennNode("Expression", token)), token };
}

test("expression supplies a float value", () => {
  const { node, token } = commandWithToken("font-size", "10 + 5");
  const style = new DrawableStyle(false);
  style.parseStyle(newBlockExpr(node), new Map([[token, 15]]));
  assert.equal(style.fontSize, 15);
});

test("expression supplies an RGB array, with alpha as the fourth component", () => {
  const rgb = commandWithToken("color", "[255, 0, 0]");
  const style = new DrawableStyle(false);
  style.parseStyle(newBlockExpr(rgb.node), new Map([[rgb.token, [255, 0, 0]]]));
  assert.deepEqual(style.color, { r: 1, g: 0, b: 0, a: 1 });

  const rgba = commandWithToken("color", "[255, 0, 0, 128]");
  style.parseStyle(newBlockExpr(rgba.node), new Map([[rgba.token, [255, 0, 0, 128]]]));
  assert.equal(style.color.a, 128 / 255);
});

test("expression supplies a color name; an unparsable value does not fall back to the token text", () => {
  const named = commandWithToken("color", "'red'");
  const style = new DrawableStyle(false);
  style.parseStyle(newBlockExpr(named.node), new Map([[named.token, "Red"]]));
  assert.deepEqual(style.color, { r: 1, g: 0, b: 0, a: 1 });

  const bad = commandWithToken("color", "42");
  const before = style.color;
  style.parseStyle(newBlockExpr(bad.node), new Map([[bad.token, 42]]));
  assert.deepEqual(style.color, before);
});

test("item style: title, marker, corner radius cap and replace, line spacing", () => {
  const style = new DrawableItemStyle(false);
  apply("title Header\nmarker dot", style);
  assert.equal(style.title, "Header");
  assert.equal(style.marker, "dot");

  apply("corner-radius 4 8 99 2", style);
  assert.deepEqual(style.cornerRadius, [4, 8, 15, 2]);
  apply("corner-radius 5", style);
  assert.deepEqual(style.cornerRadius, [5], "each declaration starts from scratch");

  apply("line-spacing 3\nfont-size 24\ndisplay text", style);
  assert.equal(style.lineSpacing, 3);
  assert.equal(style.fontSize, 24);
  assert.equal(style.display, "text");

  const copy = style.copy();
  assert.equal(copy.title, "Header");
  assert.deepEqual(copy.cornerRadius, [5]);
  assert.equal(copy.lineSpacing, 3);
});

test("scene style: grid span and default item/line styles", () => {
  assert.deepEqual(new SceneStyle(false).gridSpan, { x: 5, y: 5 });
  assert.equal(new SceneStyle(false).defaultLineStyle.fontSize, 12);

  const style = new SceneStyle(false);
  apply("grid 20 30", style);
  assert.deepEqual(style.gridSpan, { x: 20, y: 30 });

  apply("styles {\n  item {\n    font-size 24\n  }\n  line {\n    line-width 4\n  }\n}", style);
  assert.equal(style.defaultItemStyle.fontSize, 24);
  assert.equal(style.defaultLineStyle.lineWidth, 4);
  assert.notEqual(style.defaultLineStyle.fontSize, 24, "line defaults must not pick up item settings");
});

test("body text preparation", () => {
  assert.equal(prepareBodyText("\n    one\n    two\n"), "one\ntwo");
  assert.equal(prepareBodyText("\n  one\n    two\n"), "one\n  two");
  assert.equal(prepareBodyText("one\\ntwo"), "one\ntwo");
  assert.ok(!prepareBodyText("\tone").includes("\t"));
  assert.equal(prepareBodyText("\nbody\n"), "body");
  assert.equal(prepareBodyText("plain"), "plain");
  assert.equal(prepareBodyText(""), "");
});
