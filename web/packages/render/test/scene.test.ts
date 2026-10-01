// Port of the non-AppKit parts of SceneLayoutTests.swift (DrawableScene) and SceneRenderTests.swift.
import assert from "node:assert/strict";
import { test } from "node:test";
import { DiagramItem, Element, ElementModel, ExecutionContext, GridLayout, LayoutContext, LinkItem, ElementModelStore } from "@tenniarb/core";
import {
  AttributedString,
  DrawableLine,
  DrawableScene,
  ElementImageProvider,
  RoundBox,
  TextBox,
  calculateSize,
  createExecutionContext,
  getTokens,
  layoutLines,
  renderElement,
  systemFont,
  toAttributedString,
} from "../src/index.ts";
import { colorBlack } from "../src/index.ts";
import { createCanvas } from "@napi-rs/canvas";
import { loadDecoder, newCanvas, parseDiagram } from "./helpers.ts";

function makeDiagram(count: number, linked = false): { diagram: Element; items: DiagramItem[] } {
  const model = new ElementModel();
  const diagram = new Element("Diagram");
  model.add(diagram);

  const items: DiagramItem[] = [];
  for (let i = 0; i < count; i++) {
    const item = new DiagramItem("Item", `Item${i}`);
    item.x = i * 10;
    item.y = i * 10;
    diagram.add(item);
    items.push(item);
  }
  if (linked) {
    for (let i = 1; i < count; i++) {
      diagram.addSourceTarget(items[i - 1]!, items[i]!);
    }
  }
  return { diagram, items };
}

function makeScene(element: Element, darkMode = false, opts = {}): DrawableScene {
  const context = new ExecutionContext();
  context.setElement(element);
  return new DrawableScene(element, darkMode, context, opts);
}

const layoutAndDraw = (scene: DrawableScene) => {
  const bounds = scene.getBounds();
  scene.layout(bounds, bounds);
  const { ctx, canvas2d } = newCanvas();
  scene.draw(canvas2d);
  return ctx;
};

// MARK: DrawableScene (SceneLayoutTests)

test("scene builds a drawable per item and covers all of them", () => {
  const { diagram, items } = makeDiagram(3);
  const scene = makeScene(diagram);
  for (const item of items) {
    assert.ok(scene.drawables.get(item), `${item.name} must have a drawable`);
  }
  assert.ok(scene.getBounds().width > 0 && scene.getBounds().height > 0);
});

test("empty scene has empty bounds", () => {
  assert.equal(makeScene(makeDiagram(0).diagram).getBounds().width, 0);
});

test("traverse visits self and direct children only, stops when the visitor says so", () => {
  const scene = makeScene(makeDiagram(3, true).diagram);
  let visited = 0;
  scene.traverse(() => {
    visited++;
    return true;
  });
  assert.equal(visited, 2);

  visited = 0;
  scene.traverse(() => {
    visited++;
    return false;
  });
  assert.equal(visited, 1);
});

test("link drawable follows its endpoints", () => {
  const { diagram, items } = makeDiagram(2, true);
  const scene = makeScene(diagram);
  const bounds = scene.getBounds();
  scene.layout(bounds, bounds);

  const link = diagram.items.find((i) => i instanceof LinkItem)!;
  const before = scene.drawables.get(link)!.getBounds();

  items[1]!.x += 500;
  const moved = makeScene(diagram);
  assert.notDeepEqual(moved.drawables.get(link)!.getBounds(), before, "moving an endpoint must move the link");
});

test("scene implements LayoutScene for the core layouts", () => {
  const { diagram, items } = makeDiagram(4);
  const store = new ElementModelStore(diagram.model!);
  const scene = makeScene(diagram);
  const context = new LayoutContext(diagram, scene, store, { x: 0, y: 0, width: 800, height: 600 });

  assert.ok(context.getBounds(items[0]!).width > 0, "a drawn item has a real size");
  assert.deepEqual(context.getBounds(new DiagramItem("Item", "NotInScene")), { x: 0, y: 0, width: 0, height: 0 });
  assert.equal(new GridLayout().apply(context, true).length, items.length);
});

// MARK: rendering (SceneRenderTests)

const source = `
element Root {
    item First {
        pos 10 20
        color red
        font-size 20
    }
    item Second {
        pos 200 40
        display text
        text-color blue
    }
    item Circle {
        pos 100 200
        display circle
    }
    link First Second {
    }
}`;
const diagram = () => parseDiagram(source);

