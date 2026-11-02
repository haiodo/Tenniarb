// Scene rendering (docs/spec/tenn-scene.md 4, 4.7a, 5b, 5c, 13): the use-style cascade, the parameter
// values of a use, quoted path segments, slots declared in a replace and the first frame of an animation.
// The expected numbers are go/scene's: the same layout size and the same colours as go/cmd/tenn-render.
import assert from "node:assert/strict";
import { test } from "node:test";
import { readTenn } from "@tenniarb/core";
import type { Element } from "@tenniarb/core";
import { buildTennScene, isSceneElement } from "../src/scene-tenn.ts";
import type { TennScene } from "../src/scene-tenn.ts";
import type { Canvas2D } from "../src/canvas-types.ts";
import { renderTenn } from "../src/node.ts";

/** A Canvas2D that records what is painted: fills, strokes, text and the alpha in force. */
function record(): { ctx: Canvas2D; fills: string[]; strokes: string[]; texts: { text: string; fill: string; font: string }[] } {
  const fills: string[] = [];
  const strokes: string[] = [];
  const texts: { text: string; fill: string; font: string }[] = [];
  const state = { fill: "rgb(0,0,0)", stroke: "rgb(0,0,0)", alpha: 1, font: "" };
  const stack: (typeof state)[] = [];
  const paint = (c: string): string => (state.alpha === 1 || !c.startsWith("rgb(") ? c : c.replace("rgb(", "rgba(").replace(")", `,${state.alpha})`));
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
    fill: () => fills.push(paint(state.fill)),
    stroke: () => strokes.push(paint(state.stroke)),
    fillRect: () => {},
    fillText: (text: string) => texts.push({ text, fill: paint(state.fill), font: state.font }),
    measureText: (text: string) => ({ width: [...text].length * 10 }),
    drawImage: () => {},
    setLineDash: () => {},
    get fillStyle() {
      return state.fill;
    },
    set fillStyle(v: string | object) {
      state.fill = String(v);
    },
    get strokeStyle() {
      return state.stroke;
    },
    set strokeStyle(v: string | object) {
      state.stroke = String(v);
    },
    get globalAlpha() {
      return state.alpha;
    },
    set globalAlpha(v: number) {
      state.alpha = v;
    },
    get font() {
      return state.font;
    },
    set font(v: string) {
      state.font = v;
    },
    lineWidth: 1,
    lineDashOffset: 0,
    shadowColor: "",
    shadowBlur: 0,
    shadowOffsetX: 0,
    shadowOffsetY: 0,
    textBaseline: "alphabetic",
  };
  return { ctx, fills, strokes, texts };
}

function scene(text: string, name = "Screen"): { scene: TennScene; element: Element } {
  const model = readTenn(text);
  assert.ok(model !== null, "the document parses");
  const element = model.elements.find((e) => e.name === name);
  assert.ok(element !== undefined, `element ${name}`);
  return { scene: buildTennScene(element), element };
}

const paint = (s: TennScene): ReturnType<typeof record> => {
  const out = record();
  s.draw(out.ctx);
  return out;
};

const box = (s: TennScene, name: string): { width: number; height: number } => {
  const b = s.boxes.find((it) => it.name === name);
  assert.ok(b !== undefined, `box ${name}: ${s.boxes.map((it) => it.name).join(", ")}`);
  return b;
};

const component = (styles: string, body: string): string => `element "Button" {
    kind Component
    export let title = "Button"
    export let kind = primary
    export let size = medium
    styles {
        primary { color #6b3ad3; text-color #ffffff }
        secondary { color #ffffff; border-color #1725471a; text-color #172547 }
        large { height 48; padding 0 16 }
        medium { height 40; padding 0 12 }
        ${styles}
        kind one-of primary secondary
        size one-of large medium
    }
${body}
}
`;

test("a scene element is told apart from a classic diagram", () => {
  const { element: classic } = scene(`element "Screen" { item A { pos 0 0; color red } }`);
  assert.equal(isSceneElement(classic), false);
  const { element: withAutoLayout } = scene(`element "Screen" { item A { auto-layout row } }`);
  assert.equal(isSceneElement(withAutoLayout), true);
});

