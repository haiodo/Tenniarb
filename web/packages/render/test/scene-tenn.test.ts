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
import { newCanvas } from "./helpers.ts";

/** A Canvas2D that records what is painted: fills, strokes, paths, dashes, text, images and the alpha in force. */
function record(): {
  ctx: Canvas2D;
  fills: string[];
  strokes: string[];
  paths: (string | number)[][][];
  dashes: { dash: number[]; phase: number }[];
  texts: { text: string; fill: string; font: string }[];
  images: { x: number; y: number; w: number; h: number }[];
} {
  const fills: string[] = [];
  const strokes: string[] = [];
  const paths: (string | number)[][][] = [];
  const dashes: { dash: number[]; phase: number }[] = [];
  const texts: { text: string; fill: string; font: string }[] = [];
  const images: { x: number; y: number; w: number; h: number }[] = [];
  let pen: (string | number)[][] = [];
  const state = { fill: "rgb(0,0,0)", stroke: "rgb(0,0,0)", alpha: 1, font: "", dash: [] as number[], phase: 0 };
  const stack: (typeof state)[] = [];
  const paint = (c: string): string => (state.alpha === 1 || !c.startsWith("rgb(") ? c : c.replace("rgb(", "rgba(").replace(")", `,${state.alpha})`));
  const ctx: Canvas2D = {
    save: () => stack.push({ ...state }),
    restore: () => Object.assign(state, stack.pop() ?? state),
    translate: () => {},
    scale: () => {},
    rotate: () => {},
    beginPath: () => {
      pen = [];
    },
    moveTo: (x: number, y: number) => {
      pen.push(["m", x, y]);
    },
    lineTo: (x: number, y: number) => {
      pen.push(["l", x, y]);
    },
    arcTo: () => {},
    quadraticCurveTo: (cx: number, cy: number, x: number, y: number) => {
      pen.push(["q", cx, cy, x, y]);
    },
    closePath: () => {
      pen.push(["z"]);
    },
    rect: () => {},
    ellipse: () => {},
    clip: () => {},
    fill: () => fills.push(paint(state.fill)),
    stroke: () => {
      strokes.push(paint(state.stroke));
      paths.push(pen.map((cmd) => [...cmd]));
      dashes.push({ dash: [...state.dash], phase: state.phase });
    },
    fillRect: () => {},
    fillText: (text: string) => texts.push({ text, fill: paint(state.fill), font: state.font }),
    measureText: (text: string) => ({ width: [...text].length * 10 }),
    drawImage: (_image: unknown, x: number, y: number, w: number, h: number) => images.push({ x, y, w, h }),
    setLineDash: (segments: number[]) => {
      state.dash = [...segments];
    },
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
    get lineDashOffset() {
      return state.phase;
    },
    set lineDashOffset(v: number) {
      state.phase = v;
    },
    shadowColor: "",
    shadowBlur: 0,
    shadowOffsetX: 0,
    shadowOffsetY: 0,
    textBaseline: "alphabetic",
  };
  return { ctx, fills, strokes, paths, dashes, texts, images };
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

const box = (s: TennScene, name: string): TennScene["boxes"][number] => {
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

// An 8x4 PNG and a 40x20 one, the payload of a classic `image "name" @(...)`.
const png8x4 = "iVBORw0KGgoAAAANSUhEUgAAAAgAAAAECAYAAACzzX7wAAAABHNCSVQICAgIfAhkiAAAAAFzUkdCAK7OHOkAAAAVSURBVAiZYzROO/OfAQ9gwidJHQUANtQCbKJNLnwAAAAASUVORK5CYII=";
const png40x20 = "iVBORw0KGgoAAAANSUhEUgAAACgAAAAUCAYAAAD/Rn+7AAAABHNCSVQICAgIfAhkiAAAAAFzUkdCAK7OHOkAAAA3SURBVEiJ7c4xAQAgCABBpBKdTGsnicDwgw5/CW7VPjc+lq8DE4OUQcogZZAySBmkDFIGKYNUAxRmAowWRmlIAAAAAElFTkSuQmCC";

/** A decoder that lies about the pixels: the op's size has to come from the base64 header, not from it. */
const stubDecode = (data: string) => ({ image: data, width: 1, height: 1 });

test("a classic inline image nothing names fills the box and sizes it by its pixels", () => {
  const { element } = scene(`element "Screen" {
    item Pic { pos 0 0; title ""; image "logo" @(${png8x4}) }
}`);
  const s = buildTennScene(element, { decodeImage: stubDecode });
  const p = record();
  s.draw(p.ctx);
  assert.deepEqual([box(s, "Pic").width, box(s, "Pic").height], [8, 4]);
  assert.deepEqual(p.images, [{ x: 0, y: 0, w: 8, h: 4 }]);
});

test("a title reference draws the inline picture at the width it asks", () => {
  const { element } = scene(`element "Screen" {
    item Pic { pos 0 0; width 200; height 60; title "Icon @(logo|20)"; image "logo" @(${png40x20}) }
}`);
  const s = buildTennScene(element, { decodeImage: stubDecode });
  const p = record();
  s.draw(p.ctx);
  // The title names the picture, so it is text of the title and not the content of the box: one op.
  assert.deepEqual(
    p.images.map((im) => [im.w, im.h]),
    [[20, 10]],
  );
});

test("a body read with --from-file names the picture instead of the box", () => {
  const { element } = scene(`element "Screen" {
    item Pic { pos 0 0; width 300; height 100; body --from-file "docs/plan.md"; image "logo" @(${png40x20}) }
}`);
  const s = buildTennScene(element, {
    file: "a.tenn",
    readFile: (path) => (path === "docs/plan.md" ? "Logo @(logo|40) in the page" : null),
    decodeImage: stubDecode,
  });
  const p = record();
  s.draw(p.ctx);
  assert.deepEqual(s.diagnostics, []);
  assert.deepEqual(
    p.images.map((im) => [im.w, im.h]),
    [[40, 20]],
  );
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

test("an item with parameters drawn in place sees its defaults", () => {
  const { scene: s } = scene(`element "Screen" {
    item Card {
        export let state = default
        auto-layout column; width 120
        item spin { width 10; height 10; color #ff0000; hidden \${state !== "loading"} }
    }
    item next { auto-layout column; width 120
        use Card as loaded { state loading }
    }
}`);
  assert.deepEqual(s.diagnostics, []);
  // the spinner of the item drawn in place is hidden by its default; the loaded use draws one
  assert.equal(s.boxes.filter((b) => b.name === "spin").length, 1);
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

test("route elbow leaves the sides it names and rounds its corners", () => {
  const { scene: s } = scene(`element "Screen" {
    item Stage {
        width 400; height 300; auto-layout abs; color none; border-color none
        item A { pos 0 0; width 20; height 20 }
        item B { pos 200 -100; width 20; height 20 }
        link A B { route elbow bottom top }
    }
}`);
  assert.deepEqual(s.diagnostics, []);
  // A's bottom (10,20) to B's top (210,100), the two corners bent by the default 12.
  assert.deepEqual(paint(s).paths, [
    [
      ["m", 10, 20],
      ["l", 10, 48],
      ["q", 10, 60, 22, 60],
      ["l", 198, 60],
      ["q", 210, 60, 210, 72],
      ["l", 210, 100],
    ],
  ]);
});

test("two links on one side spread along it in the order of the far ends", () => {
  const { scene: s } = scene(`element "Screen" {
    item Stage {
        width 400; height 300; auto-layout abs; color none; border-color none
        item A { pos 0 0; width 20; height 20 }
        item B { pos 200 0; width 20; height 20 }
        item C { pos 200 -100; width 20; height 20 }
        link A B { route elbow right left }
        link A C { route elbow right left }
    }
}`);
  // B sits above C, so its link leaves A's right side first: one third and two thirds down.
  const starts = paint(s).paths.map((path) => path[0]!);
  assert.deepEqual(starts.map((cmd) => cmd[1]), [20, 20]);
  assert.ok(Math.abs((starts[0]![2] as number) - 20 / 3) < 0.001, `first start ${starts[0]![2]}`);
  assert.ok(Math.abs((starts[1]![2] as number) - 40 / 3) < 0.001, `second start ${starts[1]![2]}`);
});

test("line-style dashed on a link sets its dash and phase", () => {
  const { scene: s } = scene(`element "Screen" {
    item Stage {
        width 400; height 300; auto-layout abs; color none; border-color none
        item A { pos 0 0; width 20; height 20 }
        item B { pos 200 0; width 20; height 20 }
        link A B { line-style dashed }
    }
}`);
  assert.deepEqual(paint(s).dashes, [{ dash: [5, 5], phase: 5 }]);
});

test("a link whose end is an item of another parent fails", () => {
  const { scene: s } = scene(`element "Screen" {
    item A { pos 0 0; width 20; height 20 }
    item Box {
        pos 100 0; width 60; height 60
        item Inner { width 10; height 10 }
    }
    link A Inner
}`);
  assert.deepEqual(
    s.diagnostics.map((d) => [d.message, d.warn]),
    [['link "A" "Inner": both ends must be items of the same parent', false]],
  );
});

test("an item above the origin is part of the picture", () => {
  const { scene: s } = scene(`element "Screen" { item A { pos 10 10; width 20; height 10; color #ff0000 } }`);
  // pos 10 10 is 10 up: the picture spans layout y -10..10, so the box reaches 10 above the origin.
  assert.deepEqual(s.bounds, { x: 0, y: 0, width: 30, height: 10 });
});

test("color linear: the angle and the stops go to the canvas gradient", () => {
  const { scene: s } = scene(`element "Screen" {
    item A { width 100; height 50; auto-layout row; title ""; color linear 90 #ff0000 0 #00ff00 0.25 #0000ff 1 }
}`);
  const out = record();
  const gradients: { from: number[]; stops: [number, string][] }[] = [];
  out.ctx.createLinearGradient = (x0, y0, x1, y1) => {
    const g = { from: [x0, y0, x1, y1], stops: [] as [number, string][] };
    gradients.push(g);
    return { addColorStop: (offset, color) => g.stops.push([offset, color]) };
  };
  s.draw(out.ctx);
  // 90 degrees is to the right: the line goes corner to corner through the middle of the 100x50 box.
  assert.deepEqual(gradients.map((g) => ({ from: g.from.map(round), stops: g.stops })), [
    {
      from: [0, 25, 100, 25],
      stops: [
        [0, "rgb(255,0,0)"],
        [0.25, "rgb(0,255,0)"],
        [1, "rgb(0,0,255)"],
      ],
    },
  ]);
});

test("color linear 0 points up: the stops spread evenly from the bottom", () => {
  const { scene: s } = scene(`element "Screen" {
    item A { width 100; height 50; auto-layout row; title ""; color linear 0 #ff0000 #0000ff }
}`);
  const out = record();
  const gradients: { from: number[]; stops: [number, string][] }[] = [];
  out.ctx.createLinearGradient = (x0, y0, x1, y1) => {
    const g = { from: [x0, y0, x1, y1], stops: [] as [number, string][] };
    gradients.push(g);
    return { addColorStop: (offset, color) => g.stops.push([offset, color]) };
  };
  s.draw(out.ctx);
  // Go's gradient.at in the box's y-down frame: the line starts at the bottom (y 50) and runs up to 0,
  // so the first colour is at the bottom of the box, the last at its top.
  assert.deepEqual(gradients.map((g) => ({ from: g.from.map(round), stops: g.stops })), [
    {
      from: [50, 50, 50, 0],
      stops: [
        [0, "rgb(255,0,0)"],
        [1, "rgb(0,0,255)"],
      ],
    },
  ]);
});

test("a bad color linear warns and draws the default flat fill", () => {
  const { scene: s } = scene(`element "Screen" {
    item A { width 10; height 10; color linear sideways #ff0000 #0000ff }
    item B { width 10; height 10; color linear 0 #ff0000 0.5 #0000ff }
    item C { width 10; height 10; color linear 0 #ff0000 0.5 #0000ff 0.2 }
    item D { width 10; height 10; color linear 0 #ff0000 zz }
    item E { width 10; height 10; color linear 0 #ff0000 }
    item F { auto-layout row }
}`);
  const p = paint(s);
  assert.deepEqual(
    s.diagnostics.map((d) => d.message),
    [
      "color linear: the angle is not a number, the default is used",
      "color linear: give a position to every stop or to none, the default is used",
      "color linear: positions must grow within 0..1, the default is used",
      'color linear: "zz" is not a color, the default is used',
      "color linear: needs at least 2 stops, the default is used",
    ],
  );
  // Go's fill() falls back to the default of the box, white here, not to a stop of the bad gradient.
  assert.deepEqual(p.fills, ["rgb(255,255,255)", "rgb(255,255,255)", "rgb(255,255,255)", "rgb(255,255,255)", "rgb(255,255,255)"]);
});

const round = (v: number): number => Math.round(v * 1000) / 1000;

/**
 * Draws a scene on a real canvas the way drawScene does: the canvas is flipped once and draw() fills
 * the y-up world. px() then takes the scene's own coordinates, y down from the top of the picture.
 */
function pixels(src: string): { scene: TennScene; px: (x: number, y: number) => number[] } {
  const { scene: s } = scene(src);
  const { canvas2d, ctx } = newCanvas(200, 120);
  const margin = 20;
  canvas2d.fillStyle = "#ffffff";
  canvas2d.fillRect(0, 0, 200, 120);
  canvas2d.save();
  canvas2d.translate(0, 120);
  canvas2d.scale(1, -1);
  s.draw(canvas2d, { x: 0, y: 120 - margin - s.height });
  canvas2d.restore();
  return { scene: s, px: (x, y) => Array.from(ctx.getImageData(Math.round(x), Math.round(y + margin), 1, 1).data) };
}

test("color linear paints a gradient, not the flat fill of one stop", () => {
  const { scene: s, px } = pixels(`element "Screen" {
    item A { pos 0 0; width 100; height 50; title ""; color linear 90 #ff0000 #0000ff }
    item C { auto-layout row }
}`);
  const b = box(s, "A");
  const y = b.y + b.height / 2;
  const left = px(b.x + 8, y);
  const right = px(b.x + b.width - 8, y);
  assert.ok(left[0]! > 220 && left[2]! < 40, `left end is red: ${left}`);
  assert.ok(right[2]! > 220 && right[0]! < 40, `right end is blue: ${right}`);

  // 0 degrees points up: the first colour is at the bottom of the box, the last at the top.
  const up = pixels(`element "Screen" {
    item A { pos 0 0; width 100; height 50; title ""; color linear 0 #ff0000 #0000ff }
    item C { auto-layout row }
}`);
  const ub = box(up.scene, "A");
  const top = up.px(ub.x + ub.width / 2, ub.y + 4);
  const bottom = up.px(ub.x + ub.width / 2, ub.y + ub.height - 4);
  assert.ok(top[2]! > 220 && top[0]! < 40, `top is blue: ${top}`);
  assert.ok(bottom[0]! > 220 && bottom[2]! < 40, `bottom is red: ${bottom}`);
});

test("shadow is a drop shadow under the fill, offset up and to the right", () => {
  const { scene: s, px } = pixels(`element "Screen" {
    item A { pos 20 0; width 60; height 40; title ""; color #ffffff; shadow 12 8 6 }
    item C { auto-layout row }
}`);
  const b = box(s, "A");
  const mid = b.y + b.height / 2;
  const inside = px(b.x + b.width / 2, mid);
  const right = px(b.x + b.width + 6, mid);
  const above = px(b.x + b.width / 2, b.y - 6);
  const far = px(b.x - 10, mid);
  assert.deepEqual(inside.slice(0, 3), [255, 255, 255], "the fill covers the shadow");
  assert.ok(right[0]! < 245, `grey shadow right of the box: ${right}`);
  assert.ok(above[0]! < 245, `grey shadow above the box (y is up): ${above}`);
  assert.deepEqual(far.slice(0, 3), [255, 255, 255], "nothing to the left");
});

test("an SVG export of a gradient and a shadow keeps the base fill and casts the shadow", async () => {
  const svg = await renderTenn(
    `element "Screen" {
    item row { auto-layout row; gap 10
        item A { width 40; height 20; title ""; color linear 90 #ff0000 #0000ff; shadow 4 4 }
    }
}`,
    { format: "svg", element: "Screen" },
  );
  // The SVG writer has no gradient object: the box takes the base colour of the gradient.
  assert.match(svg, /fill="rgb\(255,0,0\)"/);
  assert.match(svg, /<feDropShadow /);
});

test("a box that holds items draws neither its name nor its body", () => {
  const { scene: s } = scene(`element "Screen" {
    item Box {
        width 200; height 100; color #ffffff; body "hidden"
        item In { width 50; height 20; title "in" }
    }
    item Leaf { pos 0 -120; width 100; height 40; body "shown" }
    item C { auto-layout row; color none }
}`);
  assert.deepEqual(paint(s).texts.map((t) => t.text), ["in", "Leaf", "shown"]);
});

test("each shadow line is a pass under the fill", () => {
  const one = paint(
    scene(`element "Screen" {
    item A { width 40; height 20; title ""; color #ffffff; shadow 0 2 4 }
    item C { auto-layout row; color none }
}`).scene,
  );
  const two = paint(
    scene(`element "Screen" {
    item A { width 40; height 20; title ""; color #ffffff; shadow 0 2 4; shadow 0 8 24 #00000018 }
    item C { auto-layout row; color none }
}`).scene,
  );
  assert.equal(one.fills.length, 1);
  assert.equal(two.fills.length, 2);
});

test("line-style dashed sets the dash and its phase on the border", () => {
  const { scene: s } = scene(`element "Screen" {
    item A { width 100; height 40; title ""; line-style dashed }
    item C { auto-layout row; color none }
}`);
  assert.deepEqual(paint(s).dashes, [{ dash: [5, 5], phase: 5 }]);
  const dotted = paint(
    scene(`element "Screen" {
    item A { width 100; height 40; title ""; line-style dotted }
    item C { auto-layout row; color none }
}`).scene,
  );
  assert.deepEqual(dotted.dashes, [{ dash: [1, 4], phase: 1 }]);
});

test("a dashed border does not dash the icon of a child", () => {
  const { scene: s } = scene(`element "Screen" {
    item Box {
        width 100; height 100; line-style dashed
        item Icon { pos 10 -10; width 20; height 20; path "M0 0 L10 0 L10 10 Z"; border-color #ff0000 }
    }
}`);
  assert.deepEqual(paint(s).dashes, [
    { dash: [5, 5], phase: 5 },
    { dash: [], phase: 0 },
  ]);
});