test("scene builds drawables for every item and lines for links", () => {
  const element = diagram();
  const scene = makeScene(element);
  for (const item of element.items) {
    assert.ok(scene.drawables.get(item), `${item.name} needs a drawable`);
  }
  assert.ok(scene.drawables.get(element.items.find((i) => i.kind === "Link")!) instanceof DrawableLine);
});

test("drawing the whole scene succeeds, also in dark mode and the drawBox pass", () => {
  const scene = makeScene(diagram());
  layoutAndDraw(scene);
  assert.ok(scene.getBounds().width > 0);

  const dark = makeScene(diagram(), true);
  layoutAndDraw(dark);
  assert.ok(dark.darkMode);

  const bounds = scene.getBounds();
  scene.layout(bounds, bounds);
  scene.drawBox(newCanvas().canvas2d);
});

test("drawing produces a non-blank image", () => {
  const scene = makeScene(diagram());
  const bounds = scene.getBounds();
  scene.offset = { x: -bounds.x + 10, y: -bounds.y + 10 };
  scene.layout(bounds, bounds);
  const { ctx, canvas2d } = newCanvas();
  scene.draw(canvas2d);
  const data = ctx.getImageData(0, 0, 400, 400).data;
  let painted = 0;
  for (let i = 3; i < data.length; i += 4) {
    if (data[i] !== 0) {
      painted++;
    }
  }
  assert.ok(painted > 0, "something must actually be drawn");
});

test("shadow and border styles render without trouble", () => {
  layoutAndDraw(
    makeScene(
      parseDiagram(`element Root {
    item Boxed {
        pos 0 0
        shadow 2 2 4 gray
        border-color green
        line-width 3
        corner-radius 6
    }
}`),
    ),
  );
});

function drawableOf(src: string) {
  const element = parseDiagram(src);
  const scene = makeScene(element);
  return { element, scene, drawable: scene.drawables.get(element.items[0]!)! };
}

test("item with explicit size renders at that size", () => {
  const { drawable } = drawableOf("element Root {\n item Wide {\n pos 0 0\n width 300\n height 120\n }\n}");
  assert.ok(Math.abs(drawable.getBounds().width - 300) < 2);
  assert.ok(Math.abs(drawable.getBounds().height - 120) < 2);
});

test("background layer item is not selectable", () => {
  const { drawable } = drawableOf("element Root {\n item Back {\n pos 0 0\n layer background\n }\n}");
  assert.ok(!drawable.isSelectable());
});

test("markdown body is rendered", () => {
  const { scene, drawable } = drawableOf("element Root {\n item Doc {\n pos 0 0\n body %{**bold** and _italic_}\n }\n}");
  assert.ok(drawable.getBounds().width > 0);
  layoutAndDraw(scene);
});

test("item geometry: y-up, top-left at (x, y - height)", () => {
  const { drawable } = drawableOf("element Root {\n item A {\n pos 10 20\n width 100\n height 40\n }\n}");
  const b = drawable.getSelectorBounds();
  assert.deepEqual(b, { x: 10, y: -20, width: 100, height: 40 });
});

test("scene without children and scene restricted to a subset of items", () => {
  const element = diagram();
  const context = new ExecutionContext();
  context.setElement(element);
  assert.equal(new DrawableScene(element, false, context, { buildChildren: false }).getBounds().width, 0);

  const subset = element.items.filter((i) => i.kind === "Item").slice(0, 1);
  const scene = new DrawableScene(element, false, context, { items: subset });
  assert.ok(scene.drawables.get(subset[0]!));
  assert.equal(scene.drawables.size, 1);
});

test("styles block and use-style reach the item", () => {
  const element = parseDiagram(`element Root {
    styles {
        item {
            font-size 30
        }
        big {
            width 200
            height 80
        }
    }
    item A {
        pos 0 0
    }
    item B {
        pos 0 100
        use-style big
    }
}`);
  const scene = makeScene(element);
  assert.equal(scene.sceneStyle.defaultItemStyle.fontSize, 30);
  assert.ok(Math.abs(scene.drawables.get(element.items[0]!)!.getBounds().height - 80) > 1, "A does not use the style");
  assert.ok(Math.abs(scene.drawables.get(element.items[1]!)!.getBounds().width - 200) < 2);
});

