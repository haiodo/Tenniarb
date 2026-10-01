// Port of DrawableLineTests.swift and the primitive tests of DrawableStyleTests.swift.
import assert from "node:assert/strict";
import { test } from "node:test";
import { DiagramItem, Element, newCommand, newIdent, parse } from "@tenniarb/core";
import {
  AttributedString,
  CircleBox,
  DrawableContainer,
  DrawableLine,
  DrawableLineStyle,
  DrawableItemStyle,
  DrawableStyle,
  ElementImageProvider,
  EmptyBox,
  ItemDrawable,
  RoundBox,
  TextBox,
  arrow,
  containsRect,
  midX,
  systemFont,
} from "../src/index.ts";
import type { Rect } from "../src/index.ts";
import { loadDecoder } from "./helpers.ts";

const lineStyle = (source = ""): DrawableLineStyle => {
  const style = new DrawableLineStyle(false);
  if (source !== "") {
    style.parseStyle(parse(source).tree, new Map());
  }
  return style;
};
const makeLine = (from: Rect, to: Rect, style = new DrawableLineStyle(false)): DrawableLine => new DrawableLine(from, to, style);
const host = () => new DiagramItem("Item", "Host");
const element = (...items: DiagramItem[]): Element => {
  const el = new Element("E");
  items.forEach((i) => el.add(i));
  return el;
};

const left: Rect = { x: 0, y: 0, width: 40, height: 20 };
const right: Rect = { x: 200, y: 0, width: 40, height: 20 };
const above: Rect = { x: 0, y: 200, width: 40, height: 20 };

test("endpoints sit between the two boxes", () => {
  const line = makeLine(left, right);
  assert.ok(line.source.x >= left.x);
  assert.ok(line.target.x <= right.x + right.width);
  assert.ok(line.source.x < line.target.x, "left to right");
  assert.deepEqual(line.source, { x: 40, y: 10 });
  assert.deepEqual(line.target, { x: 200, y: 10 });
});

test("endpoints follow a vertical arrangement; moving the target moves the endpoint", () => {
  const line = makeLine(left, above);
  assert.ok(line.source.y < line.target.y, "bottom to top");

  const h = makeLine(left, right);
  const before = h.target;
  h.updateLayout(left, { ...right, x: right.x + 300 });
  assert.notDeepEqual(h.target, before);
});

test("plain line has no extra points; point constructor keeps exact coordinates", () => {
  assert.deepEqual(makeLine(left, right).extraPoints, []);
  const line = new DrawableLine({ x: 1, y: 2 }, { x: 3, y: 4 }, lineStyle());
  assert.deepEqual(line.source, { x: 1, y: 2 });
  assert.deepEqual(line.target, { x: 3, y: 4 });
  assert.deepEqual(line.extraPoints, []);
});

test("a control point bends the line through one extra point", () => {
  const line = new DrawableLine(left, right, lineStyle(), { x: 0, y: 50 });
  assert.equal(line.extraPoints.length, 1);
  assert.deepEqual(line.extraPoints[0], { x: 120, y: 60 });
});

test("updateLayout clears previous waypoints", () => {
  const style = lineStyle("layout middle");
  const line = makeLine({ x: 0, y: 0, width: 40, height: 20 }, { x: 10, y: 200, width: 40, height: 20 }, style);
  assert.ok(line.extraPoints.length > 0, "the middle layout routes around the boxes");
  const before = line.extraPoints;

  line.updateLayout(left, right);
  assert.equal(line.extraPoints.length, before.length, "waypoints are rebuilt, not appended to");
  assert.notDeepEqual(line.extraPoints, before);
});

test("middle layout routes around vertically stacked boxes along one vertical", () => {
  const line = makeLine({ x: 0, y: 0, width: 40, height: 20 }, { x: 10, y: 300, width: 40, height: 20 }, lineStyle("layout middle"));
  assert.equal(line.extraPoints.length, 2);
  assert.ok(Math.abs(line.extraPoints[0]!.x - line.extraPoints[1]!.x) < 0.001);
});

