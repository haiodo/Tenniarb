// The scene renderer on a real canvas, in pixels. draw() draws in the y-up world the caller flips the
// canvas into (drawScene in render.ts, the editor, an SVG writer), so a card's title has to land above
// its body and its glyphs have to stand upright. The recorder tests read the geometry of the calls and
// miss a mirrored picture, so this one reads the image itself.
import assert from "node:assert/strict";
import { test } from "node:test";
import { readTenn } from "@tenniarb/core";
import { buildTennScene } from "../src/scene-tenn.ts";
import { newCanvas } from "./helpers.ts";

const source = `element "Card" {
    item "Doc" {
        auto-layout column
        width 200
        padding 16
        gap 10
        item "T" { title "L"; font-size 40 }
        body %{
            plain line

            \`\`\`
            code
            \`\`\`
        }
    }
}`;

interface Pixels {
  data: Uint8ClampedArray;
  width: number;
  height: number;
}

/** The card drawn the way the callers do it: the canvas is flipped once, draw() fills the y-up world. */
function render(): Pixels {
  const model = readTenn(source);
  assert.ok(model !== null, "the document parses");
  const element = model.elements.find((e) => e.name === "Card");
  assert.ok(element !== undefined, "element Card");
  const scene = buildTennScene(element, { source });
  const width = Math.ceil(scene.width);
  const height = Math.ceil(scene.height);
  assert.ok(Number.isFinite(width) && width > 0 && Number.isFinite(height) && height > 0, `the scene has a size: ${scene.width}x${scene.height}`);
  const { ctx } = newCanvas(width, height);
  ctx.fillStyle = "#ffffff";
  ctx.fillRect(0, 0, width, height);
  ctx.save();
  ctx.translate(0, height);
  ctx.scale(1, -1);
  scene.draw(ctx);
  ctx.restore();
  const image = ctx.getImageData(0, 0, width, height);
  return { data: image.data, width, height };
}

const at = (p: Pixels, x: number, y: number): number[] => {
  const i = (y * p.width + x) * 4;
  return [p.data[i]!, p.data[i + 1]!, p.data[i + 2]!];
};

/** Dark ink of a row: glyph pixels, not the white background and not the grey code background. */
const ink = (p: Pixels, y: number): number => {
  let n = 0;
  for (let x = 0; x < p.width; x += 1) {
    const [r, g, b] = at(p, x, y) as [number, number, number];
    if (r < 96 && g < 96 && b < 96) n += 1;
  }
  return n;
};

/** Rows with the grey code background: 238,238,238 against the white 255 and the black text. */
const codeRow = (p: Pixels, y: number): boolean => {
  let n = 0;
  for (let x = 0; x < p.width; x += 1) {
    const [r, g, b] = at(p, x, y) as [number, number, number];
    if (Math.abs(r - 238) <= 4 && Math.abs(g - 238) <= 4 && Math.abs(b - 238) <= 4) n += 1;
  }
  return n > p.width / 2;
};

const rows = (p: Pixels, match: (row: number) => boolean): number[] => {
  const out: number[] = [];
  for (let y = 0; y < p.height; y += 1) if (match(y)) out.push(y);
  return out;
};

test("scene draw: a title above the body, upright glyphs, the code block below", () => {
  const p = render();
  const inked = rows(p, (y) => ink(p, y) > 0);
  assert.ok(inked.length > 0, `the picture has text at all: draw() wrote inside the canvas (${p.width}x${p.height})`);

  // The title "L" is the first band of ink: a stem, and its foot along the bottom.
  const titleTop = inked[0]!;
  const gap = inked.find((y, i) => i > 0 && y > inked[i - 1]! + 1);
  assert.ok(gap !== undefined, "a blank row after the title");
  const titleBottom = gap - 1;
  assert.ok(titleTop < p.height * 0.35, `the title starts at ${titleTop} of ${p.height}, not at the top`);

  const band = inked.filter((y) => y >= titleTop && y <= titleBottom);
  const third = Math.floor(band.length / 3);
  const top = band.slice(0, third).reduce((s, y) => s + ink(p, y), 0);
  const bottom = band.slice(band.length - third).reduce((s, y) => s + ink(p, y), 0);
  assert.ok(bottom > top * 1.5, `the "L" is mirrored: ${top} ink pixels in its top third, ${bottom} in its bottom, its foot has to be the wide one`);

  // The code block is grey across the card and sits below the title, not above it.
  const grey = rows(p, (y) => codeRow(p, y));
  assert.ok(grey.length > 0, "the code block is painted");
  assert.ok(grey[0]! > titleBottom, `the code block starts at ${grey[0]}, the title ends at ${titleBottom}`);
  assert.ok(grey[0]! > p.height * 0.4, `the code block is at ${grey[0]} of ${p.height}, not in the lower part`);
});