test("inherit takes properties from a same-named item of the parent element", () => {
  const root = parseDiagram(`element Outer {
    item Base {
        color red
    }
    element Inner {
        item Child {
            inherit "../Base"
        }
    }
}`);
  const inner = root.elements[0]!;
  const scene = makeScene(inner);
  assert.deepEqual((scene.drawables.get(inner.items[0]!) as RoundBox).style.color, { r: 1, g: 0, b: 0, a: 1 });
});

// MARK: ExecutionContext bridge

function titleOf(scene: DrawableScene, item: DiagramItem): string {
  const box = scene.drawables.get(item) as RoundBox;
  return (box.children![0] as TextBox).attrStr.string;
}

test("width/height expressions see the measured drawable size", () => {
  const element = parseDiagram(`element Root {
    item A {
        pos 0 0
        width 120
        height 50
        title \${width + "x" + height}
    }
}`);
  const context = createExecutionContext();
  context.setElement(element);
  assert.equal(titleOf(new DrawableScene(element, false, context), element.items[0]!), "120x50");
});

test("utils.textSize uses the real text measurer", () => {
  const element = parseDiagram(`element Root {
    item A {
        pos 0 0
        title \${"" + utils.textSize("Hello", 18)[0]}
    }
}`);
  const context = createExecutionContext();
  context.setElement(element);
  const width = Number(titleOf(new DrawableScene(element, false, context), element.items[0]!));
  const measured = calculateSize(AttributedString.plain("Hello", systemFont(18))).width;
  assert.equal(width, Math.ceil(measured) + 6);
});

test("evaluate: false renders token text", () => {
  const element = parseDiagram(`element Root {
    item A {
        pos 0 0
        width \${100 + 50}
    }
}`);
  const context = createExecutionContext({ evaluate: false });
  context.setElement(element);
  const scene = new DrawableScene(element, false, context);
  assert.ok(scene.drawables.get(element.items[0]!)!.getBounds().width < 100);
});

// MARK: text (SceneTextTests)

const plain = (s: string) => AttributedString.plain(s, systemFont(12));

test("calculateSize grows with the text, empty text is not negative", () => {
  assert.ok(calculateSize(plain("a much longer piece of text")).width > calculateSize(plain("a")).width);
  const empty = calculateSize(plain(""));
  assert.deepEqual(empty, { width: 0, height: 0 });
});

test("markdown to attributed string keeps the text", () => {
  const attributed = toAttributedString(
    getTokens("*bold* text"),
    systemFont(12),
    colorBlack,
    { x: 0, y: 0 },
    new ElementImageProvider(new DiagramItem("Item", "Host"), 1),
    [],
  );
  assert.ok(attributed.string.includes("bold") && attributed.string.includes("text"));
  assert.ok(attributed.runs.some((r) => r.font.bold && r.text === "bold"));
  assert.ok(attributed.length > 0);
});

test("multiline text is taller than a single line; trailing newline adds no line", () => {
  assert.ok(calculateSize(plain("one\ntwo")).height > calculateSize(plain("one")).height);
  assert.equal(calculateSize(plain("one\n")).height, calculateSize(plain("one")).height);
});

test("wrapping happens at the frame width, not in calculateSize", () => {
  const text = plain("alpha beta gamma delta epsilon");
  assert.equal(layoutLines(text, null).length, 1);
  const width = calculateSize(text).width;
  const lines = layoutLines(text, width / 2);
  assert.ok(lines.length >= 2);
  for (const l of lines) {
    assert.ok(l.width <= width / 2 + 0.001);
  }
  assert.equal(lines.map((l) => l.segments.map((s) => s.text).join("")).join(" ").replace(/\s+/g, " ").trim(), "alpha beta gamma delta epsilon");
});

test("a word wider than the frame breaks between characters", () => {
  const lines = layoutLines(plain("Supercalifragilistic"), 40);
  assert.ok(lines.length > 1);
  for (const l of lines) {
    assert.ok(l.width <= 40 + 0.001);
  }
});

test("bullets indent continuation lines and widen the box", () => {
  const shift = { x: 0, y: 0 };
  const attributed = toAttributedString(getTokens("* item text that wraps"), systemFont(12), colorBlack, shift, new ElementImageProvider(new DiagramItem("Item", "H"), 1), []);
  assert.equal(shift.x, 5);
  const lines = layoutLines(attributed, 60);
  assert.ok(lines.length > 1);
  assert.equal(lines[1]!.paragraph.headIndent, 5);
});

