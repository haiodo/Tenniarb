// The body of an item (docs/spec/tenn-scene.md 5d, go/scene/body.go): the markdown under its title,
// laid out and drawn with the numbers go/scene uses. The measurer here is gfx's estimate, so a rune
// is 0.6 em wide, a line box round(0.8 + 0.25 em) and the title line of an 18 point item 19.
import assert from "node:assert/strict";
import { test } from "node:test";
import { readTenn } from "@tenniarb/core";
import type { Element } from "@tenniarb/core";
import { buildTennScene, isSceneElement } from "../src/scene-tenn.ts";
import type { TennScene } from "../src/scene-tenn.ts";
import type { Canvas2D } from "../src/canvas-types.ts";

interface Painted {
  text: string;
  font: string;
  fill: string;
  x: number;
  y: number;
}

interface Rect {
  x: number;
  y: number;
  w: number;
  h: number;
  fill: string;
}

/** A Canvas2D that records the text and the fillRect calls of a body: what a backend paints. */
function record(): { ctx: Canvas2D; texts: Painted[]; rects: Rect[] } {
  const texts: Painted[] = [];
  const rects: Rect[] = [];
  const state = { fill: "rgb(0,0,0)", font: "" };
  const stack: (typeof state)[] = [];
  const ctx: Canvas2D = {
    save: () => stack.push({ ...state }),
    restore: () => Object.assign(state, stack.pop() ?? state),
    translate: () => {},
    scale: () => {},
    beginPath: () => {},
    moveTo: () => {},
    lineTo: () => {},
    arcTo: () => {},
    quadraticCurveTo: () => {},
    closePath: () => {},
    rect: () => {},
    ellipse: () => {},
    clip: () => {},
    fill: () => {},
    stroke: () => {},
    fillRect: (x, y, w, h) => rects.push({ x, y, w, h, fill: state.fill }),
    fillText: (text, x, y) => texts.push({ text, font: state.font, fill: state.fill, x, y }),
    measureText: (text: string) => ({ width: [...text].length * 10 }),
    drawImage: () => {},
    setLineDash: () => {},
    get fillStyle() {
      return state.fill;
    },
    set fillStyle(v: string | object) {
      state.fill = String(v);
    },
    strokeStyle: "rgb(0,0,0)",
    lineWidth: 1,
    lineDashOffset: 0,
    shadowColor: "",
    shadowBlur: 0,
    shadowOffsetX: 0,
    shadowOffsetY: 0,
    get font() {
      return state.font;
    },
    set font(v: string) {
      state.font = v;
    },
    textBaseline: "alphabetic",
  };
  return { ctx, texts, rects };
}

function scene(text: string, name = "E"): TennScene {
  const model = readTenn(text);
  assert.ok(model !== null, "the document parses");
  const element: Element | undefined = model.elements.find((e) => e.name === name);
  assert.ok(element !== undefined, `element ${name}`);
  assert.ok(isSceneElement(element), `${name} is a scene`);
  return buildTennScene(element);
}

const painted = (s: TennScene): ReturnType<typeof record> => {
  const out = record();
  s.draw(out.ctx);
  return out;
};

const height = (s: TennScene, name: string): number => {
  const b = s.boxes.find((it) => it.name === name);
  assert.ok(b !== undefined, `box ${name}: ${s.boxes.map((it) => it.name).join(", ")}`);
  return b.height;
};

const width = (s: TennScene, name: string): number => {
  const b = s.boxes.find((it) => it.name === name);
  assert.ok(b !== undefined, `box ${name}`);
  return b.width;
};

test("body: a heading, a list, a code block, a quote and inline looks", () => {
  const s = scene(`element "E" {
    item "Doc" {
        auto-layout column
        width 300
        body %{
            # Head

            A **bold** and \`code\` [link](http://x).

            - one
            - two

            > quoted

            \`\`\`
            x := 1
            \`\`\`
        }
    }
}`);
  const { texts, rects } = painted(s);
  const of = (t: string): Painted => {
    const hit = texts.find((it) => it.text === t);
    assert.ok(hit !== undefined, `no "${t}" in ${JSON.stringify(texts.map((it) => it.text))}`);
    return hit;
  };
  // A link is its label, not the URL.
  assert.ok(!texts.some((it) => it.text.includes("http://x")));
  // The heading is the body of 16 + 6, bold.
  assert.match(of("Head").font, /700 22px/);
  assert.match(of("bold").font, /700 16px/);
  assert.match(of("code").font, /16px Menlo/);
  // A bullet keeps its marker at 16, its text merging with it in the same font.
  assert.equal(of("• one").font, "16px Helvetica, Arial, sans-serif");
  assert.equal(of("• one").x, 16);
  // The quote keeps its text at 12, and its bar is 3 wide.
  assert.equal(of("quoted").x, 12);
  const bar = rects.find((r) => r.w === 3);
  assert.ok(bar !== undefined, "the quote bar");
  assert.equal(bar.fill, "rgb(189,189,189)");
  // The code block is monospaced over a background across the item width.
  assert.match(of("x := 1").font, /16px Menlo/);
  const bg = rects.find((r) => r.fill === "rgb(238,238,238)" && r.w === 300);
  assert.ok(bg !== undefined, `the code background in ${JSON.stringify(rects)}`);
  assert.equal(bg.h, 17);
});