test("use-style picks the variant by parameter: the fill and the label colour follow", () => {
  const { scene: s } = scene(
    component(
      "",
      `    item Btn {
        auto-layout row; gap 8; align center; corner-radius 8
        use-style \${kind} \${size}
        item Label { title \${title}; font-size 14 }
    }`,
    ) +
      `element "Screen" {
    item row { auto-layout row; gap 16
        use Button as a { title "A"; kind primary; size large }
        use Button as b { title "B"; kind secondary; size large }
    }
}`,
  );
  const p = paint(s);
  // The box of a is #6b3ad3, of b #ffffff; their labels take the text-colour of the style.
  assert.deepEqual(p.fills.filter((f) => f === "rgb(107,58,211)"), ["rgb(107,58,211)"]);
  assert.deepEqual(p.fills.filter((f) => f === "rgb(255,255,255)"), ["rgb(255,255,255)"]);
  assert.deepEqual(
    p.texts.map((t) => [t.text, t.fill]),
    [
      ["A", "rgb(255,255,255)"],
      ["B", "rgb(23,37,71)"],
    ],
  );
  assert.equal(box(s, "Btn").height, 48);
});

test("the style goes where use-style is written: above it a default, below it wins", () => {
  const src = (props: string): string =>
    component("", `    item Btn {\n        auto-layout row\n        ${props}\n    }`) +
    `element "Screen" { item row { auto-layout row; use Button as a { kind primary; size large } } }`;
  // height 40 above use-style is a default the style overrides.
  assert.equal(box(scene(src("height 40; use-style large")).scene, "Btn").height, 48);
  // height 40 below use-style wins over the style.
  assert.equal(box(scene(src("use-style large; height 40")).scene, "Btn").height, 40);
});

test("text-color and font inherit down to the labels", () => {
  const { scene: s } = scene(`element "Card" {
    styles { dark { color #172547; text-color #ffffff; font-size 11 } }
    item Body {
        auto-layout column
        use-style dark
        item Title { title "Hi" }
        item Sub { title "sub"; font-size 20 }
    }
}
element "Screen" { item row { auto-layout row; use Card as card } }`);
  const p = paint(s);
  assert.deepEqual(
    p.texts.map((t) => [t.text, t.fill, t.font.includes("11px")]),
    [
      ["Hi", "rgb(255,255,255)", true],
      ["sub", "rgb(255,255,255)", false],
    ],
  );
});

test("the values of a use override export let, expressions are evaluated", () => {
  const { scene: s } = scene(
    component(
      "",
      `    item Btn {
        auto-layout row
        width \${size === "large" ? 100 : 50 + 2}
        use-style \${size}
        item Label { title \${title} }
    }`,
    ) +
      `element "Screen" {
    item row { auto-layout row; gap 10
        use Button as a { title "Save"; size large }
        use Button as b { title "Cancel"; size medium }
    }
}`,
  );
  const p = paint(s);
  assert.deepEqual(p.texts.map((t) => t.text), ["Save", "Cancel"]);
  // 100 from the ternary for the large one, 52 (50 + 2) for the medium one.
  assert.deepEqual(
    s.boxes.filter((b) => b.name === "Btn").map((b) => [b.width, b.height]),
    [
      [100, 48],
      [52, 40],
    ],
  );
});

test("quoted path segments name nodes with spaces and punctuation", () => {
  const { scene: s } = scene(`element "2 Пространства - пусто" {
    item "My Button" { width 20; height 10; color #ff0000 }
    item "Кнопка (primary)" { width 30; height 10; color #00ff00 }
}
element "Screen" {
    item row { auto-layout row; gap 4
        use "2 Пространства - пусто" . "My Button" as q
        use "2 Пространства - пусто".Кнопка__primary_ as k
    }
}`);
  assert.deepEqual(
    [box(s, "q").width, box(s, "q").height, box(s, "k").width],
    [20, 10, 30],
  );
  assert.deepEqual(s.diagnostics, []);
});

test("at reaches a part of an instance by a quoted path", () => {
  const { scene: s } = scene(`element "Kit" {
    item Dialog { auto-layout column
        item "Footer Ok" { title "OK"; width 40; height 20 }
    }
}
element "Screen" {
    item row { auto-layout row
        use Kit as d {
            at Dialog."Footer Ok" { width 60 }
        }
    }
}`);
  assert.equal(box(s, "Footer Ok").width, 60);
});