// MARK: renderElement and orientation

test("renderElement flips y so that larger y is higher on the canvas", () => {
  const element = parseDiagram(`element Root {
    item Top {
        pos 0 100
        width 40
        height 20
        color red
        border-color red
    }
    item Low {
        pos 0 0
        width 40
        height 20
        color blue
        border-color blue
    }
}`);
  const { ctx, canvas2d } = newCanvas(200, 200);
  const area = renderElement(canvas2d, element);
  assert.ok(area.width > 40 && area.height > 100);
  const px = (x: number, y: number) => Array.from(ctx.getImageData(x, y, 1, 1).data);
  // Top item is in the upper part of the image (small canvas y), red; Low in the lower part, blue.
  const top = px(35, 25);
  const low = px(35, Math.floor(area.height) - 25);
  assert.ok(top[0]! > 200 && top[2]! < 100, `top pixel ${top}`);
  assert.ok(low[2]! > 200 && low[0]! < 100, `low pixel ${low}`);
});

test("text is upright and top-aligned: glyph pixels only in the upper part of a tall box", () => {
  const element = parseDiagram(`element Root {
    item T {
        pos 0 0
        width 100
        height 100
        layout top left
        title "WWWW"
        color white
        border-color white
    }
}`);
  const { ctx, canvas2d } = newCanvas(130, 130);
  renderElement(canvas2d, element, { background: "#ffffff" });
  const dark = (y0: number, y1: number) => {
    const data = ctx.getImageData(15, y0, 100, y1 - y0).data;
    let n = 0;
    for (let i = 0; i < data.length; i += 4) {
      if (data[i]! < 128) {
        n++;
      }
    }
    return n;
  };
  assert.ok(dark(15, 45) > 50, "text near the top");
  assert.equal(dark(75, 115), 0, "nothing near the bottom");
});

test("scene size without drawing matches the drawn rect", async () => {
  const { getSceneSize } = await import("../src/index.ts");
  const element = diagram();
  const size = getSceneSize(element);
  const { canvas2d } = newCanvas(10, 10);
  const area = renderElement(canvas2d, element);
  assert.deepEqual(size, { width: area.width, height: area.height });
});

// MARK: images

test("an inline image grows the item and is drawn upright", async () => {
  const src = createCanvas(20, 10);
  const sctx = src.getContext("2d");
  sctx.fillStyle = "#ff0000";
  sctx.fillRect(0, 0, 20, 5); // red top half, blue bottom half
  sctx.fillStyle = "#0000ff";
  sctx.fillRect(0, 5, 20, 5);
  const b64 = src.toBuffer("image/png").toString("base64");
  const element = parseDiagram(`element Root {
    item Pic {
        pos 0 0
        image "pic" @(${b64})
        title %{@(pic|40x20)}
        display text
    }
}`);
  const decodeImage = await loadDecoder(element);
  const withoutDecoder = makeScene(element).drawables.get(element.items[0]!)!.getBounds();
  const context = createExecutionContext();
  context.setElement(element);
  const scene = new DrawableScene(element, false, context, { decodeImage });
  const bounds = scene.drawables.get(element.items[0]!)!.getBounds();
  assert.ok(bounds.width >= 40 && bounds.height >= 20, `${JSON.stringify(bounds)}`);
  assert.ok(bounds.width > withoutDecoder.width);

  const { ctx, canvas2d } = newCanvas(120, 80);
  renderElement(canvas2d, element, { decodeImage });
  const find = (r: number, b: number) => {
    const d = ctx.getImageData(0, 0, 120, 80).data;
    let minY = Infinity;
    for (let i = 0; i < d.length; i += 4) {
      if (Math.abs(d[i]! - r) < 40 && Math.abs(d[i + 2]! - b) < 40 && d[i + 3]! === 255 && Math.abs(d[i + 1]!) < 40) {
        minY = Math.min(minY, Math.floor(i / 4 / 120));
      }
    }
    return minY;
  };
  assert.ok(find(255, 0) < Infinity && find(0, 255) < Infinity, "both halves are drawn");
  assert.ok(find(255, 0) < find(0, 255), "red half above blue half");
});