test("body: height of a title line plus the body lines and gaps", () => {
  // Layout context: no padding. Title 19, a body line of 16 points 17, a gap of 5 between blocks.
  const s = scene(`element "E" {
    item "Col" {
        auto-layout column
        item "A" { title "T"; width 100; body "aaaa bbbb" }
        item "B" { title "T"; width 50; body "aaaa bbbb" }
        item "G" { title "T"; width 1000; body %{
            one

            two
        } }
        item "L" { title "T"; width 1000; body %{
            - a
            - b
        } }
        item "H1" { title "T"; width 1000; body %{
            one

            # head
        } }
        item "H2" { title "T"; width 1000; body %{
            one

            ## head
        } }
    }
}`);
  assert.equal(height(s, "A"), 19 + 17);
  assert.equal(height(s, "B"), 19 + 2 * 17);
  assert.equal(height(s, "G"), 19 + 17 + 5 + 17);
  assert.equal(height(s, "L"), 19 + 2 * 17);
  // A heading adds 10 (h1, 6 + 6 points, so a line of 23) or 6 (h2, 21) to the gap of a block.
  assert.equal(height(s, "H1"), 19 + 17 + 5 + 10 + 23);
  assert.equal(height(s, "H2"), 19 + 17 + 5 + 6 + 21);
});

test("body: a wrapped bullet hangs at the width of its marker", () => {
  const s = scene(`element "E" {
    item "L" { auto-layout column; width 100; body "- word word word word" }
}`);
  const { texts } = painted(s);
  assert.deepEqual(
    texts.map((it) => [it.text, it.x]),
    [
      ["• word ", 16],
      ["word ", 16 + 2 * 9.6],
      ["word ", 16 + 2 * 9.6],
      ["word", 16 + 2 * 9.6],
    ],
  );
});

test("body: without a width the item grows to the widest line", () => {
  // 4 and 6 runes at 9.6 points, no padding in a layout context.
  const s = scene(`element "E" {
    item "Col" {
        auto-layout column
        item "S" { title "T"; body "aaaa" }
        item "L" { title "T"; body "aaaaaa" }
    }
}`);
  assert.equal(width(s, "S"), 39);
  assert.equal(width(s, "L"), 58);
});

test("body: of an auto-layout item it flows below the children and grows the card", () => {
  const s = scene(`element "E" {
    item "Card" {
        auto-layout column
        width 420
        padding 20
        gap 12
        item "T" { title "Head" }
        body %{
            one
        }
    }
}`);
  // Padding 20, the title line 19, the gap 12, the body line 17, padding 20.
  assert.equal(height(s, "Card"), 20 + 19 + 12 + 17 + 20);
  const { texts } = painted(s);
  const title = texts.find((it) => it.text === "Head");
  const body = texts.find((it) => it.text === "one");
  assert.ok(title !== undefined && body !== undefined, "title and body are drawn");
  assert.ok(body.y > title.y, `the body baseline ${body.y} is below the title ${title.y}`);
});

test("body: more text wraps and the card grows by every line", () => {
  const s = scene(`element "E" {
    item "Card" {
        auto-layout column
        width 80
        padding 10
        gap 10
        item "T" { title "Head" }
        body "one two three four five"
    }
}`);
  // The inner width 60 takes one word a line: 5 lines at 17, title 19, gap 10, padding 20.
  assert.equal(height(s, "Card"), 10 + 19 + 10 + 5 * 17 + 10);
});

test("body: the three forms of the classic command", () => {
  const s = scene(`element "E" {
    item "Col" {
        auto-layout column
        item "A" { body "one line" }
        item "B" { body %{ two lines } }
        item "C" { body { text "custom"; font-size 14; mono "Courier"; text-color #112233; line-height 20; gap 3 } }
    }
}`);
  assert.equal(s.boxes.length, 5, "the element, the column and three items");
  const { texts } = painted(s);
  const custom = texts.find((it) => it.text === "custom");
  assert.ok(custom !== undefined, "the text of the block form");
  assert.equal(custom.fill, "rgb(17,34,51)");
  assert.equal(custom.font, "14px Helvetica, Arial, sans-serif");
});

test("body: the closing % of a markdown literal leaves no percent", () => {
  const s = scene(`element "E" { item "D" { auto-layout column; body %{ hi there %} } }`);
  const { texts } = painted(s);
  assert.deepEqual(texts.map((it) => it.text), ["hi there"]);
});
