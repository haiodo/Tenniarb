// Pins the Canvas2D members the renderer touches (also listed in .work/web-stage5/canvas-api.md for the SVG backend).
import assert from "node:assert/strict";
import { test } from "node:test";
import type { Canvas2D } from "../src/index.ts";
import { renderElement } from "../src/index.ts";
import { loadDecoder, newCanvas, parseDiagram } from "./helpers.ts";

const allowed = new Set([
  "save", "restore", "translate", "scale",
  "beginPath", "moveTo", "lineTo", "arcTo", "quadraticCurveTo", "closePath", "rect", "ellipse", "clip", "fill", "stroke", "fillRect",
  "fillStyle", "strokeStyle", "lineWidth", "lineDashOffset", "setLineDash",
  "shadowColor", "shadowBlur", "shadowOffsetX", "shadowOffsetY",
  "font", "textBaseline", "fillText", "measureText", "drawImage",
]);

function recording(target: Canvas2D): { ctx: Canvas2D; used: Set<string> } {
  const used = new Set<string>();
  const ctx = new Proxy(target, {
    get(t, key) {
      used.add(String(key));
      const v = Reflect.get(t, key, t);
      return typeof v === "function" ? v.bind(t) : v;
    },
    set(t, key, value) {
      used.add(String(key));
      return Reflect.set(t, key, value, t);
    },
  });
  return { ctx, used };
}

test("renderer only uses the documented Canvas2D members", async () => {
  const element = parseDiagram(`element Root {
    styles {
        item {
            shadow 2 2 4 gray
        }
    }
    item Box {
        pos 0 0
        line-style dashed
        corner-radius 3
        body {
            text %{# Title
* bullet *bold* _it_ ~gone~ <u>under</u> \`code\`
!(red|red) &(10|small)}
        }
    }
    item Dots {
        pos 200 0
        line-style dotted
        display stack
    }
    item Round {
        pos 0 -100
        display circle
        shadow 1 -1 3
    }
    item Plain {
        pos 200 -100
        display text
        shadow 1 -1 3
    }
    item Hidden {
        pos 300 -100
        display none
    }
    link Box Dots {
        display arrows
        layout quad
        shadow 1 1 2
        line-style dashed
    }
    link Box Round {
        layout middle
        display arrow
    }
}`);
  const { ctx, canvas2d } = newCanvas(500, 400);
  const { ctx: rec, used } = recording(canvas2d);
  renderElement(rec, element, { decodeImage: await loadDecoder(element) });
  void ctx;
  const extra = [...used].filter((k) => !allowed.has(k));
  assert.deepEqual(extra, [], "members outside the documented subset");
  for (const must of ["save", "restore", "arcTo", "quadraticCurveTo", "ellipse", "clip", "shadowBlur", "setLineDash", "fillText", "fillRect", "translate", "scale"]) {
    assert.ok(used.has(must), `${must} unused: the scene did not exercise it`);
  }
});