test("middle layout routes around horizontally placed boxes along one horizontal", () => {
  const line = makeLine({ x: 0, y: 0, width: 40, height: 20 }, { x: 300, y: 5, width: 40, height: 20 }, lineStyle("layout middle"));
  assert.equal(line.extraPoints.length, 2);
  assert.ok(Math.abs(line.extraPoints[0]!.y - line.extraPoints[1]!.y) < 0.001);
});

test("middle layout, diagonal boxes: one corner waypoint", () => {
  const line = makeLine({ x: 0, y: 0, width: 40, height: 20 }, { x: 200, y: 100, width: 40, height: 20 }, lineStyle("layout middle"));
  assert.deepEqual(line.source, { x: 40, y: 10 });
  assert.deepEqual(line.target, { x: 220, y: 100 });
  assert.deepEqual(line.extraPoints, [{ x: 220, y: 10 }]);
});

test("label is attached with a non-empty box and honours escaped newlines", () => {
  const line = makeLine(left, right);
  line.addLabel("edge label", new ElementImageProvider(host(), 1));
  assert.ok(line.label !== null && line.label.getBounds().width > 0);

  const double = makeLine(left, right);
  double.addLabel("one\\ntwo", new ElementImageProvider(host(), 1));
  const single = makeLine(left, right);
  single.addLabel("one", new ElementImageProvider(host(), 1));
  assert.ok(double.label!.getBounds().height > single.label!.getBounds().height, "a two-line label is taller");
});

test("bounds cover both endpoints", () => {
  const line = makeLine(left, right);
  const b = line.getBounds();
  assert.ok(b.x <= Math.min(line.source.x, line.target.x) + 0.001);
  assert.ok(b.x + b.width >= Math.max(line.source.x, line.target.x) - 0.001);
});

test("arrow display shortens the path and widens the bounds", () => {
  const plain = new DrawableLine({ x: 0, y: 0 }, { x: 100, y: 0 }, lineStyle());
  const arrowed = new DrawableLine({ x: 0, y: 0 }, { x: 100, y: 0 }, lineStyle("display arrow"));
  const { toPt, drawArrow } = arrowed.buildPath({ x: 0, y: 0 });
  assert.ok(drawArrow);
  assert.deepEqual(toPt, { x: 95, y: 0 });
  assert.ok(arrowed.getBounds().height > plain.getBounds().height);
  assert.equal(arrow({ x: 0, y: 0 }, { x: 3, y: 0 }, 0, 10, 10), null, "too short for an arrow");
});

test("identical boxes do not produce invalid coordinates", () => {
  const box: Rect = { x: 10, y: 10, width: 40, height: 20 };
  const line = makeLine(box, box);
  for (const v of [line.source.x, line.source.y, line.target.x, line.target.y]) {
    assert.ok(Number.isFinite(v));
  }
});

// MARK: image provider

// 1x1 transparent PNG.
const png = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==";

test("image provider: unknown name, declared image and broken data", async () => {
  const item = host();
  item.properties.append(newCommand("image", newIdent("logo"), newIdent(png)));
  const decodeImage = await loadDecoder(element(item));
  assert.equal(new ElementImageProvider(item, 1, decodeImage).resolveImage("missing"), null);
  const image = new ElementImageProvider(item, 1, decodeImage).resolveImage("logo");
  assert.notEqual(image, null, "a base64 image declared on the element must resolve");
  assert.deepEqual(image!.size, { width: 1, height: 1 });

  const broken = host();
  broken.properties.append(newCommand("image", newIdent("broken"), newIdent("not-base64!!")));
  assert.equal(new ElementImageProvider(broken, 1, await loadDecoder(element(broken))).resolveImage("broken"), null);
});

