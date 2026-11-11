// Scene context (spec 4, 5c): `context <key> <value>` gives a key to the node and its subtree, read as
// ctx.<key> in an expression. `?=` is the default that yields to a key inherited from above.
import assert from "node:assert/strict";
import { test } from "node:test";
import { readTenn } from "@tenniarb/core";
import type { Element } from "@tenniarb/core";
import { buildTennScene, evalCode } from "../src/scene-tenn.ts";
import type { TennScene } from "../src/scene-tenn.ts";
import type { Canvas2D } from "../src/canvas-types.ts";
import "./helpers.ts";

function scene(text: string, name = "Screen"): { scene: TennScene; element: Element } {
  const model = readTenn(text);
  assert.ok(model !== null, "the document parses");
  const element = model.elements.find((e) => e.name === name);
  assert.ok(element !== undefined, `element ${name}`);
  return { scene: buildTennScene(element), element };
}

const box = (s: TennScene, path: string): TennScene["boxes"][number] => {
  const b = s.boxes.find((it) => it.path === path);
  assert.ok(b !== undefined, `box ${path}: ${s.boxes.map((it) => it.path).join(", ")}`);
  return b;
};

test("a context declaration is read as ctx.key in its subtree", () => {
  const { scene: s } = scene(`element "Screen" {
    item row { auto-layout row; context theme dark
        item A { width \${ctx.theme === "dark" ? 30 : 10}; height 10 }
    }
}`);
  assert.deepEqual(s.diagnostics, []);
  assert.equal(box(s, "row.A").width, 30);
});

test("a context value is an expression in the scope it is written in", () => {
  const { scene: s } = scene(`element "Screen" {
    item row { auto-layout row; context base 4
        item inner { auto-layout column; context gap \${ctx.base * 3}
            item A { width \${ctx.gap}; height 10 }
        }
    }
}`);
  assert.deepEqual(s.diagnostics, []);
  assert.equal(box(s, "row.inner.A").width, 12);
});

test("the nearest declaration wins, the outer one still holds beside it", () => {
  const { scene: s } = scene(`element "Screen" {
    item outer { auto-layout column; context tone 10
        item inner { auto-layout column; context tone 20
            item A { width \${ctx.tone}; height 10 }
        }
        item B { width \${ctx.tone}; height 10 }
    }
}`);
  assert.equal(box(s, "outer.inner.A").width, 20);
  assert.equal(box(s, "outer.B").width, 10);
});

test("?= is the default: it yields to the context of the place the use is written", () => {
  const { scene: s } = scene(`element "Screen" {
    item Card {
        context size ?= 20
        auto-layout column
        item A { width \${ctx.size}; height 10 }
    }
    item row { auto-layout row
        context size 40
        use Card as big
    }
}`);
  assert.deepEqual(s.diagnostics, []);
  assert.equal(box(s, "row.big.A").width, 40);
});

test("?= keeps its default where nothing above declares the key", () => {
  const { scene: s } = scene(`element "Screen" {
    item Card {
        context size ?= 20
        auto-layout column
        item A { width \${ctx.size}; height 10 }
    }
    item row { auto-layout row; use Card as plain }
}`);
  assert.equal(box(s, "row.plain.A").width, 20);
});

test("a context crosses the use: the component sees the context of the use site", () => {
  const { scene: s } = scene(`element "Screen" {
    context base 40
    item Card { auto-layout column; item A { width \${ctx.base}; height 10 } }
    item row { auto-layout row; use Card as card }
}`);
  assert.deepEqual(s.diagnostics, []);
  assert.equal(box(s, "row.card.A").width, 40);
});

test("the block of a use is stronger than the definition root", () => {
  const { scene: s } = scene(`element "Screen" {
    context tone 10
    item Card { auto-layout column; context tone 20; item A { width \${ctx.tone}; height 10 } }
    item row { auto-layout row; use Card as card { context tone 30 } }
}`);
  assert.deepEqual(s.diagnostics, []);
  assert.equal(box(s, "row.card.A").width, 30);
});

test("?= in the block of a use does not overrule an inherited key", () => {
  const { scene: s } = scene(`element "Screen" {
    context tone 10
    item Card { auto-layout column; item A { width \${ctx.tone}; height 10 } }
    item row { auto-layout row; use Card as card { context tone ?= 30 } }
}`);
  assert.equal(box(s, "row.card.A").width, 10);
});

test("the content of a slot inherits the context of the component it is placed in", () => {
  const { scene: s } = scene(`element "Kit" {
    item Card { auto-layout column; context width 50; slot body }
}
element "Screen" {
    item row { auto-layout row
        use Kit.Card as card { slot body { item Inner { width \${ctx.width}; height 10 } } }
    }
}`);
  assert.deepEqual(s.diagnostics, []);
  assert.equal(box(s, "row.card.Inner").width, 50);
});

test("a context of a component is set from its parameters", () => {
  const { scene: s } = scene(`element "Screen" {
    item Card {
        export let accent = 10
        context width \${accent}
        auto-layout column
        item A { width \${ctx.width}; height 10 }
    }
    item row { auto-layout row; use Card as card { accent 70 } }
}`);
  assert.deepEqual(s.diagnostics, []);
  assert.equal(box(s, "row.card.A").width, 70);
});

