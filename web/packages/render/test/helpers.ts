import { GlobalFonts, createCanvas, loadImage } from "@napi-rs/canvas";
import { readTenn } from "@tenniarb/core";
import type { Element } from "@tenniarb/core";
import { registerInterFonts } from "../src/node.ts";
import { setMeasureContext } from "../src/text.ts";
import type { Canvas2D } from "../src/canvas-types.ts";
import { preloadImages } from "../src/images.ts";
import type { ImageDecoder } from "../src/images.ts";

registerInterFonts(GlobalFonts);

export function newCanvas(width = 400, height = 400) {
  const canvas = createCanvas(width, height);
  const ctx = canvas.getContext("2d");
  // @napi-rs/canvas segfaults when a canvas is collected while its context is still in use.
  Object.defineProperty(ctx, "ownerCanvas", { value: canvas });
  return { canvas, ctx, canvas2d: ctx as Canvas2D };
}

// Measuring needs a context; the tests that build scenes without drawing share this one.
export const measure = newCanvas(1, 1);
setMeasureContext(measure.canvas2d);

// A napi Image is only drawable after a tick, so tests load images up front with preloadImages.
export async function loadDecoder(element: Element): Promise<ImageDecoder> {
  return preloadImages(element, async (base64) => {
    const bytes = Buffer.from(base64, "base64");
    // @napi-rs/canvas segfaults (at GC) on a truncated PNG, as in fixtures/Example.tenn; only accept ones that reach IEND.
    if (bytes.subarray(0, 4).toString("latin1") === "\x89PNG" && !bytes.subarray(-8).equals(Buffer.from("49454e44ae426082", "hex"))) {
      return null;
    }
    const img = await loadImage(bytes);
    return { image: img, width: img.width, height: img.height };
  });
}

/** Parse tenn source and return the first nested element (SceneRenderTests.parseDiagram). */
export function parseDiagram(source: string): Element {
  const model = readTenn(source);
  if (model === null) {
    throw new Error("fixture must parse cleanly");
  }
  return model.elements[0]!;
}