test("image provider scales by the @(name|WxH) style", async () => {
  const item = host();
  item.properties.append(newCommand("image", newIdent("logo"), newIdent(png)));
  const r = new ElementImageProvider(item, 1, await loadDecoder(element(item))).resolveImagePath("logo|20x10");
  assert.deepEqual(r!.rect, { x: 0, y: 0, width: 20, height: 10 });
});

// MARK: primitives

test("round box bounds cover the rect plus its stroke; setPath moves it", () => {
  const bounds: Rect = { x: 1, y: 2, width: 30, height: 40 };
  const box = new RoundBox(bounds, new DrawableItemStyle(false), true);
  assert.ok(containsRect(box.getBounds(), bounds));
  assert.ok(Math.abs(box.getBounds().width - bounds.width) < 1);
  assert.notEqual(box.path, null);

  const moved: Rect = { x: 5, y: 6, width: 20, height: 20 };
  box.setPath(moved);
  assert.ok(containsRect(box.getBounds(), moved));
  assert.ok(Math.abs(midX(box.getBounds()) - midX(moved)) < 0.001);
});

test("round box path starts at the top middle and honours corner radii", () => {
  const style = new DrawableItemStyle(false);
  style.parseStyle(parse("corner-radius 2 4 6 8").tree, new Map());
  const box = new RoundBox({ x: 0, y: 0, width: 50, height: 50 }, style, true);
  assert.deepEqual(box.path!.boundingBox, { x: 0, y: 0, width: 50, height: 50 });
});

test("shadow grows the bounds on the shadow side", () => {
  const style = new DrawableItemStyle(false);
  style.parseStyle(parse("shadow 3 -3 4").tree, new Map());
  const box = new RoundBox({ x: 0, y: 0, width: 50, height: 50 }, style, true);
  const b = box.getBounds();
  assert.ok(b.width > 50 && b.height > 50);
  assert.ok(b.y < 0, "negative y offset extends below the box");
});

test("empty box and circle box keep their bounds", () => {
  const bounds: Rect = { x: 3, y: 4, width: 15, height: 25 };
  const empty = new EmptyBox(bounds, new DrawableStyle(false));
  assert.deepEqual(empty.getBounds(), bounds);
  const moved: Rect = { x: 0, y: 0, width: 1, height: 1 };
  empty.setPath(moved);
  assert.deepEqual(empty.getBounds(), moved);

  const circleBounds: Rect = { x: 0, y: 0, width: 40, height: 40 };
  assert.deepEqual(new CircleBox(circleBounds, new DrawableItemStyle(false), true).getBounds(), circleBounds);
});

test("text box reports its frame, visits itself, can be re-framed", () => {
  const text = AttributedString.plain("hello", systemFont(12));
  const box = new TextBox(text, { x: 1, y: 1, width: 50, height: 20 });
  assert.equal(box.getBounds().width, 50);
  assert.ok(box.isVisible() && box.isSelectable());
  box.setFrame({ x: 0, y: 0, width: 80, height: 30 });
  assert.equal(box.getBounds().width, 80);

  let visited = 0;
  box.traverse(() => {
    visited++;
    return true;
  });
  assert.equal(visited, 1);
});

test("item drawable defaults to selectable; background layer is not", () => {
  const d = new ItemDrawable();
  assert.ok(d.isSelectable());
  d.layer = "Background";
  assert.ok(!d.isSelectable());
});

test("container reports the union of children; empty container has no children", () => {
  const a = new EmptyBox({ x: 0, y: 0, width: 10, height: 10 }, new DrawableStyle(false));
  const b = new EmptyBox({ x: 100, y: 100, width: 10, height: 10 }, new DrawableStyle(false));
  const bounds = new DrawableContainer([a, b]).getBounds();
  assert.deepEqual(bounds, { x: 0, y: 0, width: 110, height: 110 });
  assert.equal(new DrawableContainer([]).children, null);
});