test("a context of a use block is read in the parameters of the instance", () => {
  const { scene: s } = scene(`element "Kit" {
    item Card {
        export let base = 1
        auto-layout column
        item A { width \${ctx.width}; height 10 }
    }
}
element "Screen" {
    item row { auto-layout row; use Kit.Card as card { base 30; context width \${base} } }
}`);
  assert.deepEqual(s.diagnostics, []);
  assert.equal(box(s, "row.card.A").width, 30);
});

test("a missing key is a diagnostic, undefined and NaN are not a value", () => {
  const { scene: s } = scene(`element "Screen" {
    item A { auto-layout row; width \${ctx.missing}; height 10 }
}`);
  assert.ok(s.diagnostics.length > 0);
  assert.ok(s.diagnostics.every((d) => d.warn));
  assert.ok(s.diagnostics.some((d) => /not evaluated/.test(d.message)));
  // The width falls back to the text, as with any expression that is not evaluated.
  const plain = scene(`element "Screen" { item A { auto-layout row; height 10 } }`).scene;
  assert.equal(box(s, "A").width, box(plain, "A").width);

  assert.equal(evalCode("undefined", null), null);
  assert.equal(evalCode("NaN", null), null);
  assert.equal(evalCode("ctx.missing", null, {}), null);
  assert.equal(evalCode("digits[5]", { digits: "12" }), null);
  assert.equal(evalCode("ctx.radius * 2", null, { radius: "4" }), "8");
});

test("ctx is reserved: a parameter of that name is a warning", () => {
  const { scene: s } = scene(`element "Screen" {
    item Card {
        export let ctx = 10
        auto-layout column
        item A { width 10; height 10 }
    }
}`);
  assert.ok(s.diagnostics.some((d) => d.warn && d.message.includes("reserved")), s.diagnostics.map((d) => d.message).join("; "));
});

test("a context in styles is a property of the style, not of the scope", () => {
  const { scene: s } = scene(`element "Screen" {
    styles { dark { context tone 30 } }
    item Card { auto-layout column; use-style dark; item A { width \${ctx.tone}; height 10 } }
}`);
  assert.ok(s.diagnostics.some((d) => /not evaluated/.test(d.message)));
  const plain = scene(`element "Screen" { item Card { auto-layout column; item A { height 10 } } }`).scene;
  assert.equal(box(s, "Card.A").width, box(plain, "Card.A").width);
});

test("a context in an at block is a property of the node, not of the scope", () => {
  const { scene: s } = scene(`element "Kit" {
    item Card { auto-layout column; context tone 10; item Box { item Inner { width \${ctx.tone}; height 10 } } }
}
element "Screen" {
    item row { auto-layout row; use Kit.Card as card { at Box { context tone 30 } } }
}`);
  assert.equal(box(s, "row.card.Box.Inner").width, 10);
});

test("a classic diagram keeps context as an ordinary property", () => {
  const { scene: s } = scene(`element "Screen" { item A { pos 0 0; context theme dark; width 10; height 20 } }`);
  assert.deepEqual(s.diagnostics, []);
  assert.deepEqual([box(s, "A").width, box(s, "A").height], [10, 20]);
});

test("a context on the root of the file reaches an element built on its own", () => {
  const { scene: s } = scene(`context page 40
element "Screen" {
    item A { auto-layout row; width \${ctx.page}; height 10 }
}`);
  assert.deepEqual(s.diagnostics, []);
  assert.equal(box(s, "A").width, 40);
});

test("?= does not overrule a key the root of the file declares", () => {
  const { scene: s } = scene(`context tone 10
element "Screen" {
    context tone ?= 20
    item A { auto-layout row; width \${ctx.tone}; height 10 }
}`);
  assert.equal(box(s, "A").width, 10);
});

test("a classic diagram evaluates a property that names ctx in the scene, not in the model", () => {
  const { scene: s } = scene(`context tone dark
element "Screen" {
    context ink #c1c9d6
    item A {
        pos 0 0
        width \${ctx.tone === "dark" ? 30 : 10}
        height 20
        title \${ctx.tone}
        text-color \${ctx.ink}
    }
}`);
  assert.deepEqual(s.diagnostics, []);
  assert.equal(box(s, "A").width, 30);
  assert.deepEqual(texts(s), ["dark"]);
});

/** The strings the scene paints: the title a classic diagram used to take from the model shows up here. */
function texts(s: TennScene): string[] {
  const out: string[] = [];
  const ctx = {
    save() {},
    restore() {},
    translate() {},
    scale() {},
    rotate() {},
    beginPath() {},
    moveTo() {},
    lineTo() {},
    arcTo() {},
    quadraticCurveTo() {},
    closePath() {},
    rect() {},
    ellipse() {},
    clip() {},
    fill() {},
    stroke() {},
    fillRect() {},
    fillStyle: "",
    strokeStyle: "",
    lineWidth: 1,
    lineDashOffset: 0,
    setLineDash() {},
    globalAlpha: 1,
    shadowColor: "",
    shadowBlur: 0,
    shadowOffsetX: 0,
    shadowOffsetY: 0,
    font: "",
    textBaseline: "",
    fillText: (text: string) => out.push(text),
    measureText: (text: string) => ({ width: [...text].length * 10 }),
    drawImage() {},
  };
  s.draw(ctx as unknown as Canvas2D);
  return out;
}
