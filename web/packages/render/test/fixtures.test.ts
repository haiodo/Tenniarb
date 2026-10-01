// Every fixture that parses renders without throwing; set TENN_PNG_DIR to also write PNGs for eyeballing.
import assert from "node:assert/strict";
import { mkdirSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { readTenn } from "@tenniarb/core";
import type { Element } from "@tenniarb/core";
import { buildScene, getSceneSize, renderElement } from "../src/index.ts";
import { loadDecoder, measure, newCanvas } from "./helpers.ts";

const dir = fileURLToPath(new URL("../../../fixtures/", import.meta.url));
const names = readdirSync(dir).filter((f) => f.endsWith(".tenn") && !f.endsWith(".saved.tenn"));
assert.ok(names.length > 0, `no .tenn fixtures in ${dir}`);

const pngDir = process.env.TENN_PNG_DIR;
if (pngDir !== undefined) {
  mkdirSync(pngDir, { recursive: true });
}

function* walk(el: Element, path: string): Generator<[string, Element]> {
  yield [path, el];
  for (const child of el.elements) {
    yield* walk(child, `${path}.${child.name.replace(/[^\w-]+/g, "_")}`);
  }
}

for (const name of names) {
  const model = readTenn(readFileSync(dir + name, "utf8"));
  if (model === null) {
    continue;
  }
  test(`fixture ${name} renders`, async () => {
    for (const [path, element] of walk(model, name.replace(/\.tenn$/, ""))) {
      const options = { decodeImage: await loadDecoder(element), measureContext: measure.canvas2d };
      const scene = buildScene(element, options);
      const bounds = scene.getBounds();
      for (const v of [bounds.x, bounds.y, bounds.width, bounds.height]) {
        assert.ok(Number.isFinite(v), `${path}: bounds ${JSON.stringify(bounds)}`);
      }
      assert.ok(bounds.width >= 0 && bounds.height >= 0);
      for (const item of element.items) {
        if (item.kind === "Item") {
          const b = scene.getItemBounds(item);
          assert.ok(b !== null && b.width > 0 && b.height > 0, `${path}: ${item.name} has bounds`);
        }
      }

      const size = getSceneSize(element, options);
      const { canvas, canvas2d } = newCanvas(Math.max(1, Math.ceil(size.width)), Math.max(1, Math.ceil(size.height)));
      const area = renderElement(canvas2d, element, { ...options, background: "#e7e9eb" });
      assert.deepEqual(area, { x: 0, y: 0, ...size });
      if (pngDir !== undefined && element.items.length > 0) {
        writeFileSync(`${pngDir}/${path}.png`, canvas.toBuffer("image/png"));
      }
    }
  });
}