test("a slot declared inside replace is filled from the block of the use", () => {
  const { scene: s } = scene(`element "Kit" {
    item Card { auto-layout column
        item Head { title "Card"; width 30; height 10 }
        item Body { item Inner { title "fallback"; width 30; height 10 } }
    }
}
element "Screen" {
    item row { auto-layout row
        use Kit.Card as card {
            replace Body {
                item Box { auto-layout column
                    slot content { item Fallback { title "fb"; width 20; height 10 } }
                }
            }
            slot content { item Passed { title "passed"; width 40; height 10 } }
        }
    }
}`);
  const p = paint(s);
  assert.ok(p.texts.some((t) => t.text === "passed"), "the passed node is drawn");
  assert.ok(!p.texts.some((t) => t.text === "fb"), "the fallback of the slot is replaced");
  assert.equal(box(s, "Box").height, 10);
});

test("the first frame of an animation is the still picture", () => {
  const { scene: s } = scene(`element "Screen" {
    animation fade {
        from { opacity 0.4; color #ff0000 }
        to { opacity 1; color #00ff00 }
    }
    animation spin {
        from { rotate 0; scale 1 }
        to { rotate 360; scale 2 }
    }
    item row { auto-layout row; gap 10
        item A { width 10; height 10; color #0000ff; animation fade 200ms ease-out }
        item B { width 10; height 10; color #0000ff; animation spin 1s linear infinite }
    }
}`);
  const p = paint(s);
  // The fill of A is the first frame, dimmed by its opacity; B keeps its own colour (rotate/scale do not paint).
  assert.deepEqual(p.fills, ["rgba(255,0,0,0.4)", "rgb(0,0,255)"]);
});

test("an animation of a style is played where the style is named", () => {
  const { scene: s } = scene(`element "Badge" {
    export let title = "Syncing"
    export let state = idle
    styles {
        idle { color #eef2ff }
        error { color #fee2e2; animation blink 700ms ease-in 3 }
        state one-of idle error
    }
    animation blink {
        from { opacity 0.5 }
        to { opacity 1 }
    }
    item Frame { auto-layout row; use-style \${state}; item Label { title \${title} } }
}
element "Screen" {
    item row { auto-layout row; gap 8
        use Badge as ok { title "OK"; state idle }
        use Badge as bad { title "Bad"; state error }
    }
}`);
  const p = paint(s);
  assert.deepEqual(
    p.fills.filter((f) => f.startsWith("rgba(254,226,226")),
    ["rgba(254,226,226,0.5)"],
  );
});

test("renderTenn draws a scene through the SVG backend", async () => {
  const svg = await renderTenn(
    component("", `    item Btn { auto-layout row; use-style \${kind} \${size}; item Label { title \${title} } }`) +
      `element "Screen" {
    item row {
        auto-layout row
        use Button as a { title "Save"; kind primary; size large }
    }
}`,
    { format: "svg", element: "Screen" },
  );
  assert.match(svg, /<text[^>]*>Save<\/text>/);
  assert.match(svg, /fill="rgb\(107,58,211\)"/);
  assert.match(svg, /width="\d+" height="\d+"/);
});

test("a link is drawn edge to edge, with an arrow head when display says so", () => {
  const { scene: s } = scene(`element "Screen" {
    item row { auto-layout row; gap 20
        item A { width 20; height 10; color #ff0000 }
        item B { width 20; height 10; color #ff0000 }
        link "A" "B" { display arrow }
    }
}`);
  const p = paint(s);
  assert.deepEqual(p.strokes, ["rgb(0,0,0)"]);
  // The two boxes and the filled triangle of the arrow.
  assert.deepEqual(p.fills, ["rgb(255,0,0)", "rgb(255,0,0)", "rgb(0,0,0)"]);
});

test("an item above the origin is part of the picture", () => {
  const { scene: s } = scene(`element "Screen" { item A { pos 10 10; width 20; height 10; color #ff0000 } }`);
  // pos 10 10 is 10 up: the picture spans layout y -10..10, so the box reaches 10 above the origin.
  assert.deepEqual(s.bounds, { x: 0, y: 0, width: 30, height: 10 });
});
